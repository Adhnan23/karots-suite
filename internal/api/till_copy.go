package api

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/features/denominations"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/plugin"

	"github.com/jmoiron/sqlx"
)

// BuildTillCopy reads a till's whole selling world: products priced exactly as
// the till prices them (fillPacks), customers, who may sign in, the money
// places, open sessions and held sales. For clients that keep a copy to sell
// from when the server can't be reached.
func BuildTillCopy(ctx context.Context, db *sqlx.DB) (*TillCopy, error) {
	version, err := TillCopyVersion(ctx, db)
	if err != nil {
		return nil, err
	}
	st, err := settings.NewService(db).Get(ctx)
	if err != nil {
		return nil, err
	}
	c := &TillCopy{TakenAt: time.Now().UTC().Format(time.RFC3339), Version: version, Symbol: st.CurrencySymbol,
		Shop:       ShopHeader{Name: st.ShopName, Address: strOr(st.Address), Phone: strOr(st.Phone), Footer: strOr(st.ReceiptFooter)},
		AskToPrint: st.AskToPrint, AllowUntrackedCash: st.AllowUntrackedCash, DefaultSaleType: "retail", TenderMethods: []TenderMethod{}}
	if st.DefaultSaleType == "wholesale" {
		c.DefaultSaleType = "wholesale"
	}
	c.LockTimeoutMinutes, c.AllowContinueLast = st.LockTimeoutMinutes, st.AllowContinueLastCount
	if st.DefaultLockerID != nil {
		c.DefaultLockerID = *st.DefaultLockerID
	}
	for _, t := range plugin.TenderMethods() {
		c.TenderMethods = append(c.TenderMethods, TenderMethod{Value: t.Value, Label: t.Label, Hidden: t.Hidden, OnPayments: t.OnPayments()})
	}
	if err := copyProducts(ctx, db, c); err != nil {
		return nil, err
	}
	if err := copyRows(ctx, db, c); err != nil {
		return nil, err
	}
	if err := copyMoney(ctx, db, c); err != nil {
		return nil, err
	}
	return c, nil
}

// copyMoney adds the lots a till asks Which price? about and the shop's notes
// and coins, both as the live till endpoints answer them.
func copyMoney(ctx context.Context, db *sqlx.DB, c *TillCopy) error {
	m, err := products.NewService(db).PriceOptions(ctx)
	if err != nil {
		return err
	}
	c.PriceOptions = make(map[string][]Lot, len(m))
	for pid, rows := range m {
		c.PriceOptions[strconv.FormatInt(pid, 10)] = lotsOf(rows)
	}
	dens, err := denominations.NewService(db).List(ctx, true)
	if err != nil {
		return err
	}
	c.Denominations = make([]Denomination, 0, len(dens))
	for _, d := range dens {
		c.Denominations = append(c.Denominations, Denomination{Value: d.Value.String(), IsNote: d.IsNote})
	}
	return nil
}

func copyProducts(ctx context.Context, db *sqlx.DB, c *TillCopy) error {
	rows, err := products.NewRepository(db).Sellable(ctx)
	if err != nil {
		return apperr.Internal("failed to load products", err)
	}
	products.Decorate(ctx, rows)
	tps := fillPacks(ctx, db, tillProducts(rows))
	ids := make([]int64, len(rows))
	for i, p := range rows {
		ids[i] = p.ID
	}
	packs, err := products.PacksFor(ctx, db, ids)
	if err != nil {
		return apperr.Internal("failed to load packs", err)
	}
	cats := map[int64]bool{}
	c.Products, c.Categories = make([]CopyProduct, 0, len(rows)), []CopyCategory{}
	for i, p := range rows {
		cp := CopyProduct{TillProduct: tps[i], CategoryID: p.CategoryID, Barcodes: []string(p.Barcodes), PackCodes: []PackCode{}}
		if cp.Barcodes == nil {
			cp.Barcodes = []string{}
		}
		for _, k := range packs[p.ID] {
			for _, b := range k.Barcodes {
				cp.PackCodes = append(cp.PackCodes, PackCode{PackID: k.ID, Barcode: b})
			}
		}
		c.Products = append(c.Products, cp)
		if !cats[p.CategoryID] {
			cats[p.CategoryID] = true
			c.Categories = append(c.Categories, CopyCategory{ID: p.CategoryID, Name: p.CategoryName})
		}
	}
	return nil
}

