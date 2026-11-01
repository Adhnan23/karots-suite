package pocketcashier

import (
	"context"
	"fmt"

	"karots-suite/internal/plugin"
)

// attention: phones waiting for an admin's approval, on the dashboard's "Needs
// your attention" (a cashier at the phone is waiting too).
func (p *Plugin) attention(ctx context.Context) ([]plugin.AttentionItem, error) {
	var n int
	if err := p.core.DB.GetContext(ctx, &n, `SELECT count(*) FROM pocket_phones WHERE status = 'pending'`); err != nil || n == 0 {
		return nil, err
	}
	text := fmt.Sprintf("%d phones are waiting for approval", n)
	if n == 1 {
		text = "1 phone is waiting for approval"
	}
	return []plugin.AttentionItem{{Tag: "Phones", Text: text, To: "/admin/x/pocketcashier", Go: "Approve", Tone: "warn"}}, nil
}
