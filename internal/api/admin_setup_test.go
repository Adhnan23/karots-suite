package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// A fresh shop has nothing done, and the hidden system user never counts as
// staff. Rolled back.
func TestSetupSteps(t *testing.T) {
	te := newTestEnv(t)
	ctx := context.Background()
	tx, err := te.db.BeginTxx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []string{
		`UPDATE users SET is_active = false WHERE NOT is_system`,
		`INSERT INTO users (name, phone, pin_hash, role, is_system) VALUES ('T sys', '0000000777', 'x', 'admin', true)`,
		`UPDATE settings SET shop_name = 'My Shop', receipt_printer = '', default_locker_id = NULL`,
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	done := map[string]bool{}
	for _, s := range setupSteps(ctx, tx) {
		done[s.Key] = s.Done
	}
	if done["staff"] || done["name"] || done["printer"] || done["locker"] {
		t.Fatalf("fresh shop: %v", done)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/setup", nil, te.login(t, fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000), "manager")); rec.Code != http.StatusForbidden {
		t.Fatalf("manager: %d", rec.Code)
	}
}
