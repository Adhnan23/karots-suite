// Command bench times a running Karots Suite the way a shop uses it: till
// searches, sales, the year's big reports and a backup, through the real API.
// It is how the hardware minimums in docs/guide/install.md were found: run it
// against a PC (or a VM slowed down to act like one) holding a full shop.
//
//	go run ./scripts/bench -url http://192.168.122.22:3000 -phone 0000000001 -pin 123456
//
// It rings real sales (cash, one or two lines each) into that shop: never point it at a real shop.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"slices"
	"strconv"
	"time"
)

type client struct {
	base string
	http *http.Client
}

func (c *client) do(method, path string, in, out any) (time.Duration, error) {
	var body io.Reader
	if in != nil {
		b, _ := json.Marshal(in)
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.base+"/api/v1"+path, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ks-Device", "bench")
	t0 := time.Now()
	res, err := c.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	took := time.Since(t0)
	if res.StatusCode >= 300 {
		return took, fmt.Errorf("%s %s: %d %.200s", method, path, res.StatusCode, b)
	}
	if out != nil {
		return took, json.Unmarshal(b, out)
	}
	return took, nil
}

// timings is one measure's samples.
type timings []time.Duration

func (t timings) pct(p float64) time.Duration {
	s := slices.Clone(t)
	slices.Sort(s)
	return s[min(len(s)-1, int(float64(len(s))*p))]
}

type product struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	SellingPrice string `json:"sellingPrice"`
	StockQty     string `json:"stockQty"`
	StockFrozen  bool   `json:"stockFrozen"`
}

func main() {
	base := flag.String("url", "http://127.0.0.1:3000", "the shop's address")
	phone := flag.String("phone", "0000000001", "an admin's phone")
	pin := flag.String("pin", "", "that admin's PIN")
	searches := flag.Int("searches", 200, "till searches")
	sales := flag.Int("sales", 100, "sales to ring")
	label := flag.String("label", "", "what this machine is (goes in the result)")
	flag.Parse()
	jar, _ := cookiejar.New(nil)
	c := &client{base: *base, http: &http.Client{Jar: jar, Timeout: 5 * time.Minute}}
	must := func(_ time.Duration, err error) {
		if err != nil {
			fmt.Fprintln(os.Stderr, "bench:", err)
			os.Exit(1)
		}
	}
	must(c.do("POST", "/auth/login", map[string]string{"phone": *phone, "pin": *pin}, nil))
	// The till needs its drawer open to sell; an already open one is fine.
	c.do("POST", "/till/drawer/open", map[string]any{"breakdown": map[string]int{"1000": 5}}, nil) //nolint:errcheck

	// Words a cashier types: the first 3–5 letters of real product names.
	var page struct{ Items []product }
	must(c.do("GET", "/till/products?search=a", nil, &page))
	words := []string{"pan", "vit", "amo", "sug", "mil", "ric", "tea", "soa", "bis", "cab", "bat", "cha"}
	for _, p := range page.Items {
		if len(p.Name) >= 4 {
			words = append(words, p.Name[:3+rand.IntN(2)])
		}
	}
	var search timings
	var sellable []product
	for i := range *searches {
		var pg struct{ Items []product }
		d, err := c.do("GET", "/till/products?search="+url.QueryEscape(words[i%len(words)]), nil, &pg)
		must(d, err)
		search = append(search, d)
		for _, p := range pg.Items {
			if q, _ := strconv.ParseFloat(p.StockQty, 64); (q >= 5 || p.StockFrozen) && p.SellingPrice != "" && p.SellingPrice != "0.00" {
				sellable = append(sellable, p)
			}
		}
	}
	if len(sellable) == 0 {
		fmt.Fprintln(os.Stderr, "bench: no product in stock to sell")
		os.Exit(1)
	}

	var sale timings
	refused := 0
	for range *sales {
		a, b := sellable[rand.IntN(len(sellable))], sellable[rand.IntN(len(sellable))]
		items := []map[string]any{{"productId": a.ID, "quantity": "1"}}
		total, _ := strconv.ParseFloat(a.SellingPrice, 64)
		if b.ID != a.ID && rand.IntN(2) == 0 {
			items = append(items, map[string]any{"productId": b.ID, "quantity": "1"})
			bp, _ := strconv.ParseFloat(b.SellingPrice, 64)
			total += bp
		}
		d, err := c.do("POST", "/till/sales", map[string]any{"saleType": "retail", "discount": "0", "discountType": "fixed",
			"items": items, "payments": []map[string]any{{"method": "cash", "amount": strconv.FormatFloat(total, 'f', 2, 64)}}}, nil)
		if err != nil { // refused (sold out, needs a serial): never pick those again
			refused++
			sellable = slices.DeleteFunc(sellable, func(p product) bool { return p.ID == a.ID || p.ID == b.ID })
			if len(sellable) == 0 {
				break
			}
			continue
		}
		sale = append(sale, d)
	}

	year := "from=" + time.Now().AddDate(-1, 0, 1).Format("2006-01-02") + "&to=" + time.Now().Format("2006-01-02")
	reports := map[string]time.Duration{}
	for name, path := range map[string]string{
		"dashboard":          "/admin/dashboard",
		"sales (year)":       "/admin/reports/sales?" + year,
		"finance (year)":     "/admin/finance?" + year,
		"top products":       "/admin/reports/top-products?" + year,
		"inventory":          "/admin/reports/inventory",
		"profit by category": "/admin/reports/profit-by-category?" + year,
	} {
		d, err := c.do("GET", path, nil, nil)
		must(d, err)
		reports[name] = d
	}
	backup, err := c.do("GET", "/admin/data/backup", nil, nil)
	must(backup, err)

	out := map[string]any{
		"label": *label, "at": time.Now().Format(time.RFC3339),
		"searchP50ms": search.pct(.5).Milliseconds(), "searchP95ms": search.pct(.95).Milliseconds(),
		"saleP50ms": sale.pct(.5).Milliseconds(), "saleP95ms": sale.pct(.95).Milliseconds(), "sales": len(sale), "refused": refused,
		"backupMs": backup.Milliseconds(),
	}
	rep := map[string]int64{}
	for k, v := range reports {
		rep[k] = v.Milliseconds()
	}
	out["reportsMs"] = rep
	json.NewEncoder(os.Stdout).Encode(out) //nolint:errcheck
}
