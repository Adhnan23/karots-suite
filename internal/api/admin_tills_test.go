package api

import (
	"net/http"
	"strconv"
	"testing"
)

func TestAdminTills(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009616", "admin")
	uid := te.seedUser(t, "0770009617", "cashier")
	t.Cleanup(func() { te.db.Exec(`DELETE FROM cash_register WHERE user_id=$1`, uid) }) //nolint:errcheck
	var last int64
	for i := range 3 {
		if err := te.db.Get(&last, `INSERT INTO cash_register (user_id, opening_cash, opening_breakdown, closing_cash, expected_cash, difference, closing_breakdown, opened_at, closed_at)
			VALUES ($1, 1000, '{"500": 2}', 1450, 1500, -50, '{"1000": 1, "50": 9}', now() - ($2 || ' hours')::interval, now()) RETURNING id`, uid, strconv.Itoa(3-i)); err != nil {
			t.Fatal(err)
		}
	}
	list := func(q string) []TillSessionRow {
		return decode[[]TillSessionRow](t, te.do(http.MethodGet, "/api/v1/admin/tills?user="+strconv.FormatInt(uid, 10)+q, nil, ck))
	}
	if rows := list("&preset=today"); len(rows) != 3 || rows[0].ID != last || rows[0].Difference != "-50.00" || rows[0].Counted != "1450.00" || rows[0].Open {
		t.Fatalf("list %+v", rows)
	}
	if rows := list("&offset=2"); len(rows) != 1 {
		t.Fatalf("offset page %+v", rows)
	}
	if rows := list("&open=1"); len(rows) != 0 {
		t.Fatalf("open only %+v", rows)
	}
	v := decode[TillSessionView](t, te.do(http.MethodGet, "/api/v1/admin/tills/"+strconv.FormatInt(last, 10), nil, ck))
	if v.OpeningCount["500"] != 2 || v.ClosingCount["50"] != 9 || v.Session.Cashier == "" {
		t.Fatalf("detail %+v", v)
	}

	// Denominations: a value is unique; one used in a saved count can't be deleted.
	var used int64
	if err := te.db.Get(&used, `INSERT INTO denominations (value, is_note) VALUES (50, false) ON CONFLICT (value) DO UPDATE SET value = EXCLUDED.value RETURNING id`); err != nil {
		t.Fatal(err)
	}
	rec := te.do(http.MethodPost, "/api/v1/admin/denominations", DenominationRequest{Value: "7777", IsNote: true, Active: true}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	d := decode[AdminDenomination](t, rec)
	t.Cleanup(func() { te.db.Exec(`DELETE FROM denominations WHERE value = 7777`) }) //nolint:errcheck
	if c := te.do(http.MethodPost, "/api/v1/admin/denominations", DenominationRequest{Value: "7777"}, ck).Code; c != http.StatusConflict {
		t.Fatalf("duplicate: %d", c)
	}
	if c := te.do(http.MethodPut, "/api/v1/admin/denominations/"+strconv.FormatInt(d.ID, 10), DenominationRequest{Value: "7777", IsNote: true}, ck).Code; c != http.StatusNoContent {
		t.Fatalf("toggle: %d", c)
	}
	if c := te.do(http.MethodDelete, "/api/v1/admin/denominations/"+strconv.FormatInt(used, 10), nil, ck).Code; c != http.StatusConflict {
		t.Fatalf("delete used: %d", c)
	}
	if c := te.do(http.MethodDelete, "/api/v1/admin/denominations/"+strconv.FormatInt(d.ID, 10), nil, ck).Code; c != http.StatusNoContent {
		t.Fatalf("delete unused: %d", c)
	}
}
