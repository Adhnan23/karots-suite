package api

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Activity pages by a (when, source, id) cursor, so rows one transaction
// wrote at the same instant are each shown once; the system user never shows;
// the export has every match; managers can't open it.
func TestActivity(t *testing.T) {
	te := newTestEnv(t)
	phone := func() string { return fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000) }
	admin := te.login(t, phone(), "admin")
	manager := te.login(t, phone(), "manager")
	var who, sys int64
	te.db.Get(&who, `INSERT INTO users (name, phone, pin_hash, role) VALUES ('T act staff', $1, 'x', 'cashier') RETURNING id`, phone()) //nolint:errcheck
	te.db.Get(&sys, `SELECT id FROM users WHERE is_system LIMIT 1`)                                                                     //nolint:errcheck
	t.Cleanup(func() {
		te.db.Exec(`DELETE FROM audit_log WHERE entity = 'T-act'`) //nolint:errcheck
		te.db.Exec(`DELETE FROM users WHERE id = $1`, who)         //nolint:errcheck
	})
	// 120 audit rows from T act staff at one instant, and one from the system user.
	te.db.MustExec(`INSERT INTO audit_log (user_id, user_name, action, entity, entity_id, created_at)
		SELECT $1, 'T act staff', 'update', 'T-act', g::text, '2095-05-05T10:00:00+05:30' FROM generate_series(1, 120) g`, who)
	if sys > 0 {
		te.db.MustExec(`INSERT INTO audit_log (user_id, user_name, action, entity, entity_id, created_at) VALUES ($1, 'System', 'update', 'T-act', 'sys', '2095-05-05T10:00:00+05:30')`, sys)
	}
	base := "/api/v1/admin/activity?from=2095-05-01&to=2095-05-31&q=T-act"
	seen := map[string]bool{}
	next := ""
	for pages := 0; pages < 10; pages++ {
		u := base
		if next != "" {
			u += "&before=" + url.QueryEscape(next)
		}
		page := decode[ActivityPage](t, te.do(http.MethodGet, u, nil, admin))
		for _, r := range page.Rows {
			if seen[r.Detail] || strings.Contains(r.Detail, "#sys") {
				t.Fatalf("row %q twice or the system user's", r.Detail)
			}
			seen[r.Detail] = true
		}
		if next = page.Next; next == "" {
			break
		}
	}
	if len(seen) != 120 {
		t.Fatalf("saw %d of 120", len(seen))
	}
	f := decode[ActivityFilters](t, te.do(http.MethodGet, "/api/v1/admin/activity/filters", nil, admin))
	for _, u := range f.Users {
		if u.ID == sys {
			t.Fatal("the system user is a Who option")
		}
	}
	exp := te.do(http.MethodGet, base+"&format=csv", nil, admin)
	recs, _ := csv.NewReader(strings.NewReader(exp.Body.String())).ReadAll()
	if len(recs)-1 != 120 {
		t.Fatalf("export %d rows", len(recs)-1)
	}
	if c := te.do(http.MethodGet, base, nil, manager).Code; c != http.StatusForbidden {
		t.Fatalf("manager: %d", c)
	}
}
