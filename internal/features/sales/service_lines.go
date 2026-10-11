package sales

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/recipes"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/features/warranty"
	"karots-suite/internal/money"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// saleLines is the priced, stock-adjusted result of processing a sale's items:
// the per-line rows and running totals, plus the records that can only be
// written once the sale id exists (serial warranty units, service-component
// stock movements).
type saleLines struct {
	lines           []SaleItem
	subtotal        decimal.Decimal
	taxTotal        decimal.Decimal
	itemDiscTotal   decimal.Decimal
	serviceProducts map[int64]bool
	componentMoves  map[int][]ServiceComponentParsed
	serialBatches   []serialBatch
}

// buildSaleLines prices each item, applies per-line discount + tax, takes the
// stock (lot-picked, FEFO, or found-at-till on an approved oversell) and
// validates serials — the whole per-item phase of Create, run inside its tx.
// Results land in *res only on success; early returns leave it untouched and
// the caller aborts the transaction. Returns a single error so the item-loop
// body could move here unchanged.
func (s *Service) buildSaleLines(ctx context.Context, tx *sqlx.Tx, in CreateInput, cashierID int64, res *saleLines) error {
	prodRepo := products.NewRepository(tx)
	stkRepo := stock.NewRepository(tx)
	warrRepo := warranty.NewRepository(tx)

	subtotal := decimal.Zero
	taxTotal := decimal.Zero
	itemDiscTotal := decimal.Zero
	lines := make([]SaleItem, 0, len(in.Items))
	serviceProducts := map[int64]bool{}
	componentMoves := map[int][]ServiceComponentParsed{}
	serialSeen := map[string]bool{}
	var serialBatches []serialBatch

	for _, it := range in.Items {
		p, err := prodRepo.FindByID(ctx, it.ProductID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return apperr.Validation(fmt.Sprintf("product %d not found", it.ProductID))
			}
			return apperr.Internal("failed to load product", err)
		}
		qty, err := money.Parse(it.Quantity)
		if err != nil || !qty.IsPositive() {
			return apperr.Validation(fmt.Sprintf("quantity for %s must be greater than zero", p.Name))
		}
		// Whole-only units (pieces, packets…) reject fractional quantities;
		// only units flagged allow_decimal (kg, g, ltr, ml) may be fractional.
		if !p.UnitAllowDecimal && !qty.Equal(qty.Truncate(0)) {
			return apperr.Validation(fmt.Sprintf("quantity for %s must be a whole number", p.Name))
		}
		// Serial-tracked products capture one unique serial per unit; validate
		// here and record them as warranty units once the sale id is known.
		if p.TrackSerial {
			if !qty.Equal(qty.Truncate(0)) {
				return apperr.Validation(fmt.Sprintf("%s is serial-tracked — quantity must be a whole number", p.Name))
			}
			want := int(qty.IntPart())
			serials := make([]string, 0, want)
			for _, raw := range it.Serials {
				if sn := strings.TrimSpace(raw); sn != "" {
					serials = append(serials, sn)
				}
			}
			if len(serials) != want {
				return apperr.Validation(fmt.Sprintf("enter %d serial number(s) for %s", want, p.Name))
			}
			for _, sn := range serials {
				if serialSeen[sn] {
					return apperr.Validation(fmt.Sprintf("duplicate serial number %q", sn))
				}
				serialSeen[sn] = true
				exists, err := warrRepo.SerialExists(ctx, sn)
				if err != nil {
					return apperr.Internal("failed to check serial", err)
				}
				if exists {
					return apperr.Validation(fmt.Sprintf("serial number %q is already on record", sn))
				}
			}
			serialBatches = append(serialBatches, serialBatch{productID: p.ID, months: p.WarrantyMonths, serials: serials})
		}
		discValue, err := money.Parse(it.Discount)
		if err != nil || discValue.IsNegative() {
			return apperr.Validation(fmt.Sprintf("discount for %s is invalid", p.Name))
		}
		discType := normDiscountType(it.DiscountType)

		// The cashier picked a specific lot at the till because it matches the
		// sticker on the package. Lock it now: it decides both the price below
		// and which batch is depleted further down, and locking here means a
		// concurrent sale cannot empty it between the check and the take.
		var pickedBatch *stock.Batch
		if it.BatchID > 0 && !p.IsService {
			pickedBatch, err = stkRepo.LockBatch(ctx, it.BatchID, p.ID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return apperr.Validation(fmt.Sprintf("that price is no longer available for %s — scan it again", p.Name))
				}
				return apperr.Internal("failed to load batch", err)
			}
			if pickedBatch.QtyRemaining.LessThan(qty) {
				return apperr.Conflict(fmt.Sprintf(
					"only %s of %s left at that price", pickedBatch.QtyRemaining.String(), p.Name))
			}
		}

		unitPrice := p.SellingPrice
		if p.IsService && strings.TrimSpace(it.PriceOverride) != "" {
			// Service lines (e.g. a top-up) carry a per-line amount, not a
			// fixed catalogue price.
			ov, err := money.Parse(it.PriceOverride)
			if err != nil || ov.IsNegative() {
				return apperr.Validation(fmt.Sprintf("price for %s is invalid", p.Name))
			}
			unitPrice = ov
		} else if in.SaleType == "wholesale" && p.WholesalePrice.IsPositive() {
			// Wholesale wins over a lot price: it is a customer-type price, not
			// a property of the lot sitting on the shelf.
			unitPrice = p.WholesalePrice
		} else if pickedBatch != nil && pickedBatch.SellingPrice.IsPositive() {
			// This lot was received at its own price, which is what the package
			// in the customer's hand is stickered with — so that is what it
			// rings up at, whatever the shelf price has since moved to. A lot
			// with no price of its own keeps following the product.
			unitPrice = pickedBatch.SellingPrice
		}
		lineGross := qty.Mul(unitPrice).Round(2)
		// Per-item discount: fixed is per-unit (× qty), percent is off the line.
		// Clamped to [0, lineGross] so the net is never negative.
		disc := resolveItemDiscount(discType, discValue, lineGross, qty)
		lineNet := lineGross.Sub(disc)
		lineTax := lineNet.Mul(p.TaxRate).Div(hundred).Round(2)

		subtotal = subtotal.Add(lineGross)
		itemDiscTotal = itemDiscTotal.Add(disc)
		taxTotal = taxTotal.Add(lineTax)

		// Service lines (is_service) carry no inventory of their own. They may
		// still declare components to consume (e.g. paper for a document job):
		// deplete those FEFO and use their summed cost as the line COGS.
		cost := decimal.Zero
		var lineComps []ServiceComponentParsed
		if p.IsService {
			serviceProducts[p.ID] = true
			// A stored recipe supplies the components when the caller did not.
			// The documents plugin passes explicit components (its paper choice
			// depends on the size picked at the till), so an explicit list always
			// wins; everything else — coffee, any service with a recipe — is
			// expanded here so no plugin is needed to sell it.
			if len(it.Components) == 0 {
				rcs, rerr := recipes.NewRepository(tx).For(ctx, p.ID)
				if rerr != nil {
					return apperr.Internal("failed to load recipe", rerr)
				}
				for _, cons := range recipes.Expand(rcs, qty) {
					it.Components = append(it.Components, ServiceComponent{
						ProductID: cons.ProductID,
						Quantity:  cons.Qty.String(),
					})
				}
			}
			for _, comp := range it.Components {
				if comp.ProductID <= 0 {
					continue
				}
				cq, err := money.Parse(comp.Quantity)
				if err != nil || !cq.IsPositive() {
					return apperr.Validation(fmt.Sprintf("invalid consumable quantity for %s", p.Name))
				}
				ok, err := stkRepo.DecrementGuarded(ctx, comp.ProductID, cq)
				if err != nil {
					return apperr.Internal("failed to update stock", err)
				}
				if !ok {
					// The consumable count is short. Without an override, refuse —
					// but with a machine-readable code so the till can offer a
					// found-at-till confirm (the paper is on the shelf; the count
					// was just wrong), mirroring the product path below.
					if !it.AllowOversell {
						return apperr.ConflictCode("CONSUMABLE_SHORT",
							"insufficient stock for a consumable used by "+p.Name)
					}
					onHand, err := stkRepo.GetQuantity(ctx, comp.ProductID)
					if err != nil {
						return apperr.Internal("failed to read stock", err)
					}
					compCost, err := stkRepo.ProductCost(ctx, comp.ProductID)
					if err != nil {
						return apperr.Internal("failed to read cost", err)
					}
					// Correct the count up to cover this line (batchID 0 —
					// consumables aren't lot-picked at the till), then the guarded
					// decrement must succeed, so on-hand lands at 0, never negative.
					if err := stkRepo.FoundAtTill(ctx, comp.ProductID, 0, cq.Sub(onHand), compCost, cashierID); err != nil {
						return apperr.Internal("failed to correct stock", err)
					}
					ok, err = stkRepo.DecrementGuarded(ctx, comp.ProductID, cq)
					if err != nil {
						return apperr.Internal("failed to update stock", err)
					}
					if !ok {
						return apperr.Internal("stock correction did not cover the consumable", nil)
					}
				}
				ccost, err := stkRepo.DepleteFEFO(ctx, comp.ProductID, cq)
				if err != nil {
					return apperr.Internal("failed to deplete batches", err)
				}
				// DepleteFEFO returns the weighted cost *per consumed unit*; the
				// line's COGS is the total consumed (per-unit × qty) of every
				// component, divided by the line quantity so cost_price stays a
				// per-unit figure like every other sale line.
				cost = cost.Add(ccost.Mul(cq))
				lineComps = append(lineComps, ServiceComponentParsed{ProductID: comp.ProductID, Qty: cq})
			}
			if qty.IsPositive() {
				cost = cost.Div(qty).Round(2)
			}
		}
		if !p.IsService {
			// Atomic guard: prevents overselling under concurrency.
			ok, err := stkRepo.DecrementGuarded(ctx, p.ID, qty)
			if err != nil {
				return apperr.Internal("failed to update stock", err)
			}
			if !ok {
				if !it.AllowOversell {
					return apperr.Conflict(fmt.Sprintf("insufficient stock for %s", p.Name))
				}
				// The goods exist; the count was short. Correct up to cover the
				// line (topping up the picked lot, or opening a found lot), then
				// the guarded decrement must now succeed.
				onHand, err := stkRepo.GetQuantity(ctx, p.ID)
				if err != nil {
					return apperr.Internal("failed to read stock", err)
				}
				short := qty.Sub(onHand)
				batchID := int64(0)
				if pickedBatch != nil {
					batchID = pickedBatch.ID
				}
				if err := stkRepo.FoundAtTill(ctx, p.ID, batchID, short, p.CostPrice, cashierID); err != nil {
					return apperr.Internal("failed to correct stock", err)
				}
				ok, err = stkRepo.DecrementGuarded(ctx, p.ID, qty)
				if err != nil {
					return apperr.Internal("failed to update stock", err)
				}
				if !ok {
					return apperr.Internal("stock correction did not cover the sale", nil)
				}
			}
			// Deplete batches FEFO; the weighted cost of the consumed units is the
			// COGS snapshot for this line (more accurate than the product's current
			// cost when batches have different costs).
			//
			// A picked lot overrides FEFO: the customer is holding that package,
			// so that is the one leaving the shelf. Its own cost is the COGS.
			if pickedBatch != nil {
				cost, err = stkRepo.DepleteBatch(ctx, pickedBatch.ID, qty)
			} else {
				cost, err = stkRepo.DepleteFEFO(ctx, p.ID, qty)
			}
			if err != nil {
				return apperr.Internal("failed to deplete batches", err)
			}
			if cost.IsZero() {
				cost = p.CostPrice
			}
		}

		var desc *string
		if s := strings.TrimSpace(it.Description); s != "" {
			desc = &s
		}
		var batchID *int64
		if pickedBatch != nil {
			batchID = &pickedBatch.ID
		}
		lines = append(lines, SaleItem{
			BatchID:       batchID,
			ProductID:     p.ID,
			Quantity:      qty,
			UnitPrice:     unitPrice,
			CostPrice:     cost,
			Discount:      disc,
			DiscountType:  discType,
			DiscountValue: discValue,
			Subtotal:      lineNet,
			Description:   desc,
		})
		if len(lineComps) > 0 {
			componentMoves[len(lines)-1] = lineComps
		}
	}

	*res = saleLines{
		lines:           lines,
		subtotal:        subtotal,
		taxTotal:        taxTotal,
		itemDiscTotal:   itemDiscTotal,
		serviceProducts: serviceProducts,
		componentMoves:  componentMoves,
		serialBatches:   serialBatches,
	}
	return nil
}
