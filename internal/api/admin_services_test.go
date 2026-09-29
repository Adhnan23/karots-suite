package api

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"testing"
	"time"
)

func TestAdminServicesAndRecipes(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009531", "manager")
	var cat int64
	te.db.Get(&cat, `SELECT category_id FROM products WHERE id=$1`, f.plainID) //nolint:errcheck

	rec := te.do(http.MethodPost, "/api/v1/admin/services", ServiceRequest{Name: fmt.Sprintf("T coffee %d", time.Now().UnixNano()), CategoryID: cat, UnitID: 1, SellingPrice: "150"}, ck)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	id := decode[ProductSaved](t, rec).ID
	t.Cleanup(func() { te.db.Exec(`DELETE FROM products WHERE id=$1`, id) }) //nolint:errcheck
	path := "/api/v1/admin/products/" + strconv.FormatInt(id, 10) + "/recipe"

	// Stocked products can't carry a recipe.
	if rec := te.do(http.MethodGet, "/api/v1/admin/products/"+strconv.FormatInt(f.plainID, 10)+"/recipe", nil, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("recipe on stock: %d", rec.Code)
	}
	// plain costs 50: 0.2 per cup = 10, plus a 5.00 other cost; a yield of 50 cups on another = 1.
	save := RecipeRequest{
		Components: []RecipeComponent{{ProductID: f.plainID, Amount: "0.2", Mode: "per_unit"}, {ProductID: f.lotID, Amount: "50", Mode: "yield"}},
		Costs:      []RecipeCost{{Label: "Gas", Amount: "5"}, {}},
	}
	if rec := te.do(http.MethodPost, path, save, ck); rec.Code != http.StatusNoContent {
		t.Fatalf("save: %d %s", rec.Code, rec.Body)
	}
	r := decode[Recipe](t, te.do(http.MethodGet, path, nil, ck))
	if len(r.Components) != 2 || len(r.Costs) != 1 || !slices.ContainsFunc(r.Components, func(c RecipeComponent) bool { return c.Mode == "yield" && c.Amount == "50" }) {
		t.Fatalf("recipe back: %+v", r)
	}
	rows := decode[[]ServiceRow](t, te.do(http.MethodGet, "/api/v1/admin/services", nil, ck))
	i := slices.IndexFunc(rows, func(s ServiceRow) bool { return s.ID == id })
	if i < 0 || rows[i].Ingredients != 2 || rows[i].TrueCost != "16.00" || rows[i].Margin != "134.00" {
		t.Fatalf("service row: %+v", rows[i])
	}
	if rec := te.do(http.MethodPost, path, RecipeRequest{Components: []RecipeComponent{{ProductID: id, Amount: "1"}}}, ck); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("self-consuming: %d", rec.Code)
	}
}
