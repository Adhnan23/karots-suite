package app

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"karots-suite/internal/config"
	"karots-suite/internal/features/products"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
)

// fixture is a plugin compiled into this test binary only.
type fixture struct{ setups int }

func (*fixture) Name() string                { return "Fixture" }
func (*fixture) Key() string                 { return "fixture" }
func (*fixture) Migrations() (fs.FS, string) { return nil, "" }
func (f *fixture) Setup(r *plugin.Registry) {
	f.setups++
	r.AddProductBadgeProvider(plugin.ProductBadgeProvider{Batch: func(_ context.Context, ids []int64) (map[int64][]string, error) {
		out := map[int64][]string{}
		for _, id := range ids {
			out[id] = []string{"Fixture"}
		}
		return out, nil
	}})
	// A broken provider never breaks the till: its result is skipped.
	r.AddProductBadgeProvider(plugin.ProductBadgeProvider{Batch: func(context.Context, []int64) (map[int64][]string, error) {
		return nil, errors.New("boom")
	}})
	r.AddProductSaleSuggestionProvider(plugin.ProductSaleSuggestionProvider{Batch: func(_ context.Context, ids []int64) (map[int64]plugin.SaleSuggestion, error) {
		return map[int64]plugin.SaleSuggestion{ids[0]: {DiscountType: "percent", DiscountValue: "10", Label: "Fix -10%", Prompt: "Apply?"}}, nil
	}})
	r.AddProductDetailContributor(plugin.ProductDetailContributor{Rows: func(context.Context, int64) ([]plugin.DetailRow, error) {
		return []plugin.DetailRow{{Label: "Fixture", Value: "yes"}}, nil
	}})
}

var fx = &fixture{}

func init() { plugin.Register(fx) }

// startPlugins runs every plugin's Setup once and feeds the product func-vars the
// till reads (badges, suggestions, info rows) from the plugin hooks.
func TestStartPluginsFeedsProductHooks(t *testing.T) {
	startPlugins(echo.New(), nil, &config.Config{}, &Services{})
	if fx.setups != 1 {
		t.Fatalf("Setup ran %d times, want 1", fx.setups)
	}
	rows := []products.Product{{ID: 7}}
	products.Decorate(context.Background(), rows)
	if len(rows[0].Badges) != 1 || rows[0].Badges[0] != "Fixture" {
		t.Errorf("badges = %v, want [Fixture]", rows[0].Badges)
	}
	if rows[0].Suggestion == nil || rows[0].Suggestion.Label != "Fix -10%" {
		t.Errorf("suggestion = %+v", rows[0].Suggestion)
	}
	if products.DetailContributor == nil {
		t.Fatal("detail rows not wired")
	}
	if got := products.DetailContributor(context.Background(), 7); len(got) != 1 || got[0].Value != "yes" {
		t.Errorf("detail rows = %v", got)
	}
}
