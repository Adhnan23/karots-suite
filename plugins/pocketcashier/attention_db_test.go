package pocketcashier

import (
	"context"
	"strings"
	"testing"
)

// A phone waiting for approval shows on the admin dashboard.
func TestAttentionPhonesWaiting(t *testing.T) {
	en := newEnv(t)
	en.db.Exec(`DELETE FROM pocket_phones WHERE status = 'pending'`) //nolint:errcheck
	items, err := en.p.attention(context.Background())
	if err != nil || len(items) != 0 {
		t.Fatalf("nothing waiting: %+v %v", items, err)
	}
	en.pairPhone()
	items, err = en.p.attention(context.Background())
	if err != nil || len(items) != 1 || !strings.Contains(items[0].Text, "1 phone is waiting") || items[0].To != "/admin/x/pocketcashier" {
		t.Fatalf("%+v %v", items, err)
	}
}
