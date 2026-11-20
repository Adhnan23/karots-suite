package battle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

// Power cuts (Iman has generators, no UPS): the real server, killed without
// warning mid-sale, mid-return, mid-move, mid-payment and mid-delivery, then started again. Every sale, return and
// payment is whole or absent — the auditor finds nothing — however often it
// happens. A long test: it runs in make battle (KS_BATTLE_STEPS), not make test.
func TestBattlePowerCut(t *testing.T) {
	t.Parallel()
	if os.Getenv("KS_BATTLE_STEPS") == "" {
		t.Skip("power cuts run in make battle")
	}
	s := newShop(t)
	safe := s.locker(t, "Safe", "safe", "100000")
	s.must(t, sqlxExec(context.Background(), s.db, `UPDATE users SET can_handle_suppliers = true`)) // counter expenses
	stk := s.stockForCuts(t)
	buy := s.buyingForCuts(t, safe)
	bin := serverBinary(t)
	cuts := max(steps()/200, 10)
	for cut := 0; cut < cuts; cut++ {
		srv := startServer(t, bin, s.dsn)
		c := httpClient(t, srv.url, "0770000002")
		if cut == 0 {
			c.post("/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 5}}) //nolint:errcheck
		}
		var wg sync.WaitGroup
		stop := make(chan struct{})
		admin := httpClient(t, srv.url, "0770000001")
		for w := 0; w < 6; w++ {
			wg.Add(1)
			if w == 5 { // area 5: deliveries, supplier payments, refunds and goods sent back
				go func() { defer wg.Done(); buy.busy(admin, stop) }()
				continue
			}
			if w == 4 { // area 4: counts, write-offs and bags opened while the power goes
				go func() { defer wg.Done(); stk.busy(admin, stop) }()
				continue
			}
			if w == 2 { // area 2: money moves while the power goes
				go func() { defer wg.Done(); busyMoney(c, safe, stop) }()
				continue
			}
			if w == 3 { // area 3: a customer borrows, pays and is paid back
				go func() { defer wg.Done(); s.busyCustomer(c, stop) }()
				continue
			}
			go func() { defer wg.Done(); s.busyTill(c, stop) }()
		}
		time.Sleep(time.Duration(30+rand.IntN(400)) * time.Millisecond)
		srv.cmd.Process.Signal(syscall.SIGKILL) //nolint:errcheck
		srv.cmd.Wait()                          //nolint:errcheck
		close(stop)
		wg.Wait()
		s.quiet(t) // a COMMIT sent just before the kill still lands a moment after it
		s.audit(t)
		s.agreeToday(t) // no report counts half of anything (RPT-36, area 7)
		if t.Failed() {
			t.Fatalf("after power cut %d", cut+1)
		}
	}
	var n int
	s.must(t, s.db.Get(&n, `SELECT count(*) FROM sales`))
	var moves int
	s.must(t, s.db.Get(&moves, `SELECT count(*) FROM stock_movements WHERE note IN ('cut count', 'stock-take') OR type IN ('damage', 'conversion')`))
	var buys, pays int
	s.must(t, s.db.Get(&buys, `SELECT count(*) FROM purchases`))
	s.must(t, s.db.Get(&pays, `SELECT count(*) FROM supplier_payments`))
	t.Logf("%d power cuts, %d sales, %d stock moves, %d deliveries and %d supplier payments survived", cuts, n, moves, buys, pays)
}

// busyMoney moves cash in and out of the drawer, pays expenses from it and
// closes / reopens the till (banked into the safe) until the power goes: every
// move whole or absent (DRAW-1, DRAW-9, DRAW-10, LOCK-3, EXP-1).
func busyMoney(c *hclient, safe int64, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		switch rand.IntN(5) {
		case 0:
			c.post("/api/v1/till/drawer/withdraw", map[string]any{"amount": "10.25", "reason": "cut", "lockerId": safe}) //nolint:errcheck
		case 1:
			c.post("/api/v1/till/drawer/deposit", map[string]any{"amount": "7.50", "reason": "cut", "lockerId": safe}) //nolint:errcheck
		case 2:
			c.post("/api/v1/till/expenses", map[string]any{"category": "Cut", "amount": "3.10"}) //nolint:errcheck
		case 3:
			var st struct{ Summary struct{ Expected string } }
			if c.get("/api/v1/till/drawer", &st) == nil && st.Summary.Expected != "" {
				c.post("/api/v1/till/drawer/close", map[string]any{"breakdown": map[string]int{st.Summary.Expected: 1}, "destLockerId": safe}) //nolint:errcheck
			}
		case 4:
			c.post("/api/v1/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 5}, "sourceLockerId": safe}) //nolint:errcheck
		}
	}
}

// busyCustomer puts sales on account, takes payments by cash and cheque and
// hands an advance back until the power goes: each whole or absent (CUST-1,
// CUST-5, CUST-6).
func (s *shop) busyCustomer(c *hclient, stop chan struct{}) {
	payPath := fmt.Sprintf("/api/v1/till/customers/%d/payment", s.customer)
	for i := 0; ; i++ {
		select {
		case <-stop:
			return
		default:
		}
		switch rand.IntN(4) {
		case 0:
			c.post("/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed", "customerId": s.customer, //nolint:errcheck
				"items": []line{{"productId": s.prod["plain"], "quantity": "1"}}, "payments": []pay{cash("40"), {"method": "credit", "amount": "60"}}})
		case 1:
			c.post(payPath, map[string]any{"amount": "45.50", "method": "cash", "clientRef": fmt.Sprintf("cut-%d-%d", time.Now().UnixNano(), i)}) //nolint:errcheck
		case 2:
			c.post(payPath, map[string]any{"method": "cheque", "cheques": []map[string]string{{"bank": "BOC", "number": fmt.Sprint(rand.IntN(1e9)), //nolint:errcheck
				"amount": "20", "date": "2026-12-01"}}})
		case 3:
			c.post(fmt.Sprintf("/api/v1/till/customers/%d/refund", s.customer), map[string]any{"amount": "5"}) //nolint:errcheck
		}
	}
}

