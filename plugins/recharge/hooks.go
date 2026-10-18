package recharge

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/money"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"
)

// lineData is what the till puts on a recharge cart line (CartLine.plugin.recharge).
// Kind: reload | package | billpay | deposit | cashout | charge. A bill or cash out
// names a device (its float) or a bank locker (core money).
type lineData struct {
	Kind     string `json:"kind"`
	Device   int64  `json:"device,omitempty"`
	Bank     int64  `json:"bank,omitempty"`
	Package  int64  `json:"package,omitempty"`
	Number   string `json:"number,omitempty"`
	BillType string `json:"billType,omitempty"`
	Ref      string `json:"ref,omitempty"`
}

// recordSale books each recharge line of a sale inside its tx: the device's float
// (and the carrier's commission), or the bank locker's money through core
// cashflow. It never refuses on a low balance — the money already moved on the
// phone or in the bank's app; a float below zero shows red and asks for a count.
func (p *Plugin) recordSale(ctx context.Context, tx *sqlx.Tx, ev plugin.SaleLines) error {
	for _, l := range ev.Lines {
		var ld lineData
		if err := json.Unmarshal(l.Data, &ld); err != nil {
			return apperr.Validation("a reload line lost its details — add it again")
		}
		if ld.Kind == "charge" {
			continue // the service charge is ordinary revenue
		}
		saleID, itemID := ev.SaleID, l.ItemID
		amt := l.Amount
		if ld.Kind == KindCashout {
			amt = amt.Neg() // a payout line is negative
		}
		if ld.Bank > 0 && (ld.Kind == KindBillpay || ld.Kind == KindCashout) {
			if err := p.bankLeg(ctx, tx, ev.ActorID, saleID, itemID, ld, amt); err != nil {
				return err
			}
			continue
		}
		dv, err := p.store.Device(ctx, tx, ld.Device)
		if err != nil {
			return err
		}
		rs, err := p.store.Rates(ctx, tx, dv.CarrierID)
		if err != nil {
			return err
		}
		in := TxInput{CarrierID: dv.CarrierID, DeviceID: dv.ID, Type: ld.Kind, Amount: amt, FloatDelta: amt.Neg(),
			SaleID: &saleID, SaleItemID: &itemID, CustomerNumber: strings.TrimSpace(ld.Number), BillType: strings.TrimSpace(ld.BillType),
			Reference: strings.TrimSpace(ld.Ref), CreatedBy: ev.ActorID}
		var comm decimal.Decimal
		switch ld.Kind {
		case KindReload, KindBillpay, KindDeposit:
			comm = rateOf(rs, ld.Kind).Commission(amt)
		case KindCashout:
			in.FloatDelta = amt // the customer's e-money arrives on the device
			comm = rateOf(rs, KindCashout).Commission(amt)
		case "package":
			pk, err := p.store.pkg(ctx, tx, ld.Package)
			if err != nil {
				return err
			}
			in.Type, in.PackageID, in.FloatDelta = KindReload, &pk.ID, pk.FloatCost.Neg()
			comm = packageCommission(amt, pk.FloatCost)
		default:
			return apperr.Validation("unknown reload line: " + ld.Kind)
		}
		if _, err := recordWithCommission(ctx, tx, in, comm); err != nil {
			return err
		}
	}
	return nil
}

