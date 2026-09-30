package api

import (
	"net/http"
	"testing"

	"karots-suite/internal/features/cashflow"
)

func TestAdminMoneySources(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009601", "admin")
	var lid int64
	if err := te.db.Get(&lid, `INSERT INTO lockers (name, kind) VALUES ('T safe money-src', 'safe') RETURNING id`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { te.db.Exec(`DELETE FROM lockers WHERE id=$1`, lid) }) //nolint:errcheck
	found := false
	for _, s := range decode[[]MoneySource](t, te.do(http.MethodGet, "/api/v1/admin/money-sources", nil, ck)) {
		found = found || (s.Label == "T safe money-src" && s.Group == "Lockers" && s.Kind == "safe" && s.Balance == "0.00")
	}
	if !found {
		t.Fatal("locker missing from money sources")
	}
	for v, want := range map[string]cashflow.Location{"": cashflow.External(), "locker:3": cashflow.Locker(3), "till:5": cashflow.Till(5)} {
		if got, err := parseLocation(v); err != nil || got != want {
			t.Fatalf("parseLocation(%q) = %v, %v", v, got, err)
		}
	}
	for _, v := range []string{"locker:x", "bank:1", "till:0"} {
		if _, err := parseLocation(v); err == nil {
			t.Fatalf("parseLocation(%q) accepted", v)
		}
	}
}