// busyTill sells and returns as fast as it can until the power goes.
func (s *shop) busyTill(c *hclient, stop chan struct{}) {
	var last int64
	for {
		select {
		case <-stop:
			return
		default:
		}
		if last != 0 && rand.IntN(3) == 0 {
			var v sale
			if c.get(fmt.Sprintf("/api/v1/till/sales/%d", last), &v) == nil && len(v.Lines) > 0 {
				c.post(fmt.Sprintf("/api/v1/till/sales/%d/return", last), map[string]any{ //nolint:errcheck
					"lines": []map[string]any{{"saleItemId": v.Lines[0].ID, "quantity": "1"}}})
			}
			continue
		}
		var r saleResult
		err := c.postInto("/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "3", "discountType": "fixed",
			"items":    []line{{"productId": s.prod["plain"], "quantity": "2"}, {"productId": s.prod["odd"], "quantity": "1", "discount": "5", "discountType": "percent"}},
			"payments": []pay{cash("300")}}, &r)
		if err == nil {
			last = r.ID
		}
	}
}

var buildOnce struct {
	sync.Once
	path string
	err  error
}

// serverBinary builds cmd/server once (core only, as the working tree has it).
func serverBinary(t *testing.T) string {
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "battle-server-")
		if err != nil {
			buildOnce.err = err
			return
		}
		buildOnce.path = filepath.Join(dir, "karots-suite")
		out, err := exec.Command("go", "build", "-o", buildOnce.path, "karots-suite/cmd/server").CombinedOutput()
		if err != nil {
			buildOnce.err = fmt.Errorf("%v: %s", err, out)
		}
	})
	if buildOnce.err != nil {
		t.Fatal(buildOnce.err)
	}
	return buildOnce.path
}

var serverLog string // the latest server's output (power cuts run one server at a time)

func tailOf(b []byte, n int) []byte {
	if len(b) > n {
		return b[len(b)-n:]
	}
	return b
}

type server struct {
	cmd *exec.Cmd
	url string
}

