package pocketcashier

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"karots-suite/internal/features/sales"
	"karots-suite/internal/plugin"
)

func TestSendsList(t *testing.T) {
	f := newRfix(t, "10")
	f.openAt(ago(time.Hour))
	f.send(nil, f.saleAct("one", ago(30*time.Minute), "1", "120"))
	f.send(nil, f.saleAct("two", ago(20*time.Minute), "1", "120"), f.saleAct("three", ago(19*time.Minute), "1", "120"))
	_, admin := f.en.user("admin")
	rows := decodeAs[[]SendRow](t, f.en.do(http.MethodGet, "/sends?limit=2", nil, admin, ""))
	if len(rows) != 2 || rows[0].Actions != 2 || rows[1].Actions != 1 || rows[0].Phone != "Phone T" {
		t.Fatalf("%+v", rows)
	}
}

func TestSendDetailFlags(t *testing.T) {
	f := newRfix(t, "10")
	f.openAt(ago(time.Hour))
	pcAt := ago(20 * time.Minute)
	if _, err := f.en.p.core.Sales.Create(context.Background(), sales.CreateInput{SaleType: "retail", At: &pcAt,
		Items: []sales.ItemInput{{ProductID: f.product, Quantity: "2"}}, Payments: []sales.PaymentInput{{Method: "cash", Amount: "240"}}}, f.cashier); err != nil {
		t.Fatal(err)
	}
	f.send(nil, f.saleAct("twin", pcAt.Add(time.Minute), "2", "240"))
	_, admin := f.en.user("admin")
	rows := decodeAs[[]SendRow](t, f.en.do(http.MethodGet, "/sends?limit=1", nil, admin, ""))
	d := decodeAs[SendDetail](t, f.en.do(http.MethodGet, "/sends/"+strconv.FormatInt(rows[0].ID, 10), nil, admin, ""))
	if len(d.Actions) != 1 || d.Actions[0].ReceiptNo == "" || len(d.Actions[0].Flags) == 0 || d.Actions[0].Flags[0].Kind != "lookalike" ||
		d.Actions[0].UserName == "" {
		t.Fatalf("%+v", d)
	}
}

func TestGuardBlocksUnreachablePhone(t *testing.T) {
	f := newRfix(t, "10")
	f.en.db.MustExec(`UPDATE users SET active_device = $1, active_device_label = 'Phone T' WHERE id = $2`, f.device, f.cashier)
	f.en.db.MustExec(`UPDATE pocket_phones SET user_id = $1, last_seen = now() - interval '2 minutes' WHERE device = $2`, f.cashier, f.device)
	guard := plugin.TakeoverGuards()
	if len(guard) == 0 {
		t.Fatal("no takeover guard registered")
	}
	block, reason := guard[len(guard)-1](context.Background(), f.cashier, f.device)
	if !block || reason == "" {
		t.Fatalf("2 min ago: %v %q", block, reason)
	}
	f.en.db.MustExec(`UPDATE pocket_phones SET last_seen = now() - interval '10 seconds' WHERE device = $1`, f.device)
	if block, _ := guard[len(guard)-1](context.Background(), f.cashier, f.device); block {
		t.Fatal("a reachable phone must hand the person back")
	}
	if block, _ := guard[len(guard)-1](context.Background(), f.cashier, "some-pc"); block {
		t.Fatal("a PC device is not this plugin's to hold")
	}
}
