package api

import (
	"net/http"
	"testing"
)

func TestAdminFinance(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009618", "admin")
	f := decode[AdminFinance](t, te.do(http.MethodGet, "/api/v1/admin/finance?preset=this-month", nil, ck))
	if f.Grain != "day" || f.Revenue == "" || f.NetProfit == "" || f.Trend == nil || f.Categories == nil || f.ByPayment == nil {
		t.Fatalf("finance %+v", f)
	}
	if len(f.PluginIncome) != 0 {
		t.Fatalf("plugin income in a core build: %+v", f.PluginIncome)
	}
	if y := decode[AdminFinance](t, te.do(http.MethodGet, "/api/v1/admin/finance?preset=this-year", nil, ck)); y.Grain == "day" {
		t.Fatalf("a year by day: %s", y.Grain)
	}
	if rec := te.do(http.MethodGet, "/api/v1/admin/finance?from=bad", nil, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("bad range: %d", rec.Code)
	}
	for _, g := range []struct {
		days int
		want string
	}{{1, "day"}, {31, "day"}, {32, "week"}, {183, "week"}, {184, "month"}} {
		if got := financeGrain(g.days); got != g.want {
			t.Fatalf("financeGrain(%d) = %s", g.days, got)
		}
	}
}