// bankLeg moves a bill paid from (or a cash out into) a bank locker through core
// cashflow, as already done in the bank's app (Recorded: no overdraw refusal),
// and keeps the bill detail.
func (p *Plugin) bankLeg(ctx context.Context, tx *sqlx.Tx, actor, saleID, itemID int64, ld lineData, amt decimal.Decimal) error {
	var bank struct {
		Name   string `db:"name"`
		Usable bool   `db:"usable"`
	}
	err := tx.GetContext(ctx, &bank, `SELECT name, (kind = 'bank' AND is_active AND cashier_access) AS usable FROM lockers WHERE id = $1`, ld.Bank)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && !bank.Usable) {
		return apperr.Validation("that bank account can't be used at the till")
	}
	if err != nil {
		return err
	}
	typ, party := "billpay", "Bill payment"
	if r := strings.TrimSpace(ld.Ref); r != "" {
		party = "Bill " + r
	}
	mv := cashflow.MoveInput{From: cashflow.Locker(ld.Bank), To: cashflow.External(), Amount: amt, Reason: bank.Name + " bill payment",
		ReceiptKind: "payment", Party: party, ActorID: actor, Recorded: true, Ref: &cashflow.Ref{Kind: "sale", ID: saleID}}
	if ld.Kind == KindCashout {
		typ = "getmoney"
		mv.From, mv.To, mv.Reason, mv.ReceiptKind, mv.Party = cashflow.External(), cashflow.Locker(ld.Bank), bank.Name+" cash out", "intake", "Customer"
	}
	if _, err := p.core.Cashflow.MoveTx(ctx, tx, mv); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO recharge_bill_tx (bank_locker_id, bank_name, type, amount, reference, bill_type, created_by, sale_id, sale_item_id)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9)`,
		ld.Bank, bank.Name, typ, amt, strings.TrimSpace(ld.Ref), strings.TrimSpace(ld.BillType), actor, saleID, itemID)
	return err
}

// returnData is what the Returns field sent for a recharge line: how much float
// (or bank money) came back — "all" (default), "none" (wrong number) or an amount.
type returnData struct {
	Back string `json:"back"`
}

// backOf reads the field against what left (for the units returned), clamped to it.
func backOf(raw json.RawMessage, left decimal.Decimal) decimal.Decimal {
	var rd returnData
	_ = json.Unmarshal(raw, &rd)
	switch b := strings.TrimSpace(rd.Back); b {
	case "", "all":
		return left
	case "none":
		return decimal.Zero
	default:
		v, err := money.Parse(b)
		if err != nil || v.IsNegative() {
			return left
		}
		return decimal.Min(v, left)
	}
}

// recordReturn puts back what the carrier (or the bank) returned for recharge
// lines coming back through core Returns: a 'reversal' float row (+ the share of
// its commission taken back), or the bank money in. The refund itself is core's.
func (p *Plugin) recordReturn(ctx context.Context, tx *sqlx.Tx, ev plugin.ReturnLines) error {
	for _, l := range ev.Lines {
		var orig struct {
			ID        int64           `db:"id"`
			CarrierID int64           `db:"carrier_id"`
			DeviceID  int64           `db:"device_id"`
			Float     decimal.Decimal `db:"float_delta"`
			Qty       decimal.Decimal `db:"qty"`
		}
		err := tx.GetContext(ctx, &orig, `SELECT t.id, t.carrier_id, t.device_id, t.float_delta, si.quantity AS qty
			FROM recharge_transactions t JOIN sale_items si ON si.id = t.sale_item_id
			WHERE t.sale_item_id = $1 AND t.parent_id IS NULL AND t.type IN ('reload','billpay','deposit') LIMIT 1`, l.SaleItemID)
		if errors.Is(err, sql.ErrNoRows) {
			if err := p.bankReturn(ctx, tx, ev, l); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		left := orig.Float.Neg().Mul(l.Quantity).Div(orig.Qty).Round(2)
		back := backOf(l.Data, left)
		saleID, itemID := ev.SaleID, l.SaleItemID
		typ := "reversal"
		if back.IsZero() {
			typ = "reversal_lost" // wrong number: the float is gone, the commission was earned
		}
		if _, err := recordTx(ctx, tx, TxInput{CarrierID: orig.CarrierID, DeviceID: orig.DeviceID, Type: typ, Amount: left, FloatDelta: back,
			SaleID: &saleID, SaleItemID: &itemID, ParentID: &orig.ID, Note: "returned · return #" + strconv.FormatInt(ev.ReturnID, 10), CreatedBy: ev.ActorID}); err != nil {
			return err
		}
		var comm decimal.Decimal
		if err := tx.GetContext(ctx, &comm, `SELECT COALESCE(SUM(float_delta), 0) FROM recharge_transactions WHERE parent_id = $1 AND type = 'commission'`, orig.ID); err != nil {
			return err
		}
		if take := comm.Mul(back).Div(orig.Float.Neg()).Round(2); !take.IsZero() && orig.Float.IsNegative() {
			if _, err := recordTx(ctx, tx, TxInput{CarrierID: orig.CarrierID, DeviceID: orig.DeviceID, Type: "commission", Amount: take.Abs(),
				FloatDelta: take.Neg(), SaleID: &saleID, SaleItemID: &itemID, ParentID: &orig.ID, Note: "commission back on a return", CreatedBy: ev.ActorID}); err != nil {
				return err
			}
		}
	}
	return nil
}

// bankReturn: a bill paid from a bank coming back puts what the bank returned
// into it again (Recorded); lines that aren't recharge's are left alone.
func (p *Plugin) bankReturn(ctx context.Context, tx *sqlx.Tx, ev plugin.ReturnLines, l plugin.ReturnLine) error {
	var b struct {
		ID     int64           `db:"id"`
		Bank   int64           `db:"bank_locker_id"`
		Amount decimal.Decimal `db:"amount"`
		Qty    decimal.Decimal `db:"qty"`
	}
	err := tx.GetContext(ctx, &b, `SELECT b.id, b.bank_locker_id, b.amount, si.quantity AS qty FROM recharge_bill_tx b JOIN sale_items si ON si.id = b.sale_item_id
		WHERE b.sale_item_id = $1 AND b.type = 'billpay' AND b.bank_locker_id IS NOT NULL`, l.SaleItemID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	back := backOf(l.Data, b.Amount.Mul(l.Quantity).Div(b.Qty).Round(2))
	if !back.IsPositive() {
		return nil
	}
	if _, err := p.core.Cashflow.MoveTx(ctx, tx, cashflow.MoveInput{From: cashflow.External(), To: cashflow.Locker(b.Bank), Amount: back,
		Reason: "bill payment returned", ReceiptKind: "intake", Party: "Bill refund", ActorID: ev.ActorID, Recorded: true,
		Ref: &cashflow.Ref{Kind: "sale", ID: ev.SaleID}}); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE recharge_bill_tx SET returned = returned + $2 WHERE id = $1`, b.ID, back)
	return err
}

