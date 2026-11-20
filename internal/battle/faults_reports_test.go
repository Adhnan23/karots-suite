package battle

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"karots-suite/internal/datetime"
)

// RPT-36: one page is one moment. Two tills sell and take goods back without a
// break while the Finance page and the dashboard are read again and again:
// every answer agrees with itself — its trend and categories add up to its
// revenue, its by-method lines to what it received, the dashboard's spark for
// today to its today.
func TestRPT36_AReportIsOneMoment(t *testing.T) {
	t.Parallel()
	s := newShop(t)
	s.open(t, s.till1)
	s.open(t, s.till2)
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for _, c := range []*client{s.till1, s.till2} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				var r saleResult
				if c.Do("POST", "/api/v1/till/sales", map[string]any{"saleType": "retail", "discount": "3", "discountType": "fixed",
					"items":    []line{{"productId": s.prod["plain"], "quantity": "2"}, {"productId": s.prod["odd"], "quantity": "1"}},
					"payments": []pay{cash("300")}}, &r) != nil {
					continue
				}
				var v sale
				if c.Do("GET", fmt.Sprintf("/api/v1/till/sales/%d", r.ID), nil, &v) == nil {
					c.Do("POST", fmt.Sprintf("/api/v1/till/sales/%d/return", r.ID), map[string]any{ //nolint:errcheck
						"lines": []map[string]any{{"saleItemId": v.Lines[0].ID, "quantity": "1"}}}, nil)
				}
			}
		}()
	}
	today := time.Now().In(datetime.Location).Format(time.DateOnly)
	sum := func(xs []struct{ Revenue string }) decimal.Decimal {
		d := decimal.Zero
		for _, x := range xs {
			d = d.Add(dec(t, x.Revenue))
		}
		return d
	}
	broken := 0
	for end := time.Now().Add(3 * time.Second); time.Now().Before(end) && broken < 3; {
		var f struct {
			Revenue, Received string
			Trend, Categories []struct{ Revenue string }
			ByPayment         []struct{ Method, Total string }
		}
		s.must(t, s.admin.Do("GET", "/api/v1/admin/finance?from="+today+"&to="+today, nil, &f))
		rev, recv := dec(t, f.Revenue), dec(t, f.Received)
		paid := decimal.Zero
		for _, m := range f.ByPayment {
			if m.Method != "credit" {
				paid = paid.Add(dec(t, m.Total))
			}
		}
		if !sum(f.Trend).Equal(rev) || !sum(f.Categories).Equal(rev) || !paid.Equal(recv) {
			broken++
			t.Errorf("RPT-36 one Finance answer disagrees with itself: revenue %s, trend %s, categories %s; received %s, by method %s",
				rev, sum(f.Trend), sum(f.Categories), recv, paid)
		}
		var d struct {
			Today struct{ Revenue string }
			Spark []struct {
				Day, Revenue string
			}
		}
		s.must(t, s.admin.Do("GET", "/api/v1/admin/dashboard", nil, &d))
		if n := len(d.Spark); n == 0 || d.Spark[n-1].Day != today || d.Spark[n-1].Revenue != d.Today.Revenue {
			broken++
			t.Errorf("RPT-36 the dashboard's today %s, its spark's today %+v", d.Today.Revenue, d.Spark[len(d.Spark)-1])
		}
	}
	close(stop)
	wg.Wait()
	s.audit(t)
	s.agreeToday(t)
}