func copyRows(ctx context.Context, db *sqlx.DB, c *TillCopy) error {
	c.Customers, c.People, c.Places, c.Sessions, c.Held = []CopyCustomer{}, []CopyPerson{}, []CopyPlace{}, []CopySession{}, []CopyHeld{}
	if err := db.SelectContext(ctx, &c.Customers, `SELECT id, name, COALESCE(phone, '') AS phone,
		outstanding_balance::numeric(14,2)::text AS balance, credit_limit::numeric(14,2)::text AS creditlimit
		FROM customers WHERE is_active ORDER BY name`); err != nil {
		return apperr.Internal("failed to load customers", err)
	}
	// Never an admin or the system user: a PIN hash on a phone can be cracked.
	if err := db.SelectContext(ctx, &c.People, `SELECT id, name, phone, role::text AS role, pin_hash AS pinhash,
		can_manage_credit AS canmanagecredit, can_see_cost AS canseecost
		FROM users WHERE is_active AND NOT is_system AND role IN ('cashier', 'manager') ORDER BY name`); err != nil {
		return apperr.Internal("failed to load people", err)
	}
	if err := lastCounts(ctx, db, c.People); err != nil {
		return err
	}
	if err := db.SelectContext(ctx, &c.Places, `SELECT id, name, kind, cashier_access AS cashieraccess
		FROM lockers WHERE is_active ORDER BY name`); err != nil {
		return apperr.Internal("failed to load lockers", err)
	}
	reg := cashregister.NewService(db, sales.NewService(db))
	open, err := reg.OpenSessions(ctx)
	if err != nil {
		return err
	}
	for _, s := range open {
		sum, err := reg.Summary(ctx, s.UserID)
		if err != nil {
			return err
		}
		c.Sessions = append(c.Sessions, CopySession{ID: s.ID, UserID: s.UserID, OpenedAt: s.OpenedAt.Format(time.RFC3339),
			OpeningCash: dec2(s.OpeningCash), Expected: dec2(sum.Expected)})
	}
	if err := db.SelectContext(ctx, &c.Held, `SELECT id, cashier_id AS userid, COALESCE(label, '') AS label, sale_type AS saletype,
		COALESCE(customer_id, 0) AS customerid, discount::text AS discount, cart::text AS cart, item_count AS itemcount, total::text AS total,
		to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS createdat
		FROM held_sales ORDER BY id`); err != nil {
		return apperr.Internal("failed to load held sales", err)
	}
	return nil
}

// lastCounts gives each person their last close's count, note by note.
func lastCounts(ctx context.Context, db *sqlx.DB, people []CopyPerson) error {
	var rows []struct {
		UserID int64  `db:"user_id"`
		Counts []byte `db:"closing_breakdown"`
	}
	if err := db.SelectContext(ctx, &rows, `SELECT DISTINCT ON (user_id) user_id, closing_breakdown FROM cash_register
		WHERE closed_at IS NOT NULL AND closing_breakdown IS NOT NULL ORDER BY user_id, closed_at DESC`); err != nil {
		return apperr.Internal("failed to load last counts", err)
	}
	last := map[int64][]byte{}
	for _, r := range rows {
		last[r.UserID] = r.Counts
	}
	for i := range people {
		people[i].LastCount = map[string]float64{}
		if b := last[people[i].ID]; b != nil {
			_ = json.Unmarshal(b, &people[i].LastCount) // an unreadable count just isn't offered
		}
	}
	return nil
}

// TillCopyVersion is a cheap fingerprint of everything in the copy: it changes
// when anything a till sells from changes (a price, stock, a customer, a PIN,
// a session, a held sale), so a client refetches only then.
// ponytail: hashes whole tables each call (a few ms for a few thousand
// products); updated_at columns or a change counter if shops grow far past that.
func TillCopyVersion(ctx context.Context, db *sqlx.DB) (string, error) {
	var raw string
	err := db.GetContext(ctx, &raw, `SELECT concat_ws('|',
		(SELECT md5(COALESCE(string_agg(t::text, ',' ORDER BY t.id), '')) FROM products t),
		(SELECT md5(COALESCE(string_agg(t::text, ',' ORDER BY t.id), '')) FROM product_packs t),
		(SELECT md5(COALESCE(string_agg(t::text, ',' ORDER BY t.id), '')) FROM product_barcodes t),
		(SELECT md5(COALESCE(string_agg(t.product_id || ':' || t.quantity, ',' ORDER BY t.product_id), '')) FROM stock t),
		(SELECT md5(COALESCE(string_agg(concat_ws(':', t.id, t.selling_price, t.qty_remaining, t.expiry_date), ',' ORDER BY t.id), ''))
			FROM stock_batches t WHERE t.qty_remaining > 0),
		(SELECT md5(COALESCE(string_agg(t::text, ',' ORDER BY t.id), '')) FROM denominations t),
		(SELECT md5(COALESCE(string_agg(t::text, ',' ORDER BY t.id), '')) FROM customers t),
		(SELECT md5(COALESCE(string_agg(concat_ws(':', t.id, t.name, t.phone, t.role, t.pin_hash, t.is_active,
			t.can_manage_credit, t.can_see_cost), ',' ORDER BY t.id), '')) FROM users t),
		(SELECT md5(COALESCE(string_agg(t::text, ',' ORDER BY t.id), '')) FROM lockers t),
		(SELECT md5(COALESCE(string_agg(t::text, ',' ORDER BY t.id), '')) FROM cash_register t WHERE t.closed_at IS NULL),
		(SELECT md5(COALESCE(string_agg(t.id::text, ',' ORDER BY t.id), '')) FROM held_sales t),
		(SELECT COALESCE(max(id), 0) FROM sales), (SELECT COALESCE(max(id), 0) FROM cash_movements),
		(SELECT md5(t::text) FROM settings t WHERE id = 1))`)
	if err != nil {
		return "", apperr.Internal("failed to fingerprint the till", err)
	}
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:8]), nil
}