// settleWallet credits the device a customer paid by eZ Cash / mCash: a sale
// tender or a credit repayment, with the carrier's wallet-payment commission.
func (p *Plugin) settleWallet(ctx context.Context, tx *sqlx.Tx, ev plugin.TenderSettle) error {
	id, _ := strconv.ParseInt(ev.Meta["device"], 10, 64)
	if id <= 0 {
		return apperr.Validation("pick the device the wallet money came to")
	}
	dv, err := p.store.Device(ctx, tx, id)
	if err != nil {
		return err
	}
	rs, err := p.store.Rates(ctx, tx, dv.CarrierID)
	if err != nil {
		return err
	}
	in := TxInput{CarrierID: dv.CarrierID, DeviceID: dv.ID, Type: KindWalletIn, Amount: ev.Amount, FloatDelta: ev.Amount, CreatedBy: ev.ActorID,
		Reference: strings.TrimSpace(ev.Meta["ref"])}
	if ev.SaleID > 0 {
		in.SaleID = &ev.SaleID
	} else {
		in.Note = "credit payment #" + strconv.FormatInt(ev.CustomerPaymentID, 10)
	}
	_, err = recordWithCommission(ctx, tx, in, rateOf(rs, KindWalletIn).Commission(ev.Amount))
	return err
}