func startServer(t *testing.T, bin, dsn string) server {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), "DATABASE_URL="+dsn, fmt.Sprintf("SERVER_PORT=%d", port), "APP_ENV=development",
		"COOKIE_SECURE=false", "JWT_SECRET=battle-battle-battle-battle-battle-battle", "KAROTS_NO_SPOOL=1", "BACKUP_DIR="+t.TempDir(),
		"BACKUP_INTERVAL=0", "CLOUD_BACKUP=", "BACKUP_COPY_DIR=")
	logf, err := os.Create(filepath.Join(t.TempDir(), "server.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stdout, cmd.Stderr = logf, logf
	serverLog = logf.Name()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	for i := 0; i < 300; i++ {
		if r, err := http.Get(url + "/api/v1/me"); err == nil {
			r.Body.Close()
			return server{cmd: cmd, url: url}
		}
		time.Sleep(50 * time.Millisecond)
	}
	cmd.Process.Kill() //nolint:errcheck
	t.Fatal("server didn't start")
	return server{}
}

// hclient is a till over real HTTP (the in-process gen.Client can't lose power).
type hclient struct {
	base string
	http *http.Client
}

func httpClient(t *testing.T, base, phone string) *hclient {
	jar, _ := cookiejar.New(nil)
	c := &hclient{base: base, http: &http.Client{Jar: jar, Timeout: 5 * time.Second}}
	// A fresh server can drop its first connection (EOF) while it settles: try again for a moment.
	var err error
	for range 20 {
		if err = c.postInto("/api/v1/auth/login", map[string]string{"phone": phone, "pin": "1234"}, nil); err == nil {
			return c
		}
		time.Sleep(100 * time.Millisecond)
	}
	out, _ := os.ReadFile(serverLog)
	t.Fatalf("%v\nserver log:\n%s", err, tailOf(out, 3000))
	return c
}

func (c *hclient) post(path string, in any) error { return c.postInto(path, in, nil) }

func (c *hclient) postInto(path string, in, out any) error {
	b, _ := json.Marshal(in)
	req, _ := http.NewRequestWithContext(context.Background(), "POST", c.base+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ks-Device", "battle-powercut")
	return c.do(req, out)
}

func (c *hclient) del(path string) error {
	req, _ := http.NewRequestWithContext(context.Background(), "DELETE", c.base+path, nil)
	req.Header.Set("X-Ks-Device", "battle-powercut")
	return c.do(req, nil)
}

func (c *hclient) get(path string, out any) error {
	req, _ := http.NewRequestWithContext(context.Background(), "GET", c.base+path, nil)
	req.Header.Set("X-Ks-Device", "battle-powercut")
	return c.do(req, out)
}

func (c *hclient) do(req *http.Request, out any) error {
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode > 299 {
		return fmt.Errorf("%s %s: %d", req.Method, req.URL.Path, res.StatusCode)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}

// cutStock is what the stock worker counts, writes off and opens.
type cutStock struct{ product, lot, conv int64 }

func (s *shop) stockForCuts(t *testing.T) cutStock {
	p := s.stocked(t, "Cut soap", s.unitPcs, 0, nil)
	s.addLot(t, p, "100000", "10", "")
	bag := s.stocked(t, "Cut bag", s.unitPcs, 0, nil)
	s.addLot(t, bag, "100000", "2500", "")
	loose := s.stocked(t, "Cut loose", s.unitKg, 0, nil)
	var cv struct{ ID int64 }
	s.must(t, s.admin.Do("POST", "/api/v1/admin/conversions", map[string]any{"fromProductId": bag, "toProductId": loose, "ratio": "25"}, &cv))
	return cutStock{product: p, lot: s.lotIDs(t, p)[0], conv: cv.ID}
}

// busy counts, counts a lot, writes off and opens bags until the power goes: each
// whole or absent (STK-1, STK-2, STK-23).
func (k cutStock) busy(c *hclient, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		default:
		}
		switch rand.IntN(4) {
		case 0:
			c.post("/api/v1/admin/stock/adjust", map[string]any{"productId": k.product, "newQuantity": fmt.Sprint(90000 + rand.IntN(1000)), "note": "cut count"}) //nolint:errcheck
		case 1:
			c.post("/api/v1/admin/stock/take", map[string]any{"lines": []map[string]any{{"productId": k.product, //nolint:errcheck
				"batches": []map[string]any{{"batchId": k.lot, "counted": fmt.Sprint(50000 + rand.IntN(1000))}}}}})
		case 2:
			c.post("/api/v1/admin/stock/damage", map[string]any{"productId": k.product, "quantity": "1", "reason": "damage"}) //nolint:errcheck
		case 3:
			c.post(fmt.Sprintf("/api/v1/admin/conversions/%d/run", k.conv), map[string]string{"quantity": "1", "toQty": "24.6"}) //nolint:errcheck
		}
	}
}

// quiet waits until no other connection to the shop's database is still at
// work: Postgres finishes a COMMIT the killed server had already sent, a few
// milliseconds after the kill, so checking at once can count a sale the next
// read sees (make battle, 2026-11-20: 30 sales, then 31).
func (s *shop) quiet(t *testing.T) {
	t.Helper()
	for i := 0; i < 100; i++ {
		var busy int
		s.must(t, s.db.Get(&busy, `SELECT count(*) FROM pg_stat_activity
			WHERE datname = current_database() AND pid <> pg_backend_pid() AND state <> 'idle'`))
		if busy == 0 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("the database never went quiet after the power cut")
}
