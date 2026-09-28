# M1a Till: Sell — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A cashier can open the till, sell by scan, search, group menu or frequently-sold grid, handle every karots-pos stock prompt (which price, expired lot, found at till, clearance suggestion), take cash, card, online, on-account and split tenders with change and Keep, hold and resume sales, move cash in and out of the drawer, and close and count the drawer. All of this runs on the JSON `/api/v1/till/*` API in the approved Studio and Night design.

**Why M1 is split.** M1 in the spec is the whole of §6.5. That is two independent subsystems:

- **M1a (this plan):** the sell screen and the drawer.
- **M1b (next plan):** the counter operations that live in their own till tabs:
  - returns and refunds, write-off (the damage page), convert
  - credit collection, advances and refunds
  - the receipts hub with reprints, the browser receipt view and the Z report
  - labels, warranty
  - suppliers and expenses at the counter

M1 is done when both plans are done. M1a leaves the tab row in the till top bar so M1b only adds tabs.

**Architecture:**
- **Backend:** new handler files `internal/api/till_*.go` under one protected group `/api/v1/till`. Handlers bind camelCase DTOs (`dto_till.go`, `dto_sale.go`, `dto_drawer.go`), map them to the forked domain inputs, and return DTOs. Seam-free services (products, groups, customers, stock, held sales, lockers, …) are built from the DB inside `api.Register`. Services with core seams (sales, cash register, the drawer kicker) come in through `api.Deps` from `app.NewServices`.
- **Frontend:** the till is `web/src/features/till/`.
  - **Pure logic, all Vitest-tested:** `cart/` (types, line maths, totals, entry parsing, held blobs, the reducer, the checkout gate) and `hotkeys.ts`.
  - **State:** held in a `useReducer` provider at the till layout, persisted to `sessionStorage` per user, so auto-lock, reload and (in M1b) switching till tabs never lose the cart.
  - **Components:** read server data through TanStack Query (`features/till/api.ts`).
- **Plugins:** the plugin seams are typed slots in `web/src/plugins/slots.ts` (menu roots, till actions, drawer sections, tenders, before-checkout and after-sale hooks). They are empty in M1a and filled in M7.

**Tech Stack:**
- **Go:** Go 1.26, Echo v4, sqlx, Postgres 17, tygo
- **Web:** React 19, TypeScript strict, TanStack Router and Query, Tailwind v4, shadcn/Radix, sonner, lucide
- **New this milestone:** `motion`
- **Tests:** Vitest, Playwright

**Spec:** `docs/specs/2026-09-27-karots-suite-design.md` (§5, §6.5, §7, §8). Design: `docs/design/theme.md` §7 (till) and §8 (motion). Status and way of working: `docs/status.md`.

**Behaviour source (read before each UI task):**
- the karots-pos Alpine mixins `~/Projects/go/karots-pos/static/js/pos*.js`
- the templates `~/Projects/go/karots-pos/templates/pages/cashier/pos*.templ`
- the till bug history in `~/.claude/projects/-home-karots-Projects-go-karots-pos/memory/MEMORY.md` ("TILL UX FIXES")

When this plan and karots-pos disagree about *behaviour*, karots-pos wins. Flag the difference in the task's commit message.

## Global Constraints

- **Commits.** Commit on `main`, local, one commit per task step marked **Commit**. **No Claude/AI attribution** in any commit: no `Co-Authored-By`, no "Generated with".
- **Core is plugin-inert.** Nothing under `internal/` (outside `internal/plugin`) or `web/src` (outside `web/src/plugins`) may name a plugin (recharge, wallet, reload, documents, …). Plugin behaviour enters only through `plugin.*` hooks (Go) and `slots` (React).
- **Money path.**
  - Money moves only through the forked services (`sales.Create`, `cashregister.*`, `cashflow`).
  - The client never sends a price for a stocked product. It sends `batchId`, and the server reads the price.
  - Client maths is a preview only.
- **API shape:**
  - JSON under `/api/v1`
  - camelCase fields
  - money and quantities as strings
  - errors `{"error":{"code","message"}}`
  - lists as plain arrays unless paged (`{items,total}`)
- **Types.** Run `make types` after any `dto*.go` change. Never hand-edit `web/src/api/types.gen.ts`. `tygo.yaml` lists every DTO file.
- **File size.** Every source file stays at 500 lines or fewer (Go and TS).
- **Theme.**
  - Components use theme tokens only (`bg-card`, `bg-brand`, `text-muted-foreground`, `text-success`, `text-danger`, `text-warning`, `bg-brand-soft`, `bg-selected`, `var(--shadow)`, `var(--glass)`, …).
  - No raw colour classes (`bg-slate-*`, `text-emerald-*`, hex).
  - Never branch on the palette.
  - `--glass` only on the three big panels, never on tiles or cart lines.
  - `tabular-nums` wherever money shows.
  - The till total and the change figure use `font-display`.
- **Till layout** (theme.md §7):
  - 60px top bar
  - three columns `1fr 392px 356px` at 1200px and wider
  - two columns `1fr 360px` below that, with cart and pay stacked
  - kiosk autofit below 1024px (already in `lib/autofit.ts`)
- **Motion** (theme.md §8): use `motion/react`, durations of 350ms or less, and `MotionConfig reducedMotion="user"` around the till.
- **Money cue.** Green (`text-success`) with ↓ means money in. Red (`text-danger`) with ↑ means money out.
- **Till semantics:**
  - F1 / "⟲ Menu" clears the search and the group drill-down **only**. Only New Sale (or a completed sale) clears the cart.
  - With a customer picked, the tender method starts **unpicked**, so there is never a silent cash default.
- **UI gates.** Tasks 9, 11, 12 and 13 end in a gate. Stop and show screenshots of all four palette × mode combinations (`SHOTS=<gate> bunx playwright test e2e/till-shots.spec.ts`, Task 8) at 1366×768 and 1024×768, then wait for approval. Plain or default-looking UI is rejected. Match the "Night Counter" and "Studio" till in https://claude.ai/artifact/GkJjnypEes4GmHiHMDzFT5.
- **TanStack skills.** Before Task 8, from `web/`, run `intent list`, then `intent load` the router and query skills.

## Review Focus

These are the conditions most likely to hurt a real cashier that no single happy-path test covers. Each one is pinned by the named test.

1. **Auto-lock, a reload or a crash mid-sale** must bring the same cart back for the same user, and never show it to a different user. Pinned by Task 6 (`persist.test.ts`) and Task 14 (e2e "reload keeps the cart").
2. **Rapid scanning.** A scanner types a whole barcode and presses Enter faster than the 300ms search debounce, again and again. Every scan must add exactly one unit, and the next scan's digits must never be wiped by the previous lookup finishing. Pinned by Task 6 (`entry.test.ts`) and Task 14 (e2e "three fast scans = qty 3").
3. **A customer picked and the tender left blank** must never complete as paid in cash. It must raise the "put it on account?" prompt. Pinned by Task 7 (`checkout.test.ts`) and Task 14 (e2e credit flow).
4. **A double Enter or double click on Complete** must post exactly one sale. Pinned by Task 13 (`useCheckout.test.tsx`).
5. **A held sale resumes intact**, including a percent bill discount, lot lines and serials. A held cart in karots-pos's array shape (from a restored backup) must also resume. Pinned by Task 6 (`held.test.ts`).

---

## File Structure (M1a)

```
internal/api/
  dto_till.go            config, catalog, customers DTOs
  dto_sale.go            sale request/result, held sale DTOs
  dto_drawer.go          drawer DTOs
  till.go                registerTill, tillServices, shared helpers (dec2, pathID, audit)
  till_catalog.go        config, groups, products, lots, price options, quick picks, units
  till_customers.go      customers, credit limit, quick item, barcode, expired write-off
  till_drawer.go         drawer state, open/close/withdraw/deposit/no-sale
  till_sales.go          create sale, held sales
  till_print.go          receipt queue, sale receipt print, money slip print + builder
  till_fixtures_test.go  seedTill, login, decode helpers
  till_catalog_test.go, till_customers_test.go, till_drawer_test.go, till_sales_test.go
internal/features/cashregister/breakdown.go   amountFrom/breakdownTotal moved out of api.go
internal/features/products/hooks.go           + Decorate()
DELETED: features/productgroups/api.go, features/cashregister/api.go, heldsales handler section
scripts/e2e-till.sql
web/src/lib/money.ts (+ .test.ts)
web/src/plugins/slots.ts
web/src/features/till/
  api.ts  print.ts  hotkeys.ts (+test)  TillScreen.tsx  TillUi.tsx  TillProvider.tsx  ui.ts
  cart/   types.ts lines.ts totals.ts entry.ts held.ts store.ts persist.ts checkout.ts (+ tests)
  drawer/ count.ts (+test) DenomCounter.tsx LockerSelect.tsx OpenRegister.tsx CloseRegister.tsx
          MoneyMove.tsx DrawerPill.tsx DrawerActions.tsx
  catalog/ CatalogPanel.tsx ProductTile.tsx GroupCard.tsx ProductInfo.tsx QuickItem.tsx useEntry.ts
  cart-ui/ CartPanel.tsx CartLineRow.tsx PricePick.tsx HeldSales.tsx PostSaleChange.tsx
  pay/    PayPanel.tsx CustomerPicker.tsx AddCustomer.tsx Tender.tsx Summary.tsx ChangeBanner.tsx
  checkout/ CheckoutProvider.tsx useCheckout.ts (+test) Prompts.tsx SaleComplete.tsx Hotkeys.tsx
web/src/routes/_authed/till.tsx, till/index.tsx   (modified)
web/src/theme/TillShell.tsx, AdminShell.tsx        (modified)
web/e2e/till.spec.ts, web/e2e/till-shots.spec.ts
```

---

### Task 1: Till read API — config and catalog

**Files:**
- Create: `internal/api/dto_till.go`, `internal/api/till.go`, `internal/api/till_catalog.go`, `internal/api/till_fixtures_test.go`, `internal/api/till_catalog_test.go`
- Modify:
  - `internal/api/api.go` (handlers get `t tillServices`; call `h.registerTill(v1)`)
  - `internal/features/products/hooks.go` (add `Decorate`)
  - `tygo.yaml`
- Delete: `internal/features/productgroups/api.go`

**Interfaces:**
- Produces:
  - `GET /api/v1/till/config` → `TillConfig`
  - `GET /till/groups` → `TillGroup[]`
  - `GET /till/groups/:id` → `GroupView`
  - `GET /till/products?search=` → `ProductPage`
  - `GET /till/products/barcode/:code` → `TillProduct` (404 `NOT_FOUND`)
  - `GET /till/products/:id` → `TillProduct`
  - `GET /till/products/:id/detail` → `ProductDetail`
  - `GET /till/products/:id/lots` → `Lot[]`
  - `GET /till/price-options` → `{[productId]: Lot[]}`
  - `GET /till/quick-picks` → `TillProduct[]`
  - `GET /till/units` → `Unit[]`
  - Go helpers used by later tasks: `dec2(decimal.Decimal) string`, `strOr(*string) string`, `pathID(echo.Context) (int64, error)`, `tillProduct(products.Product) TillProduct`, `(h *handlers) audit(c, action, entity, id, detail)`, the struct `tillServices`
  - Test helpers: `(te *testEnv) seedTill(t) tillFix`, `(te *testEnv) login(t, phone, role) *http.Cookie`, `decode[T](t, rec) T`

- [ ] **Step 1: DTOs.** Create `internal/api/dto_till.go`:

```go
package api

// ---- Till: config ----

type TenderMethod struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// TillConfig is everything the till needs from settings + the user's flags,
// loaded once when the till opens.
type TillConfig struct {
	Symbol             string         `json:"symbol"`
	DefaultSaleType    string         `json:"defaultSaleType"` // "retail" | "wholesale"
	AskToPrint         bool           `json:"askToPrint"`
	OpenCashDrawer     bool           `json:"openCashDrawer"`
	AllowUntrackedCash bool           `json:"allowUntrackedCash"`
	DefaultLockerID    int64          `json:"defaultLockerId"` // 0 = none
	AllowContinueLast  bool           `json:"allowContinueLast"`
	CanManageCredit    bool           `json:"canManageCredit"`
	CanSeeCost         bool           `json:"canSeeCost"`
	CanChangePin       bool           `json:"canChangePin"`
	IsManager          bool           `json:"isManager"`     // admin/manager: shows the Admin link
	TenderMethods      []TenderMethod `json:"tenderMethods"` // plugin tenders; core ones are fixed in the SPA
}

// ---- Till: catalog ----

type SaleSuggestion struct {
	DiscountType  string `json:"discountType"` // "fixed" | "percent"
	DiscountValue string `json:"discountValue"`
	Label         string `json:"label"`
	Prompt        string `json:"prompt"`
}

// TillProduct is one sellable card. Money and quantities are decimal strings.
type TillProduct struct {
	ID               int64           `json:"id"`
	Name             string          `json:"name"`
	Barcode          string          `json:"barcode"`
	SellingPrice     string          `json:"sellingPrice"`
	WholesalePrice   string          `json:"wholesalePrice"`
	TaxRate          string          `json:"taxRate"`
	StockQty         string          `json:"stockQty"`
	UnitAbbr         string          `json:"unitAbbr"`
	UnitAllowDecimal bool            `json:"unitAllowDecimal"`
	IsService        bool            `json:"isService"`
	TrackSerial      bool            `json:"trackSerial"`
	WarrantyMonths   int             `json:"warrantyMonths"`
	Emoji            string          `json:"emoji"`
	Badges           []string        `json:"badges"`
	Suggestion       *SaleSuggestion `json:"suggestion,omitempty"`
}

type ProductPage struct {
	Items []TillProduct `json:"items"`
	Total int           `json:"total"`
}

type TillGroup struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Emoji       string `json:"emoji"`
	HasChildren bool   `json:"hasChildren"`
	ItemCount   int    `json:"itemCount"`
}

type GroupView struct {
	Group      TillGroup     `json:"group"`
	Breadcrumb []TillGroup   `json:"breadcrumb"`
	Children   []TillGroup   `json:"children"`
	Products   []TillProduct `json:"products"`
}

// Lot is one live stock batch. ExpiryDate/ReceivedAt are YYYY-MM-DD ("" = none).
type Lot struct {
	BatchID      int64  `json:"batchId"`
	BatchNo      string `json:"batchNo"`
	ExpiryDate   string `json:"expiryDate"`
	QtyRemaining string `json:"qtyRemaining"`
	Price        string `json:"price"`
	ReceivedAt   string `json:"receivedAt"`
}

type DetailRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// ProductDetail feeds the ⓘ popup. Cost fields are empty unless ShowCost.
type ProductDetail struct {
	ID           int64       `json:"id"`
	Name         string      `json:"name"`
	Category     string      `json:"category"`
	Barcode      string      `json:"barcode"`
	Unit         string      `json:"unit"`
	SellingPrice string      `json:"sellingPrice"`
	StockQty     string      `json:"stockQty"`
	Warranty     string      `json:"warranty"`
	ShowCost     bool        `json:"showCost"`
	CostPrice    string      `json:"costPrice"`
	MarginPct    string      `json:"marginPct"`
	Rows         []DetailRow `json:"rows"`
}

type Unit struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Abbreviation string `json:"abbreviation"`
	AllowDecimal bool   `json:"allowDecimal"`
}
```

Edit `tygo.yaml` so `include_files` lists every DTO file. Tasks 3 and 4 add their files the same way:

```yaml
    include_files: ["dto.go", "dto_till.go"]
```

- [ ] **Step 2: Shared till plumbing.** Create `internal/api/till.go`:

```go
package api

import (
	"strconv"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/productgroups"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/units"
	"karots-suite/internal/middleware"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

// tillServices are the seam-free domain services the till calls, built once from
// the DB. Services that carry core seams (sales, cash register, drawer kick) come
// in through Deps instead, wired in app.NewServices.
type tillServices struct {
	products *products.Service
	groups   *productgroups.Service
	units    *units.Service
	audit    *audit.Service
}

func newTillServices(db *sqlx.DB) tillServices {
	return tillServices{
		products: products.NewService(db),
		groups:   productgroups.NewService(db),
		units:    units.NewService(db),
		audit:    audit.NewService(db),
	}
}

// registerTill mounts the cashier API. Every route needs a live, unlocked,
// PIN-chosen session (h.protected); per-route guards add role/flag checks.
func (h *handlers) registerTill(v1 *echo.Group) {
	g := v1.Group("/till", h.protected...)
	h.registerTillCatalog(g)
}

func dec2(d decimal.Decimal) string { return d.StringFixed(2) }

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func pathID(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.BadRequest("invalid id")
	}
	return id, nil
}

// audit records a till action for the owner's trail. The hidden system account
// is never audited (spec §6.6).
func (h *handlers) audit(c echo.Context, action, entity, id, detail string) {
	ctx := c.Request().Context()
	if middleware.IsSystemUserCtx(ctx) {
		return
	}
	h.t.audit.Record(ctx, middleware.CurrentUserID(c), action, entity, id, detail)
}
```

In `internal/api/api.go`:
- add `t tillServices` to `handlers`
- build it in `Register`: `h := &handlers{d: d, protected: ..., t: newTillServices(d.DB)}`
- add `h.registerTill(v1)` after `h.registerSystem(v1)`

- [ ] **Step 3: `products.Decorate`.** Append to `internal/features/products/hooks.go`:

```go
// Decorate fills the plugin-contributed, non-persisted fields (till badges and
// sale suggestions) on rows, exactly as the legacy List/Get handlers did. The
// JSON till layer calls it for every product list it returns.
func Decorate(ctx context.Context, rows []Product) {
	if BadgeProvider != nil && len(rows) > 0 {
		ids := make([]int64, len(rows))
		for i := range rows {
			ids[i] = rows[i].ID
		}
		if m := BadgeProvider(ctx, ids); m != nil {
			for i := range rows {
				rows[i].Badges = m[rows[i].ID]
			}
		}
	}
	applySuggestions(ctx, rows)
}
```

- [ ] **Step 4: Test fixtures.** Create `internal/api/till_fixtures_test.go`:

```go
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type tillFix struct {
	plainID, lotID, expiringID, scarceID, groupID int64
	plainBarcode                                  string
}

// seedTill makes a category, four products with stock + lots, and a group, all
// with fresh names/barcodes. Rows are left behind on purpose: sales reference
// them and the dev DB is disposable (make test-fresh).
//   plain     Rs 100, 10 in one lot
//   lot       Rs 100, lots of 5 @ product price and 5 @ 120 (price disagreement)
//   expiring  Rs 80, 3 expired yesterday (FEFO-first) + 3 fresh
//   scarce    Rs 250, 1 on hand
func (te *testEnv) seedTill(t *testing.T) tillFix {
	t.Helper()
	ctx := context.Background()
	sfx := time.Now().UnixNano()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	var cat int64
	must(te.db.GetContext(ctx, &cat, `INSERT INTO categories (name) VALUES ($1) RETURNING id`, fmt.Sprintf("T cat %d", sfx)))
	product := func(name, barcode, price, qty string) int64 {
		var id int64
		must(te.db.GetContext(ctx, &id, `INSERT INTO products (name, barcode, category_id, unit_id, cost_price, selling_price)
			VALUES ($1, NULLIF($2, ''), $3, 1, 50, $4) RETURNING id`, fmt.Sprintf("%s %d", name, sfx), barcode, cat, price))
		_, err := te.db.ExecContext(ctx, `INSERT INTO stock (product_id, quantity) VALUES ($1, $2)
			ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity`, id, qty)
		must(err)
		return id
	}
	lot := func(pid int64, qty string, price *string, expiry *time.Time) {
		_, err := te.db.ExecContext(ctx, `INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, selling_price, expiry_date)
			VALUES ($1, $2, $2, 50, $3, $4)`, pid, qty, price, expiry)
		must(err)
	}
	var f tillFix
	f.plainBarcode = fmt.Sprintf("88%011d", sfx%100_000_000_000)
	f.plainID = product("T plain", f.plainBarcode, "100", "10")
	lot(f.plainID, "10", nil, nil)
	f.lotID = product("T lot", "", "100", "10")
	p120 := "120"
	lot(f.lotID, "5", nil, nil)
	lot(f.lotID, "5", &p120, nil)
	yesterday, later := time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 6, 0)
	f.expiringID = product("T expiring", "", "80", "6")
	lot(f.expiringID, "3", nil, &yesterday)
	lot(f.expiringID, "3", nil, &later)
	f.scarceID = product("T scarce", "", "250", "1")
	lot(f.scarceID, "1", nil, nil)
	must(te.db.GetContext(ctx, &f.groupID, `INSERT INTO product_groups (name) VALUES ($1) RETURNING id`, fmt.Sprintf("T group %d", sfx)))
	_, err := te.db.ExecContext(ctx, `INSERT INTO product_group_items (group_id, product_id) VALUES ($1, $2)`, f.groupID, f.plainID)
	must(err)
	return f
}

// login seeds a user (PIN 1234) and returns their session cookie.
func (te *testEnv) login(t *testing.T, phone, role string) *http.Cookie {
	t.Helper()
	te.seedUser(t, phone, role)
	return sessionCookie(t, te.do(http.MethodPost, "/api/v1/auth/login", LoginRequest{Phone: phone, Pin: "1234"}, nil))
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %d %s: %v", rec.Code, rec.Body, err)
	}
	return v
}
```

- [ ] **Step 5: Write the failing catalog test.** Create `internal/api/till_catalog_test.go`:

```go
package api

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
)

func TestTillNeedsSession(t *testing.T) {
	te := newTestEnv(t)
	if rec := te.do(http.MethodGet, "/api/v1/till/config", nil, nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no cookie: %d", rec.Code)
	}
}

func TestTillCatalog(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009101", "cashier")

	cfg := decode[TillConfig](t, te.do(http.MethodGet, "/api/v1/till/config", nil, ck))
	if cfg.Symbol == "" || cfg.TenderMethods == nil || cfg.IsManager || cfg.CanSeeCost {
		t.Fatalf("config %+v", cfg)
	}

	rec := te.do(http.MethodGet, "/api/v1/till/products/barcode/"+f.plainBarcode, nil, ck)
	if p := decode[TillProduct](t, rec); rec.Code != 200 || p.ID != f.plainID || p.SellingPrice != "100.00" || p.Badges == nil {
		t.Fatalf("barcode %d %+v", rec.Code, p)
	}
	if rec := te.do(http.MethodGet, "/api/v1/till/products/barcode/000000000", nil, ck); rec.Code != 404 {
		t.Fatalf("unknown barcode %d", rec.Code)
	}

	page := decode[ProductPage](t, te.do(http.MethodGet, "/api/v1/till/products?search="+f.plainBarcode, nil, ck))
	if len(page.Items) != 1 || page.Items[0].ID != f.plainID {
		t.Fatalf("search %+v", page)
	}

	top := decode[[]TillGroup](t, te.do(http.MethodGet, "/api/v1/till/groups", nil, ck))
	found := false
	for _, g := range top {
		found = found || g.ID == f.groupID
	}
	gv := decode[GroupView](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/groups/%d", f.groupID), nil, ck))
	if !found || len(gv.Products) != 1 || gv.Products[0].ID != f.plainID || gv.Breadcrumb == nil || gv.Children == nil {
		t.Fatalf("groups top-found=%v view=%+v", found, gv)
	}

	opts := decode[map[string][]Lot](t, te.do(http.MethodGet, "/api/v1/till/price-options", nil, ck))
	lots := opts[strconv.FormatInt(f.lotID, 10)]
	if len(lots) != 2 || lots[0].Price == lots[1].Price {
		t.Fatalf("lot product options %+v", lots)
	}
	if _, ok := opts[strconv.FormatInt(f.expiringID, 10)]; !ok {
		t.Fatal("expired-first product must be offered (expiry prompt)")
	}
	if _, ok := opts[strconv.FormatInt(f.plainID, 10)]; ok {
		t.Fatal("single-price, unexpired product must not prompt")
	}
	own := decode[[]Lot](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/lots", f.expiringID), nil, ck))
	if len(own) != 2 || own[0].ExpiryDate == "" {
		t.Fatalf("lots %+v", own)
	}

	d := decode[ProductDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/detail", f.plainID), nil, ck))
	if d.ShowCost || d.CostPrice != "" {
		t.Fatalf("cashier without flag must not see cost: %+v", d)
	}
	admin := te.login(t, "0770009102", "admin")
	d = decode[ProductDetail](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/detail", f.plainID), nil, admin))
	if !d.ShowCost || d.CostPrice != "50.00" {
		t.Fatalf("admin sees cost: %+v", d)
	}
	if units := decode[[]Unit](t, te.do(http.MethodGet, "/api/v1/till/units", nil, ck)); len(units) == 0 {
		t.Fatal("units empty")
	}
	if picks := te.do(http.MethodGet, "/api/v1/till/quick-picks", nil, ck); picks.Code != 200 {
		t.Fatalf("quick picks %d", picks.Code)
	}
}
```

Run: `make test-fresh` once, then `set -a; . ./.env; set +a; go test ./internal/api -run 'TestTill' -v`
Expected: FAIL. The handlers don't exist yet, so the build fails with `h.registerTillCatalog undefined`.

- [ ] **Step 6: Implement the catalog handlers.** Create `internal/api/till_catalog.go`:

```go
package api

import (
	"net/http"
	"strconv"

	"karots-suite/internal/features/auth"
	"karots-suite/internal/features/productgroups"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"
	"karots-suite/internal/plugin"

	"github.com/labstack/echo/v4"
	"github.com/shopspring/decimal"
)

func (h *handlers) registerTillCatalog(g *echo.Group) {
	g.GET("/config", h.TillConfig)
	g.GET("/groups", h.TillGroups)
	g.GET("/groups/:id", h.TillGroup)
	g.GET("/products", h.TillProducts)
	g.GET("/products/barcode/:code", h.TillProductByBarcode)
	g.GET("/products/:id", h.TillProduct)
	g.GET("/products/:id/detail", h.TillProductDetail)
	g.GET("/products/:id/lots", h.TillLots)
	g.GET("/price-options", h.TillPriceOptions)
	g.GET("/quick-picks", h.TillQuickPicks)
	g.GET("/units", h.TillUnits)
}

func (h *handlers) TillConfig(c echo.Context) error {
	ctx := c.Request().Context()
	st, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	role := middleware.CurrentRole(c)
	out := TillConfig{
		Symbol:             st.CurrencySymbol,
		DefaultSaleType:    "retail",
		AskToPrint:         st.AskToPrint,
		OpenCashDrawer:     st.OpenCashDrawer,
		AllowUntrackedCash: st.AllowUntrackedCash,
		AllowContinueLast:  st.AllowContinueLastCount,
		CanManageCredit:    middleware.MayManageCredit(role, middleware.CanManageCredit(c)),
		CanSeeCost:         middleware.MaySeeCost(role, middleware.CanSeeCost(c)),
		CanChangePin:       role != auth.RoleCashier || h.d.Auth.AllowCashierPinChange(ctx),
		IsManager:          role == auth.RoleAdmin || role == auth.RoleManager,
		TenderMethods:      []TenderMethod{},
	}
	// A DB predating credit-as-a-payment may still hold 'credit' here; fall back
	// to retail so the till stays usable (karots-pos cashier.POS did the same).
	if st.DefaultSaleType == "wholesale" {
		out.DefaultSaleType = "wholesale"
	}
	if st.DefaultLockerID != nil {
		out.DefaultLockerID = *st.DefaultLockerID
	}
	for _, t := range plugin.TenderMethods() {
		out.TenderMethods = append(out.TenderMethods, TenderMethod{Value: t.Value, Label: t.Label})
	}
	return c.JSON(http.StatusOK, out)
}

func tillProduct(p products.Product) TillProduct {
	out := TillProduct{
		ID: p.ID, Name: p.Name, Barcode: strOr(p.Barcode),
		SellingPrice: dec2(p.SellingPrice), WholesalePrice: dec2(p.WholesalePrice),
		TaxRate: p.TaxRate.String(), StockQty: p.StockQty.String(),
		UnitAbbr: p.UnitAbbr, UnitAllowDecimal: p.UnitAllowDecimal,
		IsService: p.IsService, TrackSerial: p.TrackSerial, WarrantyMonths: p.WarrantyMonths,
		Badges: p.Badges,
	}
	if out.Badges == nil {
		out.Badges = []string{}
	}
	if s := p.Suggestion; s != nil {
		out.Suggestion = &SaleSuggestion{DiscountType: s.DiscountType, DiscountValue: s.DiscountValue, Label: s.Label, Prompt: s.Prompt}
	}
	return out
}

func tillProducts(rows []products.Product) []TillProduct {
	out := make([]TillProduct, 0, len(rows))
	for _, p := range rows {
		out = append(out, tillProduct(p))
	}
	return out
}

func groupProduct(p productgroups.GroupProduct) TillProduct {
	return TillProduct{
		ID: p.ProductID, Name: p.Name, Barcode: strOr(p.Barcode),
		SellingPrice: dec2(p.SellingPrice), WholesalePrice: dec2(p.WholesalePrice),
		TaxRate: p.TaxRate.String(), StockQty: p.StockQty.String(),
		UnitAbbr: p.UnitAbbr, UnitAllowDecimal: p.UnitAllowDecimal,
		IsService: p.IsService, TrackSerial: p.TrackSerial, WarrantyMonths: p.WarrantyMonths,
		Emoji: strOr(p.Emoji), Badges: []string{},
	}
}

func tillGroups(rows []productgroups.Group) []TillGroup {
	out := make([]TillGroup, 0, len(rows))
	for _, g := range rows {
		out = append(out, TillGroup{ID: g.ID, Name: g.Name, Emoji: strOr(g.Emoji), HasChildren: g.HasChildren, ItemCount: g.ItemCount})
	}
	return out
}

func lotOf(o stock.PriceOption) Lot {
	l := Lot{BatchID: o.BatchID, BatchNo: strOr(o.BatchNo), QtyRemaining: o.QtyRemaining.String(),
		Price: dec2(o.Price), ReceivedAt: o.ReceivedAt.Format("2006-01-02")}
	if o.ExpiryDate != nil {
		l.ExpiryDate = o.ExpiryDate.Format("2006-01-02")
	}
	return l
}

func lotsOf(rows []stock.PriceOption) []Lot {
	out := make([]Lot, 0, len(rows))
	for _, o := range rows {
		out = append(out, lotOf(o))
	}
	return out
}

func (h *handlers) TillGroups(c echo.Context) error {
	rows, err := h.t.groups.Children(c.Request().Context(), nil)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, tillGroups(rows))
}

func (h *handlers) TillGroup(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	g, err := h.t.groups.Get(ctx, id)
	if err != nil {
		return err
	}
	children, err := h.t.groups.Children(ctx, &id)
	if err != nil {
		return err
	}
	prods, err := h.t.groups.Products(ctx, id)
	if err != nil {
		return err
	}
	crumb, err := h.t.groups.Breadcrumb(ctx, id)
	if err != nil {
		return err
	}
	out := GroupView{Group: tillGroups([]productgroups.Group{*g})[0], Breadcrumb: tillGroups(crumb),
		Children: tillGroups(children), Products: make([]TillProduct, 0, len(prods))}
	for _, p := range prods {
		out.Products = append(out.Products, groupProduct(p))
	}
	return c.JSON(http.StatusOK, out)
}

// TillProducts is the typed search behind the scan/search box (name, barcode,
// fuzzy rescue — all in products.List). Same 100-row cap as karots-pos.
func (h *handlers) TillProducts(c echo.Context) error {
	ctx := c.Request().Context()
	q := products.ListQuery{Page: 1, Limit: 100, Search: c.QueryParam("search")}
	q.Normalize()
	rows, total, err := h.t.products.List(ctx, q)
	if err != nil {
		return err
	}
	products.Decorate(ctx, rows)
	return c.JSON(http.StatusOK, ProductPage{Items: tillProducts(rows), Total: total})
}

func (h *handlers) TillProductByBarcode(c echo.Context) error {
	ctx := c.Request().Context()
	p, err := h.t.products.GetByBarcode(ctx, c.Param("code"))
	if err != nil {
		return err
	}
	one := []products.Product{*p}
	products.Decorate(ctx, one)
	return c.JSON(http.StatusOK, tillProduct(one[0]))
}

func (h *handlers) TillProduct(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	p, err := h.t.products.Get(ctx, id)
	if err != nil {
		return err
	}
	one := []products.Product{*p}
	products.Decorate(ctx, one)
	return c.JSON(http.StatusOK, tillProduct(one[0]))
}

// TillProductDetail is the ⓘ popup. Cost/margin only for users allowed to see
// cost (role-inclusive, like every other cost gate).
func (h *handlers) TillProductDetail(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	p, err := h.t.products.Get(ctx, id)
	if err != nil {
		return err
	}
	d := ProductDetail{ID: p.ID, Name: p.Name, Category: p.CategoryName, Barcode: strOr(p.Barcode), Unit: p.UnitAbbr,
		SellingPrice: dec2(p.SellingPrice), StockQty: p.StockQty.String(), Rows: []DetailRow{}}
	if p.WarrantyMonths > 0 {
		d.Warranty = strconv.Itoa(p.WarrantyMonths) + " month warranty"
	}
	if middleware.MaySeeCost(middleware.CurrentRole(c), middleware.CanSeeCost(c)) {
		d.ShowCost = true
		d.CostPrice = dec2(p.CostPrice)
		if p.SellingPrice.IsPositive() && p.CostPrice.IsPositive() {
			d.MarginPct = p.SellingPrice.Sub(p.CostPrice).Div(p.SellingPrice).Mul(decimal.NewFromInt(100)).StringFixed(1)
		}
	}
	if products.DetailContributor != nil {
		for _, r := range products.DetailContributor(ctx, p.ID) {
			d.Rows = append(d.Rows, DetailRow{Label: r.Label, Value: r.Value})
		}
	}
	return c.JSON(http.StatusOK, d)
}

func (h *handlers) TillLots(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	rows, err := h.t.products.LotsFor(c.Request().Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, lotsOf(rows))
}

// TillPriceOptions lists the FEW products whose live lots disagree on price or
// whose FEFO-first lot is expired — the only ones the till asks "which price?".
func (h *handlers) TillPriceOptions(c echo.Context) error {
	m, err := h.t.products.PriceOptions(c.Request().Context())
	if err != nil {
		return err
	}
	out := make(map[string][]Lot, len(m))
	for pid, rows := range m {
		out[strconv.FormatInt(pid, 10)] = lotsOf(rows)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) TillQuickPicks(c echo.Context) error {
	ctx := c.Request().Context()
	rows, err := h.t.products.QuickPicks(ctx)
	if err != nil {
		return err
	}
	products.Decorate(ctx, rows)
	return c.JSON(http.StatusOK, tillProducts(rows))
}

func (h *handlers) TillUnits(c echo.Context) error {
	rows, err := h.t.units.List(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]Unit, 0, len(rows))
	for _, u := range rows {
		out = append(out, Unit{ID: u.ID, Name: u.Name, Abbreviation: u.Abbreviation, AllowDecimal: u.AllowDecimal})
	}
	return c.JSON(http.StatusOK, out)
}
```

Notes for the implementer:
- `products.Service.PriceOptions` returns `map[int64][]stock.PriceOption`. Check it with `go doc`.
- If `QuickPicks` already calls `applySuggestions` inside the service, `Decorate` running it again is harmless (it is idempotent). Leave it as is.
- If `h.d.Auth.AllowCashierPinChange` has a different name in the fork, use `grep -n 'func (s \*Service) Allow' internal/features/auth/*.go` to find it. Don't add a new one.

- [ ] **Step 7: Delete the legacy group routes.** `git rm internal/features/productgroups/api.go`. Nothing mounts it (M0 unmounted every `RegisterAPI`). Check with `grep -rn 'productgroups.RegisterAPI\|productgroups.NewAPIHandler' --include='*.go' .`. Expected: no output.

- [ ] **Step 8: Run the tests.**

Run: `make types && go vet ./... && set -a; . ./.env; set +a; go test ./internal/api -run 'TestTill' -v`
Expected: PASS. `web/src/api/types.gen.ts` now contains `TillProduct`, `Lot` and the other DTOs.

- [ ] **Step 9: Commit.**

```bash
git add internal/api tygo.yaml web/src/api/types.gen.ts internal/features/products/hooks.go internal/features/productgroups
git commit -m "feat(api): till config + catalog endpoints (groups, search, barcode, lots, price options, quick picks, units)"
```

---

### Task 2: Till customers, quick item and expired-lot write-off

**Files:**
- Modify: `internal/api/dto_till.go`, `internal/api/till.go` (add services + register call)
- Create: `internal/api/till_customers.go`, `internal/api/till_customers_test.go`

**Interfaces:**
- Consumes (Task 1): `tillServices`, `pathID`, `dec2`, `strOr`, `tillProduct`, `h.audit`, `seedTill`, `login`, `decode`.
- Produces:
  - `GET /till/customers` → `TillCustomer[]`
  - `POST /till/customers` (`CustomerCreateRequest`) → 201 `CustomerCreated`, or 200 with `existed: true`
  - `PATCH /till/customers/:id/credit-limit` (`CreditLimitRequest`) → 204. This needs manage-credit (403 otherwise).
  - `GET /till/barcode/generate` → `{barcode}`
  - `POST /till/quick-item` (`QuickItemRequest`) → `TillProduct`
  - `POST /till/write-off` (`WriteOffRequest`) → 204

- [ ] **Step 1: DTOs.** Append to `internal/api/dto_till.go`:

```go
// ---- Till: customers ----

// TillCustomer: Outstanding > 0 means they owe the shop; < 0 is an advance the
// shop holds for them.
type TillCustomer struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	CreditLimit string `json:"creditLimit"`
	Outstanding string `json:"outstanding"`
}

type CustomerCreateRequest struct {
	Name        string `json:"name"`
	Phone       string `json:"phone"`
	CreditLimit string `json:"creditLimit"`
}

type CustomerCreated struct {
	Customer TillCustomer `json:"customer"`
	Existed  bool         `json:"existed"` // phone matched an existing customer; that one is returned
}

type CreditLimitRequest struct {
	CreditLimit string `json:"creditLimit"`
}

// ---- Till: quick item / write-off ----

type QuickItemRequest struct {
	Name    string `json:"name"`
	Price   string `json:"price"`
	Qty     string `json:"qty"`
	Barcode string `json:"barcode"`
	UnitID  int64  `json:"unitId"`
}

type GeneratedBarcode struct {
	Barcode string `json:"barcode"`
}

// WriteOffRequest removes a lot from stock as damage at the till (the expired-lot
// prompt's "Write off" button).
type WriteOffRequest struct {
	ProductID int64  `json:"productId"`
	BatchID   int64  `json:"batchId"`
	Quantity  string `json:"quantity"`
	Note      string `json:"note"`
}
```

- [ ] **Step 2: Write the failing test.** Create `internal/api/till_customers_test.go`:

```go
package api

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"
)

func TestTillCustomers(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009201", "cashier")
	phone := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)

	rec := te.do(http.MethodPost, "/api/v1/till/customers", CustomerCreateRequest{Name: "Till Cust", Phone: phone, CreditLimit: "1000"}, ck)
	created := decode[CustomerCreated](t, rec)
	if rec.Code != 201 || created.Existed || created.Customer.CreditLimit != "1000.00" {
		t.Fatalf("create %d %+v", rec.Code, created)
	}
	rec = te.do(http.MethodPost, "/api/v1/till/customers", CustomerCreateRequest{Name: "Dup", Phone: phone}, ck)
	if again := decode[CustomerCreated](t, rec); rec.Code != 200 || !again.Existed || again.Customer.ID != created.Customer.ID {
		t.Fatalf("dedupe by phone %d %+v", rec.Code, again)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/customers", CustomerCreateRequest{Name: "No phone"}, ck); rec.Code != 400 && rec.Code != 422 {
		t.Fatalf("phone required, got %d", rec.Code)
	}

	path := fmt.Sprintf("/api/v1/till/customers/%d/credit-limit", created.Customer.ID)
	if rec := te.do(http.MethodPatch, path, CreditLimitRequest{CreditLimit: "5000"}, ck); rec.Code != 403 {
		t.Fatalf("cashier without flag must not raise limits: %d", rec.Code)
	}
	admin := te.login(t, "0770009202", "admin")
	if rec := te.do(http.MethodPatch, path, CreditLimitRequest{CreditLimit: "5000"}, admin); rec.Code != 204 {
		t.Fatalf("admin limit %d %s", rec.Code, rec.Body)
	}
	var got TillCustomer
	for _, c := range decode[[]TillCustomer](t, te.do(http.MethodGet, "/api/v1/till/customers", nil, ck)) {
		if c.ID == created.Customer.ID {
			got = c
		}
	}
	if got.CreditLimit != "5000.00" || got.Outstanding != "0.00" {
		t.Fatalf("list %+v", got)
	}
}

func TestTillQuickItemAndWriteOff(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009203", "cashier")

	bc := decode[GeneratedBarcode](t, te.do(http.MethodGet, "/api/v1/till/barcode/generate", nil, ck))
	rec := te.do(http.MethodPost, "/api/v1/till/quick-item", QuickItemRequest{Name: "Quick thing", Price: "45", Qty: "2", Barcode: bc.Barcode, UnitID: 1}, ck)
	if p := decode[TillProduct](t, rec); rec.Code != 200 || p.ID == 0 || p.SellingPrice != "45.00" || p.Barcode != bc.Barcode {
		t.Fatalf("quick item %d %+v", rec.Code, p)
	}

	lots := decode[[]Lot](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/lots", f.expiringID), nil, ck))
	expired := lots[0] // FEFO: the expired one sorts first
	rec = te.do(http.MethodPost, "/api/v1/till/write-off", WriteOffRequest{ProductID: f.expiringID, BatchID: expired.BatchID, Quantity: expired.QtyRemaining, Note: "expired " + expired.ExpiryDate}, ck)
	if rec.Code != 204 {
		t.Fatalf("write-off %d %s", rec.Code, rec.Body)
	}
	opts := decode[map[string][]Lot](t, te.do(http.MethodGet, "/api/v1/till/price-options", nil, ck))
	if _, ok := opts[strconv.FormatInt(f.expiringID, 10)]; ok {
		t.Fatal("after writing off the expired lot the product must stop prompting")
	}
}
```

Run: `go test ./internal/api -run 'TestTillCustomers|TestTillQuickItem' -v`
Expected: FAIL (404s, because the routes aren't registered).

- [ ] **Step 3: Implement.** In `till.go`:
- add fields to `tillServices`: `customers *customers.Service` and `stock *stock.Service`
- initialise them with `customers.NewService(db)` and `stock.NewService(db)`
- add `h.registerTillCustomers(g)` to `registerTill`

Create `internal/api/till_customers.go`:

```go
package api

import (
	"net/http"
	"strconv"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/customers"
	"karots-suite/internal/features/products"
	"karots-suite/internal/features/stock"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerTillCustomers(g *echo.Group) {
	g.GET("/customers", h.TillCustomers)
	g.POST("/customers", h.TillCreateCustomer)
	g.PATCH("/customers/:id/credit-limit", h.TillSetCreditLimit, middleware.RequireManageCredit())
	g.GET("/barcode/generate", h.TillGenerateBarcode)
	g.POST("/quick-item", h.TillQuickItem)
	g.POST("/write-off", h.TillWriteOff)
}

func tillCustomer(c customers.Customer) TillCustomer {
	return TillCustomer{ID: c.ID, Name: c.Name, Phone: strOr(c.Phone),
		CreditLimit: dec2(c.CreditLimit), Outstanding: dec2(c.OutstandingBalance)}
}

// TillCustomers returns every active customer; the till filters client-side
// (name/phone, top 50), exactly as karots-pos did.
func (h *handlers) TillCustomers(c echo.Context) error {
	rows, err := h.t.customers.List(c.Request().Context(), "")
	if err != nil {
		return err
	}
	out := make([]TillCustomer, 0, len(rows))
	for _, r := range rows {
		out = append(out, tillCustomer(r))
	}
	return c.JSON(http.StatusOK, out)
}

// TillCreateCustomer adds a customer from the till. Phone is required — it is
// the key that stops duplicate credit accounts; a matching phone returns the
// existing customer (200, existed) instead of creating a second one.
func (h *handlers) TillCreateCustomer(c echo.Context) error {
	var req CustomerCreateRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	phone := strings.TrimSpace(req.Phone)
	in := customers.CreateInput{Name: strings.TrimSpace(req.Name), Phone: &phone, CreditLimit: req.CreditLimit}
	if err := c.Validate(&in); err != nil {
		return err
	}
	cust, existed, err := h.t.customers.Create(c.Request().Context(), in)
	if err != nil {
		return err
	}
	status := http.StatusCreated
	if existed {
		status = http.StatusOK
	}
	return c.JSON(status, CustomerCreated{Customer: tillCustomer(*cust), Existed: existed})
}

func (h *handlers) TillSetCreditLimit(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	var req CreditLimitRequest
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.CreditLimit) == "" {
		return apperr.Validation("enter a credit limit")
	}
	if err := h.t.customers.SetCreditLimit(c.Request().Context(), id, req.CreditLimit); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "customer", strconv.FormatInt(id, 10), "credit limit set at till: "+req.CreditLimit)
	return c.NoContent(http.StatusNoContent)
}

func (h *handlers) TillGenerateBarcode(c echo.Context) error {
	code, err := h.t.products.GenerateBarcode(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, GeneratedBarcode{Barcode: code})
}

// TillQuickItem sells something not yet in the catalogue: a minimal product with
// its stock seeded, flagged for admin review (products.QuickCreate).
func (h *handlers) TillQuickItem(c echo.Context) error {
	var req QuickItemRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	p, err := h.t.products.QuickCreate(c.Request().Context(), products.QuickInput{
		Name: req.Name, Price: req.Price, Qty: req.Qty, Barcode: req.Barcode, UnitID: req.UnitID,
	}, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	h.audit(c, audit.ActionCreate, "product", strconv.FormatInt(p.ID, 10), "quick-add at till: "+p.Name)
	return c.JSON(http.StatusOK, tillProduct(*p))
}

// TillWriteOff writes a lot off as damage (batch-specific) so the till stops
// offering it. Same permission as karots-pos's cashier damage route (any role).
func (h *handlers) TillWriteOff(c echo.Context) error {
	var req WriteOffRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := stock.ConsumeInput{ProductID: req.ProductID, Quantity: req.Quantity, Reason: "damage", Note: req.Note, BatchID: req.BatchID}
	if err := c.Validate(&in); err != nil {
		return err
	}
	if err := h.t.stock.Consume(c.Request().Context(), in, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	h.audit(c, audit.ActionUpdate, "product", strconv.FormatInt(req.ProductID, 10), "wrote off at till: "+req.Note)
	return c.NoContent(http.StatusNoContent)
}
```

If `products.QuickCreate` returns a product without its joined fields (`UnitAbbr` is empty), re-read it with `h.t.products.Get(ctx, p.ID)` before mapping.

- [ ] **Step 4: Run the tests.**

Run: `make types && go vet ./... && go test ./internal/api -run 'TestTill' -v`
Expected: PASS.

- [ ] **Step 5: Commit.**

```bash
git add internal/api web/src/api/types.gen.ts
git commit -m "feat(api): till customers, credit-limit edit, quick item, expired-lot write-off"
```

---

### Task 3: Drawer API and money-slip printing

**Files:**
- Create:
  - `internal/api/dto_drawer.go`
  - `internal/api/till_drawer.go`
  - `internal/api/till_print.go`
  - `internal/api/till_drawer_test.go`
  - `internal/features/cashregister/breakdown.go`
- Modify:
  - `internal/api/api.go` (`Deps.KickDrawer`)
  - `internal/api/till.go`
  - `internal/api/testutil_test.go`
  - `internal/app/services.go` (`Services.KickDrawer`)
  - `internal/app/app.go`
  - `tygo.yaml`
- Delete: `internal/features/cashregister/api.go` (after moving its two helpers)

**Interfaces:**
- Consumes (Tasks 1–2): `tillServices`, `pathID`, `dec2`, `h.audit`, `login`, `decode`.
- Produces:
  - `GET /till/drawer` → `DrawerState`
  - `POST /till/drawer/open` (`OpenDrawerRequest`) → 201 `ReceiptRef`
  - `POST /till/drawer/close` (`CloseDrawerRequest`) → `CloseResult`
  - `POST /till/drawer/withdraw`, `POST /till/drawer/deposit` (`MoneyMoveRequest`) → `ReceiptRef`
  - `POST /till/drawer/no-sale` → 204 (403 when the drawer is disabled)
  - `POST /till/money-receipts/:id/print[?kick=1]` → 204. Errors: 409 `NO_PRINTER` or `PRINT_FAILED`.
  - Go: `Deps.KickDrawer func(context.Context)` and `(h *handlers) receiptQueue(c, cfg) string`. Task 4 uses both.

- [ ] **Step 1: Move the breakdown helpers.**
  - Create `internal/features/cashregister/breakdown.go` containing `amountFrom` and `breakdownTotal`, moved **verbatim** from `api.go`, with their doc comments and imports.
  - Then `git rm internal/features/cashregister/api.go`.
  - Run `go build ./...`. Expected: it builds. `cashregister.go` uses `amountFrom`, and nothing else used the deleted handlers.

- [ ] **Step 2: DTOs.** Create `internal/api/dto_drawer.go` and add it to `tygo.yaml` `include_files`:

```go
package api

type DrawerSession struct {
	ID          int64  `json:"id"`
	OpeningCash string `json:"openingCash"`
	OpenedAt    string `json:"openedAt"` // RFC3339
}

// DrawerSummary is this user's till session. LastBreakdown is the previous close's
// {denominationValue: count}, for "Continue last count".
type DrawerSummary struct {
	Session       *DrawerSession     `json:"session,omitempty"`
	CashSales     string             `json:"cashSales"`
	PayIns        string             `json:"payIns"`
	Withdrawals   string             `json:"withdrawals"`
	Expected      string             `json:"expected"`
	LastClosing   *string            `json:"lastClosing,omitempty"`
	LastBreakdown map[string]float64 `json:"lastBreakdown,omitempty"`
}

type Denomination struct {
	Value  string `json:"value"` // decimal string as the breakdown key, e.g. "500"
	IsNote bool   `json:"isNote"`
}

// Locker is a cashier-accessible money account: cash lockers (safe/pocket/other)
// for drawer moves, card/bank ones as card/online tender destinations.
type Locker struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"` // safe | pocket | other | card | bank
	Balance string `json:"balance"`
}

type DrawerState struct {
	Summary       DrawerSummary  `json:"summary"`
	Denominations []Denomination `json:"denominations"`
	Lockers       []Locker       `json:"lockers"`
}

type OpenDrawerRequest struct {
	Breakdown      map[string]float64 `json:"breakdown"`
	SourceLockerID int64              `json:"sourceLockerId"` // 0 = untracked
}

type CloseDrawerRequest struct {
	Breakdown    map[string]float64 `json:"breakdown"`
	DestLockerID int64              `json:"destLockerId"` // 0 = leave in drawer (if allowed)
}

type CloseResult struct {
	SessionID    int64  `json:"sessionId"`
	CashSales    string `json:"cashSales"`
	ExpectedCash string `json:"expectedCash"`
	Counted      string `json:"counted"`
	Difference   string `json:"difference"`
	ReceiptID    *int64 `json:"receiptId,omitempty"`
}

type MoneyMoveRequest struct {
	Amount          string `json:"amount"`
	Reason          string `json:"reason"`
	LockerID        int64  `json:"lockerId"`
	DeferDrawerKick bool   `json:"deferDrawerKick"` // auto-print folds the pulse into the slip job
}

// ReceiptRef points at the CR- money receipt a drawer move produced (if any).
type ReceiptRef struct {
	ReceiptID *int64 `json:"receiptId,omitempty"`
}
```

- [ ] **Step 3: Wire the drawer kicker.**
  - In `internal/app/services.go`, add `KickDrawer func(context.Context)` to `Services` and set `KickDrawer: kick` in the returned struct.
  - In `internal/api/api.go`, add `KickDrawer func(context.Context)` to `Deps`.
  - In `internal/app/app.go`, pass `KickDrawer: svc.KickDrawer`.
  - In `internal/api/testutil_test.go`, pass `KickDrawer: func(context.Context) {}` in `Register(...)`.

- [ ] **Step 4: Write the failing test.** Create `internal/api/till_drawer_test.go`:

```go
package api

import (
	"net/http"
	"testing"
)

func TestTillDrawerLifecycle(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009301", "cashier")

	st := decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	if st.Summary.Session != nil || len(st.Denominations) == 0 || st.Lockers == nil {
		t.Fatalf("fresh user: %+v", st)
	}
	rec := te.do(http.MethodPost, "/api/v1/till/drawer/open", OpenDrawerRequest{Breakdown: map[string]float64{"500": 2, "100": 3}}, ck)
	if rec.Code != 201 {
		t.Fatalf("open %d %s", rec.Code, rec.Body)
	}
	st = decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	if st.Summary.Session == nil || st.Summary.Session.OpeningCash != "1300.00" || st.Summary.Expected != "1300.00" {
		t.Fatalf("after open %+v", st.Summary)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/drawer/deposit", MoneyMoveRequest{Amount: "200", Reason: "change float"}, ck); rec.Code != 200 {
		t.Fatalf("deposit %d %s", rec.Code, rec.Body)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/drawer/withdraw", MoneyMoveRequest{Amount: "99999"}, ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("over-withdraw must be refused, got %d", rec.Code)
	}
	if rec := te.do(http.MethodPost, "/api/v1/till/drawer/withdraw", MoneyMoveRequest{Amount: "100", Reason: "bank"}, ck); rec.Code != 200 {
		t.Fatalf("withdraw %d %s", rec.Code, rec.Body)
	}
	st = decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	if st.Summary.Expected != "1400.00" || st.Summary.PayIns != "200.00" || st.Summary.Withdrawals != "100.00" {
		t.Fatalf("after moves %+v", st.Summary)
	}
	// Logout is refused while the till is open (M0 behaviour the SPA routes on).
	if rec := te.do(http.MethodPost, "/api/v1/auth/logout", nil, ck); errCode(rec) != "TILL_OPEN" {
		t.Fatalf("logout with open till: %d %s", rec.Code, rec.Body)
	}
	rec = te.do(http.MethodPost, "/api/v1/till/drawer/close", CloseDrawerRequest{Breakdown: map[string]float64{"500": 2, "100": 3}}, ck)
	res := decode[CloseResult](t, rec)
	if rec.Code != 200 || res.Counted != "1300.00" || res.ExpectedCash != "1400.00" || res.Difference != "-100.00" {
		t.Fatalf("close %d %+v", rec.Code, res)
	}
	st = decode[DrawerState](t, te.do(http.MethodGet, "/api/v1/till/drawer", nil, ck))
	if st.Summary.Session != nil || st.Summary.LastClosing == nil || st.Summary.LastBreakdown["500"] != 2 {
		t.Fatalf("after close %+v", st.Summary)
	}
}

func TestTillNoSaleNeedsDrawerEnabled(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009302", "cashier")
	te.db.Exec(`UPDATE settings SET open_cash_drawer = false`) //nolint:errcheck
	if rec := te.do(http.MethodPost, "/api/v1/till/drawer/no-sale", nil, ck); rec.Code != 403 {
		t.Fatalf("drawer disabled: %d", rec.Code)
	}
}
```

Run: `go test ./internal/api -run 'TestTillDrawer|TestTillNoSale' -v`
Expected: FAIL (404).

If the `Difference` sign convention in `cashregister.Close` is counted − expected, the test above is right. If the forked service uses the opposite sign, keep the service as it is and fix the test's expectation. Domain behaviour is forked, not changed.

- [ ] **Step 5: Implement the drawer handlers.**
  - In `till.go`, add these to `tillServices`, initialised from `db`:
    - `denoms *denominations.Service` (`denominations.NewService(db)`)
    - `lockers *lockers.Service` (`lockers.NewService(db)`)
    - `receipts *cashflow.ReceiptService` (`cashflow.NewReceiptService(db)`)
  - The user lookup in `receiptQueue` uses the existing `h.d.Auth`. Don't add an auth field.
  - Add `h.registerTillDrawer(g)`.

Create `internal/api/till_drawer.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/audit"
	"karots-suite/internal/features/cashregister"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerTillDrawer(g *echo.Group) {
	g.GET("/drawer", h.TillDrawer)
	g.POST("/drawer/open", h.TillOpenDrawer)
	g.POST("/drawer/close", h.TillCloseDrawer)
	g.POST("/drawer/withdraw", h.TillWithdraw)
	g.POST("/drawer/deposit", h.TillDeposit)
	g.POST("/drawer/no-sale", h.TillNoSale)
	g.POST("/money-receipts/:id/print", h.TillPrintMoneyReceipt)
}

func drawerSummary(sum *cashregister.Summary) DrawerSummary {
	out := DrawerSummary{CashSales: dec2(sum.CashSales), PayIns: dec2(sum.PayIns),
		Withdrawals: dec2(sum.Withdrawals), Expected: dec2(sum.Expected)}
	if s := sum.Session; s != nil {
		out.Session = &DrawerSession{ID: s.ID, OpeningCash: dec2(s.OpeningCash), OpenedAt: s.OpenedAt.Format(time.RFC3339)}
	}
	if sum.LastClosing != nil {
		v := dec2(*sum.LastClosing)
		out.LastClosing = &v
	}
	if len(sum.LastBreakdown) > 0 {
		var m map[string]float64
		if json.Unmarshal(sum.LastBreakdown, &m) == nil {
			out.LastBreakdown = m
		}
	}
	return out
}

// TillDrawer is one call for everything the drawer UI needs: this user's session
// summary, the active denominations, and the cashier-accessible lockers.
func (h *handlers) TillDrawer(c echo.Context) error {
	ctx := c.Request().Context()
	sum, err := h.d.CashRegister.Summary(ctx, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	dens, err := h.t.denoms.List(ctx, true)
	if err != nil {
		return err
	}
	lks, err := h.t.lockers.ListForCashier(ctx)
	if err != nil {
		return err
	}
	out := DrawerState{Summary: drawerSummary(sum), Denominations: make([]Denomination, 0, len(dens)), Lockers: make([]Locker, 0, len(lks))}
	for _, d := range dens {
		out.Denominations = append(out.Denominations, Denomination{Value: d.Value.String(), IsNote: d.IsNote})
	}
	for _, l := range lks {
		out.Lockers = append(out.Lockers, Locker{ID: l.ID, Name: l.Name, Kind: l.Kind, Balance: dec2(l.Balance)})
	}
	return c.JSON(http.StatusOK, out)
}

func breakdownJSON(m map[string]float64) json.RawMessage {
	if len(m) == 0 {
		return nil
	}
	b, _ := json.Marshal(m)
	return b
}

func (h *handlers) TillOpenDrawer(c echo.Context) error {
	var req OpenDrawerRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	sess, err := h.d.CashRegister.Open(c.Request().Context(), middleware.CurrentUserID(c),
		cashregister.OpenInput{Breakdown: breakdownJSON(req.Breakdown), SourceLockerID: req.SourceLockerID})
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, ReceiptRef{ReceiptID: sess.ReceiptID})
}

func (h *handlers) TillCloseDrawer(c echo.Context) error {
	var req CloseDrawerRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	res, err := h.d.CashRegister.Close(c.Request().Context(), middleware.CurrentUserID(c),
		cashregister.CloseInput{Breakdown: breakdownJSON(req.Breakdown), DestLockerID: req.DestLockerID})
	if err != nil {
		return err
	}
	out := CloseResult{SessionID: res.Session.ID, CashSales: dec2(res.CashSales), ExpectedCash: dec2(res.ExpectedCash),
		Difference: dec2(res.Difference), Counted: "0.00", ReceiptID: res.ReceiptID}
	if res.Session.ClosingCash != nil {
		out.Counted = dec2(*res.Session.ClosingCash)
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) moneyMove(c echo.Context, move func(ctx echo.Context, in cashregister.MovementInput) (*cashregister.Summary, error)) error {
	var req MoneyMoveRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := cashregister.MovementInput{Amount: req.Amount, Reason: req.Reason, CounterLockerID: req.LockerID, DeferDrawerKick: req.DeferDrawerKick}
	if err := c.Validate(&in); err != nil {
		return err
	}
	sum, err := move(c, in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, ReceiptRef{ReceiptID: sum.ReceiptID})
}

func (h *handlers) TillWithdraw(c echo.Context) error {
	return h.moneyMove(c, func(c echo.Context, in cashregister.MovementInput) (*cashregister.Summary, error) {
		return h.d.CashRegister.Withdraw(c.Request().Context(), middleware.CurrentUserID(c), in)
	})
}

func (h *handlers) TillDeposit(c echo.Context) error {
	return h.moneyMove(c, func(c echo.Context, in cashregister.MovementInput) (*cashregister.Summary, error) {
		return h.d.CashRegister.PayIn(c.Request().Context(), middleware.CurrentUserID(c), in)
	})
}

// TillNoSale pops the drawer with no transaction. Only when the shop enabled a
// drawer, and always audited — a no-sale open is a classic theft surface.
func (h *handlers) TillNoSale(c echo.Context) error {
	ctx := c.Request().Context()
	st, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	if st == nil || !st.OpenCashDrawer {
		return apperr.Forbidden("cash drawer is not enabled")
	}
	h.d.KickDrawer(ctx)
	h.audit(c, audit.ActionUpdate, "cash_drawer", "", "opened drawer (no sale)")
	return c.NoContent(http.StatusNoContent)
}
```

- [ ] **Step 6: Implement money-slip printing.** Create `internal/api/till_print.go`:

```go
package api

import (
	"bytes"
	"log"
	"net/http"
	"strings"

	"karots-suite/internal/apperr"
	"karots-suite/internal/escpos"
	"karots-suite/internal/features/cashflow"
	"karots-suite/internal/features/settings"
	"karots-suite/internal/middleware"
	"karots-suite/internal/money"
	"karots-suite/internal/printing"
	"karots-suite/internal/receiptimg"

	"github.com/labstack/echo/v4"
)

const printFailed = "Receipt didn't print — check the printer is on and connected"

// receiptQueue is the print target for the signed-in cashier: their own counter
// printer when set on their user record, else the shop-wide receipt printer
// ("" = OS default; "tcp://host:9100" = network).
func (h *handlers) receiptQueue(c echo.Context, cfg *settings.Settings) string {
	if uid := middleware.CurrentUserID(c); uid != 0 {
		if u, err := h.d.Auth.GetUser(c.Request().Context(), uid); err == nil && strings.TrimSpace(u.ReceiptPrinter) != "" {
			return u.ReceiptPrinter
		}
	}
	return cfg.ReceiptPrinter
}

// TillPrintMoneyReceipt prints a CR- slip (till open/close/withdraw/deposit).
// ?kick=1 (fresh withdraw/deposit in auto-print mode) folds the drawer pulse into
// this one job; reprints never pass it.
func (h *handlers) TillPrintMoneyReceipt(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	rec, err := h.t.receipts.Get(ctx, id)
	if err != nil {
		return err
	}
	cfg, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	target := h.receiptQueue(c, cfg)
	if strings.TrimSpace(target) == "" {
		return apperr.ConflictCode("NO_PRINTER", "No receipt printer configured")
	}
	slip := moneySlip(cfg, *rec, receiptimg.SlipOptions(ctx, cfg, nil))
	if c.QueryParam("kick") == "1" {
		slip = append(escpos.DrawerKick(*cfg), slip...)
	}
	if err := printing.Raw(ctx, target, slip); err != nil {
		log.Printf("money slip %d: print: %v", id, err)
		return apperr.ConflictCode("PRINT_FAILED", printFailed)
	}
	return c.NoContent(http.StatusNoContent)
}

// moneySlip renders a money receipt as ESC/POS with the shared receipt header and
// footer. Ported from karots-pos web/admin_money_receipts.go buildReceiptSlip.
func moneySlip(cfg *settings.Settings, r cashflow.Receipt, opts escpos.Options) []byte {
	w := escpos.Columns(cfg.ReceiptWidth)
	var b bytes.Buffer
	escpos.Init(&b)
	escpos.Header(&b, *cfg, opts)
	escpos.Title(&b, *cfg, receiptKindLabel(r.Kind), w)
	escpos.Left(&b)
	escpos.Divider(&b, *cfg, w)
	escpos.Line(&b, escpos.LeftRight("Receipt:", r.ReceiptNo, w))
	escpos.Line(&b, escpos.LeftRight("Date:", r.CreatedAt.Format("2006-01-02 15:04"), w))
	escpos.Line(&b, escpos.LeftRight("From:", escpos.ASCII(r.FromLabel), w))
	escpos.Line(&b, escpos.LeftRight("To:", escpos.ASCII(r.ToLabel), w))
	if strings.TrimSpace(r.Party) != "" {
		escpos.Line(&b, escpos.LeftRight("Party:", escpos.ASCII(r.Party), w))
	}
	if r.CreatedByName != nil && *r.CreatedByName != "" {
		escpos.Line(&b, escpos.LeftRight("By:", escpos.ASCII(*r.CreatedByName), w))
	}
	escpos.Divider(&b, *cfg, w)
	escpos.Emphasis(&b, true)
	escpos.Line(&b, escpos.LeftRight("Amount", money.Format(cfg.CurrencySymbol, r.Amount), w))
	escpos.Emphasis(&b, false)
	if strings.TrimSpace(r.Note) != "" {
		for _, ln := range escpos.Wrap(escpos.ASCII("Note: "+r.Note), w) {
			escpos.Line(&b, ln)
		}
	}
	escpos.Divider(&b, *cfg, w)
	escpos.Center(&b)
	escpos.Line(&b, "Signature: ____________________")
	escpos.Line(&b, "")
	escpos.Line(&b, "")
	escpos.Footer(&b, *cfg)
	return b.Bytes()
}

// receiptKindLabel is the human label for a core receipt kind. Plugin kinds fall
// through to their raw name (M7 can add a label hook).
func receiptKindLabel(k string) string {
	switch k {
	case "transfer":
		return "Transfer"
	case "payment":
		return "Payment"
	case "intake":
		return "Intake"
	case "supplier_payment":
		return "Supplier payment"
	case "supplier_refund":
		return "Supplier refund"
	case "customer_payment":
		return "Customer payment"
	case "expense":
		return "Expense"
	case "refund":
		return "Refund"
	case "capital":
		return "Capital"
	case "bank_charge":
		return "Bank charge"
	case "interest":
		return "Interest"
	case "adjust":
		return "Adjustment"
	case "deposit":
		return "Deposit"
	case "withdrawal":
		return "Withdrawal"
	}
	return k
}
```

The karots-pos version also mapped `billpay`, `getmoney`, `reload` and `topup`. Those are plugin kinds, and core must not name them, so they are left out on purpose. If any of the `escpos.*` helper names differ in the fork, check with `grep -n '^func ' internal/escpos/*.go` and use the forked names. Don't add new helpers.

- [ ] **Step 7: Run the tests.**

Run: `make types && go vet ./... && go test ./internal/api -v`
Expected: PASS for every api test, old and new.

- [ ] **Step 8: Commit.**

```bash
git add internal tygo.yaml web/src/api/types.gen.ts
git commit -m "feat(api): till drawer (open/close/withdraw/deposit/no-sale) + CR- slip printing; drop legacy cash-register API"
```

---

### Task 4: Sales, sale-receipt printing and held sales

**Files:**
- Create: `internal/api/dto_sale.go`, `internal/api/till_sales.go`, `internal/api/till_sales_test.go`
- Modify:
  - `internal/api/api.go` (`Deps.Sales`)
  - `internal/api/till.go`
  - `internal/api/till_print.go` (sale print)
  - `internal/api/testutil_test.go`
  - `internal/app/app.go`
  - `tygo.yaml`
  - `internal/features/heldsales/heldsales.go` (delete the `APIHandler` section and `RegisterAPI`)

**Interfaces:**
- Consumes: `receiptQueue`, `printFailed`, `seedTill`, `login`, `decode`, and `CustomerCreateRequest` (Task 2).
- Produces:
  - `POST /till/sales` (`SaleRequest`) → 201 `SaleResult`. Errors:
    - 409 `CONSUMABLE_SHORT`
    - 4xx validation for oversell, over-limit and tender problems
  - `POST /till/sales/:id/print[?kick=1]` → 204. Error: 409 `PRINT_FAILED`.
  - `GET /till/held` → `HeldSale[]`
  - `POST /till/held` (`HoldRequest`) → 201 `HeldSale`. A cart that isn't valid JSON gets 400.
  - `DELETE /till/held/:id` → 204

- [ ] **Step 1: DTOs.** Create `internal/api/dto_sale.go` and add it to `tygo.yaml`:

```go
package api

type ServiceComponent struct {
	ProductID int64  `json:"productId"`
	Quantity  string `json:"quantity"`
}

// SaleItemRequest never carries a price for stocked goods: the server prices from
// the catalogue, or from BatchID when the cashier picked a lot. PriceOverride is
// honoured only for is_service products.
type SaleItemRequest struct {
	ProductID     int64              `json:"productId"`
	Quantity      string             `json:"quantity"`
	Discount      string             `json:"discount"`
	DiscountType  string             `json:"discountType"` // "fixed" (per unit) | "percent"
	PriceOverride string             `json:"priceOverride"`
	BatchID       int64              `json:"batchId"`
	AllowOversell bool               `json:"allowOversell"`
	Serials       []string           `json:"serials"`
	Description   string             `json:"description"`
	Components    []ServiceComponent `json:"components"`
}

type PaymentRequest struct {
	Method    string            `json:"method"` // cash | card | online | credit | <plugin tender>
	Amount    string            `json:"amount"`
	Reference string            `json:"reference"`
	AccountID int64             `json:"accountId"`      // card/bank locker for card/online; 0 = none
	Meta      map[string]string `json:"meta,omitempty"` // opaque plugin-tender data
}

type SaleRequest struct {
	SaleType          string            `json:"saleType"` // retail | wholesale
	CustomerID        *int64            `json:"customerId,omitempty"`
	Discount          string            `json:"discount"`
	DiscountType      string            `json:"discountType"`
	AllowOverLimit    bool              `json:"allowOverLimit"`
	DeferDrawerKick   bool              `json:"deferDrawerKick"`
	ChangeGiven       string            `json:"changeGiven"` // "" = full computed change
	OnAccountRounding bool              `json:"onAccountRounding"`
	Items             []SaleItemRequest `json:"items"`
	Payments          []PaymentRequest  `json:"payments"`
}

type SaleResult struct {
	ID                   int64   `json:"id"`
	ReceiptNo            string  `json:"receiptNo"`
	Total                string  `json:"total"`
	PaidAmount           string  `json:"paidAmount"`
	ChangeGiven          string  `json:"changeGiven"`
	RoundingToAccount    string  `json:"roundingToAccount"`
	CustomerName         string  `json:"customerName"`
	CustomerBalanceAfter *string `json:"customerBalanceAfter,omitempty"`
	Methods              []string `json:"methods"` // distinct tender methods, for the complete overlay
}

// HeldSale.Cart is an opaque JSON document owned by the SPA (see
// web/src/features/till/cart/held.ts); the server only checks it is valid JSON.
type HeldSale struct {
	ID         int64  `json:"id"`
	Label      string `json:"label"`
	SaleType   string `json:"saleType"`
	CustomerID *int64 `json:"customerId,omitempty"`
	Discount   string `json:"discount"`
	Cart       string `json:"cart"`
	ItemCount  int    `json:"itemCount"`
	Total      string `json:"total"`
	CreatedAt  string `json:"createdAt"`
}

type HoldRequest struct {
	Label      string `json:"label"`
	SaleType   string `json:"saleType"`
	CustomerID *int64 `json:"customerId,omitempty"`
	Discount   string `json:"discount"`
	Cart       string `json:"cart"`
	ItemCount  int    `json:"itemCount"`
	Total      string `json:"total"`
}
```

- [ ] **Step 2: Wire `Deps.Sales`.**
  - Add `Sales *sales.Service` to `Deps`.
  - Pass `Sales: svc.Sales` in `app.go`.
  - In `testutil_test.go`, pass `Sales: sales.NewService(db)`.

- [ ] **Step 3: Write the failing test.** Create `internal/api/till_sales_test.go`:

```go
package api

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func sale(pid int64, qty string, pays ...PaymentRequest) SaleRequest {
	return SaleRequest{SaleType: "retail", DiscountType: "fixed",
		Items: []SaleItemRequest{{ProductID: pid, Quantity: qty, DiscountType: "fixed"}}, Payments: pays}
}

func TestTillCashSaleAndLotPrice(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009401", "cashier")

	rec := te.do(http.MethodPost, "/api/v1/till/sales", sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "150"}), ck)
	res := decode[SaleResult](t, rec)
	if rec.Code != 201 || res.ReceiptNo == "" || res.Total != "100.00" || res.ChangeGiven != "50.00" || len(res.Methods) != 1 {
		t.Fatalf("cash sale %d %+v", rec.Code, res)
	}

	lots := decode[[]Lot](t, te.do(http.MethodGet, fmt.Sprintf("/api/v1/till/products/%d/lots", f.lotID), nil, ck))
	var dear Lot
	for _, l := range lots {
		if l.Price == "120.00" {
			dear = l
		}
	}
	req := sale(f.lotID, "1", PaymentRequest{Method: "cash", Amount: "120"})
	req.Items[0].BatchID = dear.BatchID
	if res := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", req, ck)); res.Total != "120.00" {
		t.Fatalf("picked lot must price at 120: %+v", res)
	}

	// Keep: the customer leaves Rs 5 of a Rs 50 change behind.
	req = sale(f.plainID, "1", PaymentRequest{Method: "cash", Amount: "150"})
	req.ChangeGiven = "45.00"
	if res := decode[SaleResult](t, te.do(http.MethodPost, "/api/v1/till/sales", req, ck)); res.ChangeGiven != "45.00" {
		t.Fatalf("kept change: %+v", res)
	}
}

func TestTillOversellNeedsConfirm(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	ck := te.login(t, "0770009402", "cashier")
	if rec := te.do(http.MethodPost, "/api/v1/till/sales", sale(f.scarceID, "2", PaymentRequest{Method: "cash", Amount: "500"}), ck); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("selling 2 of 1 without confirm must fail, got %d", rec.Code)
	}
	req := sale(f.scarceID, "2", PaymentRequest{Method: "cash", Amount: "500"})
	req.Items[0].AllowOversell = true
	if rec := te.do(http.MethodPost, "/api/v1/till/sales", req, ck); rec.Code != 201 {
		t.Fatalf("found-at-till sale %d %s", rec.Code, rec.Body)
	}
}

func TestTillOverLimitIsGatedByFlag(t *testing.T) {
	te := newTestEnv(t)
	f := te.seedTill(t)
	cashier := te.login(t, "0770009403", "cashier")
	admin := te.login(t, "0770009404", "admin")
	phone := fmt.Sprintf("07%08d", time.Now().UnixNano()%100_000_000)
	cust := decode[CustomerCreated](t, te.do(http.MethodPost, "/api/v1/till/customers", CustomerCreateRequest{Name: "Limit Cust", Phone: phone, CreditLimit: "100"}, cashier)).Customer

	req := sale(f.plainID, "2", PaymentRequest{Method: "credit", Amount: "200"})
	req.CustomerID = &cust.ID
	req.AllowOverLimit = true // a cashier without the flag: the server must strip this
	if rec := te.do(http.MethodPost, "/api/v1/till/sales", req, cashier); rec.Code < 400 || rec.Code >= 500 {
		t.Fatalf("cashier over limit must be refused, got %d", rec.Code)
	}
	rec := te.do(http.MethodPost, "/api/v1/till/sales", req, admin)
	if res := decode[SaleResult](t, rec); rec.Code != 201 || res.CustomerBalanceAfter == nil {
		t.Fatalf("admin over-limit approval %d %s", rec.Code, rec.Body)
	}
}

func TestTillHeldSales(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009405", "cashier")
	if rec := te.do(http.MethodPost, "/api/v1/till/held", HoldRequest{Label: "x", SaleType: "retail", Cart: "{not json"}, ck); rec.Code != 400 {
		t.Fatalf("bad cart %d", rec.Code)
	}
	rec := te.do(http.MethodPost, "/api/v1/till/held", HoldRequest{Label: "Kamal", SaleType: "retail", Discount: "10", Cart: `{"v":2,"lines":[],"billDiscountType":"percent"}`, ItemCount: 2, Total: "340"}, ck)
	held := decode[HeldSale](t, rec)
	if rec.Code != 201 || held.ID == 0 || held.Cart == "" {
		t.Fatalf("hold %d %+v", rec.Code, held)
	}
	list := decode[[]HeldSale](t, te.do(http.MethodGet, "/api/v1/till/held", nil, ck))
	if len(list) != 1 || list[0].Label != "Kamal" || list[0].Discount != "10.00" {
		t.Fatalf("list %+v", list)
	}
	if rec := te.do(http.MethodDelete, fmt.Sprintf("/api/v1/till/held/%d", held.ID), nil, ck); rec.Code != 204 {
		t.Fatalf("delete %d", rec.Code)
	}
	if list := decode[[]HeldSale](t, te.do(http.MethodGet, "/api/v1/till/held", nil, ck)); len(list) != 0 {
		t.Fatalf("after delete %+v", list)
	}
}

func TestTillPrintUnknownSale(t *testing.T) {
	te := newTestEnv(t)
	ck := te.login(t, "0770009406", "cashier")
	if rec := te.do(http.MethodPost, "/api/v1/till/sales/999999999/print", nil, ck); rec.Code != 404 {
		t.Fatalf("unknown sale print %d", rec.Code)
	}
}
```

Run: `go test ./internal/api -run 'TestTill(Cash|Oversell|OverLimit|Held|Print)' -v`
Expected: FAIL (404).

- [ ] **Step 4: Implement.**
  - In `till.go`, add `held *heldsales.Service` and `warranty *warranty.Service` to `tillServices`.
  - Add `h.registerTillSales(g)`.

Create `internal/api/till_sales.go`:

```go
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"karots-suite/internal/apperr"
	"karots-suite/internal/features/heldsales"
	"karots-suite/internal/features/sales"
	"karots-suite/internal/middleware"

	"github.com/labstack/echo/v4"
)

func (h *handlers) registerTillSales(g *echo.Group) {
	g.POST("/sales", h.TillCreateSale)
	g.POST("/sales/:id/print", h.TillPrintSale)
	g.GET("/held", h.TillHeld)
	g.POST("/held", h.TillHold)
	g.DELETE("/held/:id", h.TillDeleteHeld)
}

func saleInput(r SaleRequest) sales.CreateInput {
	in := sales.CreateInput{
		CustomerID: r.CustomerID, SaleType: r.SaleType, Discount: r.Discount, DiscountType: r.DiscountType,
		AllowOverLimit: r.AllowOverLimit, ChangeGiven: r.ChangeGiven, OnAccountRounding: r.OnAccountRounding,
		DeferDrawerKick: r.DeferDrawerKick,
		Items:           make([]sales.ItemInput, 0, len(r.Items)),
		Payments:        make([]sales.PaymentInput, 0, len(r.Payments)),
	}
	for _, it := range r.Items {
		comps := make([]sales.ServiceComponent, 0, len(it.Components))
		for _, c := range it.Components {
			comps = append(comps, sales.ServiceComponent{ProductID: c.ProductID, Quantity: c.Quantity})
		}
		in.Items = append(in.Items, sales.ItemInput{
			ProductID: it.ProductID, Quantity: it.Quantity, Discount: it.Discount, DiscountType: it.DiscountType,
			PriceOverride: it.PriceOverride, BatchID: it.BatchID, AllowOversell: it.AllowOversell,
			Serials: it.Serials, Description: it.Description, Components: comps,
		})
	}
	for _, p := range r.Payments {
		pi := sales.PaymentInput{Method: p.Method, Amount: p.Amount, AccountID: p.AccountID, Meta: p.Meta}
		if ref := strings.TrimSpace(p.Reference); ref != "" {
			pi.Reference = &ref
		}
		in.Payments = append(in.Payments, pi)
	}
	return in
}

func saleResult(d *sales.Detail) SaleResult {
	s := d.Sale
	out := SaleResult{ID: s.ID, ReceiptNo: s.ReceiptNo, Total: dec2(s.Total), PaidAmount: dec2(s.PaidAmount),
		ChangeGiven: dec2(s.ChangeGiven), RoundingToAccount: dec2(s.RoundingToAccount),
		CustomerName: strOr(s.CustomerName), Methods: []string{}}
	if s.CustomerBalanceAfter != nil {
		v := dec2(*s.CustomerBalanceAfter)
		out.CustomerBalanceAfter = &v
	}
	seen := map[string]bool{}
	for _, p := range d.Payments {
		if !seen[p.Method] {
			seen[p.Method] = true
			out.Methods = append(out.Methods, p.Method)
		}
	}
	return out
}

// TillCreateSale posts a sale. The server recomputes every amount; the over-limit
// approval is honoured only for a user the role/flag permits — never the client alone.
func (h *handlers) TillCreateSale(c echo.Context) error {
	var req SaleRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	in := saleInput(req)
	if err := c.Validate(&in); err != nil {
		return err
	}
	if in.AllowOverLimit && !middleware.MayManageCredit(middleware.CurrentRole(c), middleware.CanManageCredit(c)) {
		in.AllowOverLimit = false
	}
	detail, err := h.d.Sales.Create(c.Request().Context(), in, middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, saleResult(detail))
}

func heldDTO(x heldsales.HeldSale) HeldSale {
	return HeldSale{ID: x.ID, Label: strOr(x.Label), SaleType: x.SaleType, CustomerID: x.CustomerID,
		Discount: dec2(x.Discount), Cart: string(x.Cart), ItemCount: x.ItemCount, Total: dec2(x.Total),
		CreatedAt: x.CreatedAt.Format(time.RFC3339)}
}

func (h *handlers) TillHeld(c echo.Context) error {
	rows, err := h.t.held.List(c.Request().Context(), middleware.CurrentUserID(c))
	if err != nil {
		return err
	}
	out := make([]HeldSale, 0, len(rows))
	for _, r := range rows {
		out = append(out, heldDTO(r))
	}
	return c.JSON(http.StatusOK, out)
}

func (h *handlers) TillHold(c echo.Context) error {
	var req HoldRequest
	if err := c.Bind(&req); err != nil {
		return apperr.BadRequest("invalid request body")
	}
	if !json.Valid([]byte(req.Cart)) {
		return apperr.BadRequest("cart must be JSON")
	}
	in := heldsales.Input{Label: req.Label, SaleType: req.SaleType, CustomerID: req.CustomerID,
		Discount: req.Discount, Cart: json.RawMessage(req.Cart), ItemCount: req.ItemCount, Total: req.Total}
	if err := c.Validate(&in); err != nil {
		return err
	}
	held, err := h.t.held.Hold(c.Request().Context(), middleware.CurrentUserID(c), in)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, heldDTO(*held))
}

func (h *handlers) TillDeleteHeld(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	if err := h.t.held.Delete(c.Request().Context(), id, middleware.CurrentUserID(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
```

Append to `internal/api/till_print.go` (add the imports `context`, `github.com/shopspring/decimal`, `karots-suite/internal/datetime` and `karots-suite/internal/features/sales`):

```go
// TillPrintSale sends a sale receipt to the thermal printer as ESC/POS. ?kick=1
// (the till's auto-print of a fresh cash sale) folds the drawer pulse into THIS
// job — one USB job, no inter-job gap. Reprints never pass kick.
func (h *handlers) TillPrintSale(c echo.Context) error {
	id, err := pathID(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	detail, err := h.d.Sales.Get(ctx, id)
	if err != nil {
		return err
	}
	cfg, err := h.d.Settings.Get(ctx)
	if err != nil {
		return err
	}
	opts := receiptimg.SlipOptions(ctx, cfg, nil)
	opts.Serials = h.saleSerials(ctx, id)
	opts.CustomerDue = h.customerDue(ctx, detail)
	payload := escpos.Document(*detail, *cfg, opts)
	if c.QueryParam("kick") == "1" && paidCash(detail) {
		payload = append(escpos.DrawerKick(*cfg), payload...)
	}
	if err := escpos.Send(ctx, h.receiptQueue(c, cfg), payload); err != nil {
		log.Printf("sale %d: print: %v", id, err)
		return apperr.ConflictCode("PRINT_FAILED", printFailed)
	}
	return c.NoContent(http.StatusNoContent)
}

func paidCash(d *sales.Detail) bool {
	for _, p := range d.Payments {
		if p.Method == "cash" && p.Amount.IsPositive() {
			return true
		}
	}
	return false
}

// customerDue is the customer's outstanding balance for the "Total due" line on
// credit receipts. Best-effort: zero on any failure.
func (h *handlers) customerDue(ctx context.Context, d *sales.Detail) decimal.Decimal {
	if d.Sale.CustomerID == nil {
		return decimal.Zero
	}
	cust, err := h.t.customers.Get(ctx, *d.Sale.CustomerID)
	if err != nil {
		return decimal.Zero
	}
	return cust.OutstandingBalance
}

// saleSerials labels the serial-tracked units on the receipt ("SN: … (wty …)").
func (h *handlers) saleSerials(ctx context.Context, saleID int64) map[int64][]string {
	units, err := h.t.warranty.UnitsForSale(ctx, saleID)
	if err != nil || len(units) == 0 {
		return nil
	}
	m := make(map[int64][]string, len(units))
	for _, u := range units {
		label := "SN: " + u.SerialNo
		if u.WarrantyMonths > 0 {
			label += " (wty " + datetime.Date(u.WarrantyUntil) + ")"
		}
		m[u.ProductID] = append(m[u.ProductID], label)
	}
	return m
}
```

Delete the legacy held-sales handlers: in `internal/features/heldsales/heldsales.go`, remove `type APIHandler`, `NewAPIHandler`, its methods and `RegisterAPI` (lines 134 to the end), then drop any imports that are now unused. `go build ./...` must pass.

- [ ] **Step 5: Run the tests.**

Run: `make types && go vet ./... && go test ./... 2>&1 | tail -20`
Expected: every package passes. Remember that DB tests need `DATABASE_URL` (`set -a; . ./.env; set +a`).

- [ ] **Step 6: Commit.**

```bash
git add internal tygo.yaml web/src/api/types.gen.ts
git commit -m "feat(api): till sales (over-limit gated by flag), sale receipt print with single-job drawer kick, held sales"
```

---

### Task 5: Money formatting and till totals (pure TS)

**Files:**
- Create:
  - `web/src/lib/money.ts`, `web/src/lib/money.test.ts`
  - `web/src/features/till/cart/types.ts`
  - `web/src/features/till/cart/totals.ts`, `web/src/features/till/cart/totals.test.ts`

**Interfaces:**
- Produces:
  - from `money.ts`: `num(v: unknown): number`, `r2(n: number): number`, `money(sym: string, v: unknown): string`, `qtyText(v: unknown): string`, `dec(n: number): string`
  - the cart types: `CartLine`, `Payment`, `TillState`, `ServiceLineInput`, `SaleType`, `DiscountType`, `Notice`, `Component`
  - from `totals.ts`: `lineGross`, `lineDiscount`, `lineNet`, `subtotal`, `itemDiscountTotal`, `billDiscount`, `taxTotal`, `total`, `saleTotal(s: TillState)`, `paidTotal`, `accountTotal`, `changeDue`, `roundable`, `keptAmount`, `actualChange`, `balanceDue`, `notesFor`, `tenderIssue`, `quickCash`, `coinKeep`

- [ ] **Step 1: Write the failing tests.** Create `web/src/lib/money.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { dec, money, num, qtyText, r2 } from './money'

describe('money', () => {
  it('formats with the shop symbol, grouping and 2 decimals', () => {
    expect(money('Rs.', 1234.5)).toBe('Rs. 1,234.50')
    expect(money('Rs.', '99')).toBe('Rs. 99.00')
    expect(money('Rs.', -50)).toBe('Rs. -50.00')
  })
  it('parses loosely and never returns NaN', () => {
    expect(num('1,200')).toBe(1200)
    expect(num('abc')).toBe(0)
    expect(num(undefined)).toBe(0)
    expect(num(' 12.5 ')).toBe(12.5)
  })
  it('rounds to cents without float drift', () => {
    expect(r2(0.1 + 0.2)).toBe(0.3)
    expect(r2(1.005)).toBe(1.01)
    expect(dec(10)).toBe('10.00')
  })
  it('prints quantities without forced decimals', () => {
    expect(qtyText(137)).toBe('137')
    expect(qtyText(1.5)).toBe('1.5')
    expect(qtyText(0.3333333)).toBe('0.333')
  })
})
```

Create `web/src/features/till/cart/totals.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import type { CartLine, Payment } from './types'
import {
  accountTotal, billDiscount, changeDue, coinKeep, keptAmount, lineDiscount, lineNet, notesFor,
  paidTotal, quickCash, roundable, taxTotal, tenderIssue, total,
} from './totals'

const line = (p: Partial<CartLine>): CartLine => ({
  key: 1, productId: 1, name: 'x', unitPrice: 100, taxRate: 0, qty: 1, stock: 10, isService: false,
  allowDecimal: false, batchId: 0, batchRemaining: 0, batchLabel: '', allowOversell: false,
  discount: '', discountType: 'fixed', trackSerial: false, warrantyMonths: 0, serials: [],
  priceOverride: '', description: '', components: [], ...p,
})
const pay = (method: string, amount: string): Payment => ({ method, amount, reference: '', accountId: '', meta: {} })

describe('line maths (mirrors the server)', () => {
  it('fixed discount is per unit', () => {
    const l = line({ qty: 2, discount: '10' })
    expect(lineDiscount(l)).toBe(20)
    expect(lineNet(l)).toBe(180)
  })
  it('percent discount is off the line gross', () => {
    expect(lineDiscount(line({ qty: 2, discount: '10', discountType: 'percent' }))).toBe(20)
  })
  it('a discount never exceeds the line', () => {
    expect(lineDiscount(line({ discount: '500' }))).toBe(100)
    expect(lineDiscount(line({ discount: '-5' }))).toBe(0)
  })
})

describe('bill totals', () => {
  const ls = [line({ qty: 2, discount: '10' }), line({ key: 2, unitPrice: 50, taxRate: 10 })]
  it('bill percent applies to the net after item discounts', () => {
    expect(billDiscount(ls, '10', 'percent')).toBe(23) // (200-20+50) * 10%
  })
  it('tax is on each line net, before the bill discount (server rule)', () => {
    expect(taxTotal(ls)).toBe(5)
  })
  it('total = gross − item disc − bill disc + tax, never negative', () => {
    expect(total(ls, '10', 'percent')).toBe(212) // 250 - 20 - 23 + 5
    expect(total(ls, '9999', 'fixed')).toBe(5)
  })
})

describe('tenders', () => {
  it('an on-account line is a debt, not money: excluded from paid, no change on it', () => {
    const ps = [pay('cash', '50'), pay('credit', '50')]
    expect(paidTotal(ps)).toBe(50)
    expect(accountTotal(ps)).toBe(50)
    expect(changeDue(100, ps)).toBe(0)
    expect(roundable(100, [pay('cash', '150')])).toBe(true)
    expect(roundable(100, [pay('cash', '150'), pay('credit', '1')])).toBe(false)
  })
  it('kept amount is clamped to [0, change]; blank keeps nothing', () => {
    expect(keptAmount('', 50)).toBe(0)
    expect(keptAmount('80', 50)).toBe(50)
    expect(keptAmount('-3', 50)).toBe(0)
    expect(coinKeep(47.5)).toBe(7.5)
  })
  it('tenderIssue: shortfall, over-covered by an account line, or fine', () => {
    expect(tenderIssue(100, 60, 0, true)).toEqual({ kind: 'shortfall', amount: 40, hasCustomer: true })
    expect(tenderIssue(100, 80, 50, true)).toEqual({ kind: 'overcovered', amount: 50, hasCustomer: true })
    expect(tenderIssue(100, 150, 0, false)).toBeNull()
  })
  it('notesFor breaks change down high→low in whole cents', () => {
    expect(notesFor(1270, [5000, 1000, 500, 100, 50, 20, 10, 5, 2, 1])).toEqual([
      { value: 1000, qty: 1 }, { value: 100, qty: 2 }, { value: 50, qty: 1 }, { value: 20, qty: 1 },
    ])
  })
  it('quickCash offers exact then round notes at or above the due', () => {
    expect(quickCash(1270)).toEqual([1270, 1500, 2000, 5000])
    expect(quickCash(6200)).toEqual([6200, 6500, 7000])
    expect(quickCash(0)).toEqual([])
  })
})
```

Run: `cd web && bunx vitest run src/lib/money.test.ts src/features/till/cart/totals.test.ts`
Expected: FAIL (modules not found).

- [ ] **Step 2: Implement `money.ts`.** Create `web/src/lib/money.ts`:

```ts
const nf = new Intl.NumberFormat('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })

/** Loose number parse for form input / decimal strings; never NaN. */
export function num(v: unknown): number {
  const n = typeof v === 'number' ? v : Number(String(v ?? '').replace(/,/g, '').trim())
  return Number.isFinite(n) ? n : 0
}

/** Round to cents (EPSILON nudge so 1.005 → 1.01, like the server's decimal). */
export const r2 = (n: number) => Math.round((n + Number.EPSILON) * 100) / 100

/** "Rs. 1,234.50" — the one money formatter every screen uses. */
export const money = (sym: string, v: unknown) => `${sym} ${nf.format(r2(num(v)))}`

/** A quantity without forced decimals: 137 → "137", 1.5 → "1.5". */
export const qtyText = (v: unknown) => String(Number(num(v).toFixed(3)))

/** A number as the decimal string the API expects. */
export const dec = (n: number) => r2(n).toFixed(2)
```

- [ ] **Step 3: Implement the types.** Create `web/src/features/till/cart/types.ts`:

```ts
export type SaleType = 'retail' | 'wholesale'
export type DiscountType = 'fixed' | 'percent'
export type Component = { productId: number; quantity: string }

/** One cart line. Numbers here are a live preview; the server re-prices on post. */
export type CartLine = {
  key: number // unique per line: a product can be two lines (two lots, two service jobs)
  productId: number
  name: string
  unitPrice: number
  taxRate: number
  qty: number
  stock: number // MAX_STOCK for services: they never trip the found-at-till prompt
  isService: boolean
  allowDecimal: boolean
  batchId: number // the lot picked at "which price?" (0 = FEFO)
  batchRemaining: number
  batchLabel: string
  allowOversell: boolean // found-at-till confirmed for this line
  discount: string
  discountType: DiscountType // fixed is PER UNIT
  trackSerial: boolean
  warrantyMonths: number
  serials: string[]
  priceOverride: string // service lines only
  description: string
  components: Component[]
  plugin?: Record<string, unknown> // opaque data a plugin slot attached (M7)
}

export type Payment = {
  method: string // '' = not picked yet
  amount: string
  reference: string
  accountId: string // card/bank locker id for card/online
  meta: Record<string, string> // plugin tender data
}

export type ServiceLineInput = {
  productId: number
  name: string
  price: number
  qty?: number
  unitPrice?: number
  description?: string
  components?: Component[]
  plugin?: Record<string, unknown>
}

export type Notice = { tone: 'success' | 'error'; text: string } | null

export type TillState = {
  lines: CartLine[]
  seq: number
  lastAddedKey: number
  saleType: SaleType
  customerId: number | null
  billDiscount: string
  billDiscountType: DiscountType
  tenders: Payment[]
  draft: Payment
  keep: string
  roundToAccount: boolean
  overLimitApproved: boolean
  postSaleChange: number // auto-print mode: last sale's change, shown until the next add
  notice: Notice
}
```

- [ ] **Step 4: Implement `totals.ts`.** Create `web/src/features/till/cart/totals.ts`:

```ts
import { num, r2 } from '@/lib/money'
import type { CartLine, DiscountType, Payment, TillState } from './types'

export const lineGross = (l: CartLine) => num(l.qty) * num(l.unitPrice)

/** Fixed is per unit (× qty); percent is off the line gross; clamped to [0, gross]. */
export function lineDiscount(l: CartLine) {
  const g = lineGross(l)
  const v = num(l.discount)
  const d = l.discountType === 'percent' ? (g * v) / 100 : v * num(l.qty)
  return Math.min(Math.max(d, 0), g)
}

export const lineNet = (l: CartLine) => lineGross(l) - lineDiscount(l)
export const subtotal = (ls: CartLine[]) => ls.reduce((s, l) => s + lineGross(l), 0)
export const itemDiscountTotal = (ls: CartLine[]) => ls.reduce((s, l) => s + lineDiscount(l), 0)

/** Bill discount resolved against the net after item discounts. */
export function billDiscount(ls: CartLine[], value: string, type: DiscountType) {
  const base = subtotal(ls) - itemDiscountTotal(ls)
  const v = num(value)
  const d = type === 'percent' ? (base * v) / 100 : v
  return Math.min(Math.max(d, 0), base)
}

/** Tax on each discounted line net (the server's rule). */
export const taxTotal = (ls: CartLine[]) => ls.reduce((s, l) => s + (lineNet(l) * l.taxRate) / 100, 0)

export const total = (ls: CartLine[], value: string, type: DiscountType) =>
  r2(Math.max(0, subtotal(ls) - itemDiscountTotal(ls) - billDiscount(ls, value, type) + taxTotal(ls)))

export const saleTotal = (s: TillState) => total(s.lines, s.billDiscount, s.billDiscountType)

/** Money actually handed over. An on-account line is a debt, not money. */
export const paidTotal = (ps: Payment[]) => ps.filter((p) => p.method !== 'credit').reduce((s, p) => s + num(p.amount), 0)
export const accountTotal = (ps: Payment[]) => ps.filter((p) => p.method === 'credit').reduce((s, p) => s + num(p.amount), 0)
export const changeDue = (t: number, ps: Payment[]) => r2(Math.max(0, paidTotal(ps) + accountTotal(ps) - t))
export const balanceDue = (t: number, ps: Payment[]) => r2(Math.max(0, t - paidTotal(ps) - accountTotal(ps)))

/** Rounding only on a pure cash-change sale (the server zeroes change with an account line). */
export const roundable = (t: number, ps: Payment[]) => accountTotal(ps) === 0 && changeDue(t, ps) > 0.005

/** What the shop keeps: the cashier's entry clamped to [0, change]; blank = nothing. */
export function keptAmount(keep: string, change: number) {
  if (keep.trim() === '') return 0
  return Math.min(change, Math.max(0, num(keep)))
}
export const actualChange = (change: number, kept: number) => r2(Math.max(0, change - kept))

/** The "Keep Rs x (no coins)" suggestion: the part of the change under Rs 10. */
export const coinKeep = (change: number) => r2(change % 10)

/** Greedy high→low note breakdown in whole cents (advisory only). */
export function notesFor(amount: number, denoms: number[]) {
  let rem = Math.round(amount * 100)
  const out: { value: number; qty: number }[] = []
  for (const v of [...denoms].sort((a, b) => b - a)) {
    const c = Math.round(v * 100)
    if (c <= 0) continue
    const qty = Math.floor(rem / c)
    if (qty > 0) {
      out.push({ value: v, qty })
      rem -= qty * c
    }
  }
  return out
}

export type TenderIssue = { kind: 'shortfall' | 'overcovered'; amount: number; hasCustomer: boolean } | null

/** The whole tender rule in one place (karots-pos pos-payment.js tenderIssue). */
export function tenderIssue(t: number, paid: number, onAccount: number, hasCustomer: boolean): TenderIssue {
  const covered = paid + onAccount
  if (covered < t - 0.005) return { kind: 'shortfall', amount: r2(t - covered), hasCustomer }
  if (onAccount > 0 && covered > t + 0.005) return { kind: 'overcovered', amount: r2(onAccount), hasCustomer }
  return null
}

/** Quick-cash chips: exact, then the next 500 / 1000, then 5000 — only ≥ the due. */
export function quickCash(due: number): number[] {
  if (due <= 0) return []
  const c = [r2(due), Math.ceil(due / 500) * 500, Math.ceil(due / 1000) * 1000, 5000]
  return [...new Set(c)].filter((v) => v >= due).slice(0, 4)
}
```

- [ ] **Step 5: Run the tests.**

Run: `cd web && bunx vitest run src/lib src/features/till && bunx tsc -b`
Expected: PASS and no type errors.

- [ ] **Step 6: Commit.**

```bash
git add web/src/lib/money.ts web/src/lib/money.test.ts web/src/features/till/cart
git commit -m "feat(web): money helpers + till totals/tender maths (mirrors karots-pos pos-payment)"
```

---

### Task 6: Cart lines, entry parsing, held blobs, reducer and persistence (pure TS)

**Files:**
- Create in `web/src/features/till/cart/`:
  - `lines.ts`, `lines.test.ts`
  - `entry.ts`, `entry.test.ts`
  - `held.ts`, `held.test.ts`
  - `store.ts`, `store.test.ts`
  - `persist.ts`, `persist.test.ts`

**Interfaces:**
- Consumes: `types.ts`, `money.ts`, and the generated `TillProduct`, `Lot` and `HeldSale`.
- Produces:
  - from `lines.ts`: `MAX_STOCK`, `lotExpired(lot, today?)`, `lotsFor(options, productId, today?)`, `lotLabel(lot)`, `unitPriceFor(p, saleType)`, `syncSerials(line)`, `addProduct(lines, seq, p, saleType, lot, lots)`, `setQty(lines, seq, key, qty, lots)`, `serviceLine(key, input)`, where `LinesResult = { lines, seq, key, notice }`
  - from `entry.ts`: `parseEntry(raw): Entry`
  - from `held.ts`: `toHeldBlob(s): string`, `fromHeld(h, seq): { lines, seq, billDiscountType }`
  - from `store.ts`: `blankDraft(method)`, `initialState(saleType)`, `tillReducer(state, action)`, `type Action`
  - from `persist.ts`: `loadCart(userId): TillState | null`, `saveCart(userId, state)`, `cartKey(userId)`

- [ ] **Step 1: Write the failing tests.**

`web/src/features/till/cart/lines.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import type { Lot, TillProduct } from '@/api/types.gen'
import { addProduct, lotExpired, lotsFor, serviceLine, setQty, unitPriceFor } from './lines'

const prod = (p: Partial<TillProduct> = {}): TillProduct => ({
  id: 7, name: 'Soap', barcode: '', sellingPrice: '100.00', wholesalePrice: '90.00', taxRate: '0',
  stockQty: '10', unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false,
  warrantyMonths: 0, emoji: '', badges: [], ...p,
})
const lot = (batchId: number, price: string, qty: string, expiryDate = ''): Lot => ({
  batchId, batchNo: '', expiryDate, qtyRemaining: qty, price, receivedAt: '2026-09-01',
})

describe('lots', () => {
  it('expired means strictly before today (best-before today still sells)', () => {
    expect(lotExpired(lot(1, '1', '1', '2026-09-27'), '2026-09-28')).toBe(true)
    expect(lotExpired(lot(1, '1', '1', '2026-09-28'), '2026-09-28')).toBe(false)
    expect(lotExpired(lot(1, '1', '1'), '2026-09-28')).toBe(false)
  })
  it('prompts on price disagreement or an expired FEFO-first lot only', () => {
    const today = '2026-09-28'
    expect(lotsFor({ 7: [lot(1, '100', '5'), lot(2, '120', '5')] }, 7, today)).toHaveLength(2)
    expect(lotsFor({ 7: [lot(1, '100', '5', '2026-09-01'), lot(2, '100', '5')] }, 7, today)).toHaveLength(2)
    expect(lotsFor({ 7: [lot(1, '100', '5'), lot(2, '100', '5')] }, 7, today)).toEqual([])
    expect(lotsFor(undefined, 7, today)).toEqual([])
  })
})

describe('adding', () => {
  it('wholesale uses the wholesale price when set', () => {
    expect(unitPriceFor(prod(), 'wholesale')).toBe(90)
    expect(unitPriceFor(prod({ wholesalePrice: '0.00' }), 'wholesale')).toBe(100)
  })
  it('merges the same product + lot, keeps two lots as two lines', () => {
    let r = addProduct([], 0, prod(), 'retail', undefined, [])
    r = addProduct(r.lines, r.seq, prod(), 'retail', undefined, [])
    expect(r.lines).toHaveLength(1)
    expect(r.lines[0].qty).toBe(2)
    r = addProduct(r.lines, r.seq, prod(), 'retail', lot(9, '120', '5'), [])
    expect(r.lines).toHaveLength(2)
    expect(r.lines[1]).toMatchObject({ unitPrice: 120, batchId: 9, batchLabel: 'in 2026-09-01' })
  })
  it('service products never trip found-at-till', () => {
    const r = addProduct([], 0, prod({ isService: true, stockQty: '0' }), 'retail', undefined, [])
    expect(r.lines[0].stock).toBeGreaterThan(1e9)
  })
  it('service lines are one per call, priced by the plugin', () => {
    const a = serviceLine(1, { productId: 3, name: 'Copy', price: 100, qty: 20, unitPrice: 5 })
    expect(a).toMatchObject({ qty: 20, unitPrice: 5, priceOverride: '5', isService: true })
  })
})

describe('quantities', () => {
  it('whole-unit products round; serial boxes follow the qty', () => {
    const r0 = addProduct([], 0, prod({ trackSerial: true }), 'retail', undefined, [])
    const r = setQty(r0.lines, r0.seq, r0.key, 2.6, [])
    expect(r.lines[0].qty).toBe(3)
    expect(r.lines[0].serials).toEqual(['', '', ''])
  })
  it('a picked lot too small spills the rest onto the next lot at its price', () => {
    const lots = [lot(1, '100', '4'), lot(2, '120', '10')]
    const r0 = addProduct([], 0, prod(), 'retail', lots[0], lots)
    const r = setQty(r0.lines, r0.seq, r0.key, 10, lots)
    expect(r.lines.map((l) => [l.qty, l.unitPrice, l.batchId])).toEqual([[4, 100, 1], [6, 120, 2]])
    expect(r.notice?.tone).toBe('success')
  })
  it('no lot left to spill onto: clamp and warn', () => {
    const lots = [lot(1, '100', '4'), lot(2, '120', '1')]
    const r0 = addProduct([], 0, prod(), 'retail', lots[0], lots)
    const r = setQty(r0.lines, r0.seq, r0.key, 10, lots)
    expect(r.lines.map((l) => l.qty)).toEqual([4, 1])
    expect(r.notice?.tone).toBe('error')
  })
})
```

`web/src/features/till/cart/entry.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { parseEntry } from './entry'

describe('parseEntry', () => {
  it('reads the phone quick-sell handoff KQ<barcode>X<qty>', () => {
    expect(parseEntry('KQ4790001X12')).toEqual({ kind: 'quick', barcode: '4790001', qty: 12 })
    expect(parseEntry('kq4790001x3')).toEqual({ kind: 'quick', barcode: '4790001', qty: 3 })
  })
  it('digits probe the barcode first; anything else is a search', () => {
    expect(parseEntry(' 4790000000011 ')).toEqual({ kind: 'barcode', code: '4790000000011' })
    expect(parseEntry('milk')).toEqual({ kind: 'search', q: 'milk' })
    expect(parseEntry('SKU-12')).toEqual({ kind: 'search', q: 'SKU-12' })
    expect(parseEntry('   ')).toEqual({ kind: 'empty' })
  })
})
```

`web/src/features/till/cart/held.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import type { HeldSale } from '@/api/types.gen'
import { fromHeld, toHeldBlob } from './held'
import { initialState, tillReducer } from './store'

const held = (cart: string): HeldSale => ({
  id: 1, label: 'x', saleType: 'retail', discount: '10.00', cart, itemCount: 1, total: '90', createdAt: '',
})

describe('held sales', () => {
  it('round-trips lines, lots, serials and a percent bill discount', () => {
    let s = initialState('retail')
    s = tillReducer(s, { type: 'addService', line: { productId: 3, name: 'Job', price: 50 } })
    s = { ...s, billDiscountType: 'percent', lines: s.lines.map((l) => ({ ...l, batchId: 4, serials: ['A1'] })) }
    const back = fromHeld(held(toHeldBlob(s)), 10)
    expect(back.billDiscountType).toBe('percent')
    expect(back.lines[0]).toMatchObject({ productId: 3, batchId: 4, serials: ['A1'], key: 11 })
    expect(back.seq).toBe(11)
  })
  it('resumes a karots-pos (array) held cart from a restored backup', () => {
    const legacy = JSON.stringify([{ id: 7, name: 'Soap', unit_price: 100, tax_rate: 0, qty: 2, stock: 10, is_service: false,
      allowDecimal: false, batch_id: 0, discount: '5', discountType: 'fixed', track_serial: false, warranty_months: 0,
      serials: [], price_override: '', description: '', components: [{ product_id: 9, quantity: '1' }] }])
    const back = fromHeld(held(legacy), 0)
    expect(back.lines[0]).toMatchObject({ productId: 7, unitPrice: 100, qty: 2, discount: '5', components: [{ productId: 9, quantity: '1' }] })
    expect(back.billDiscountType).toBe('fixed')
  })
  it('an unreadable cart resumes empty instead of crashing', () => {
    expect(fromHeld(held('{oops'), 0).lines).toEqual([])
  })
})
```

`web/src/features/till/cart/store.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import type { TillProduct } from '@/api/types.gen'
import { initialState, tillReducer } from './store'

const p: TillProduct = { id: 1, name: 'Milk', barcode: '1', sellingPrice: '100.00', wholesalePrice: '0.00', taxRate: '0',
  stockQty: '5', unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false, warrantyMonths: 0, emoji: '', badges: [] }

describe('tillReducer', () => {
  it('picking a customer un-picks the default cash tender; walk-in restores it', () => {
    let s = initialState('retail')
    expect(s.draft.method).toBe('cash')
    s = tillReducer(s, { type: 'customer', id: 5 })
    expect(s.draft.method).toBe('')
    s = tillReducer(s, { type: 'customer', id: null })
    expect(s.draft.method).toBe('cash')
  })
  it('never overrides a deliberate method or a typed amount', () => {
    let s = tillReducer(initialState('retail'), { type: 'draft', patch: { method: 'card' } })
    s = tillReducer(s, { type: 'customer', id: 5 })
    expect(s.draft.method).toBe('card')
  })
  it('commitDraft moves a priced draft into the tender list and keeps the method', () => {
    let s = tillReducer(initialState('retail'), { type: 'draft', patch: { amount: '500' } })
    s = tillReducer(s, { type: 'commitDraft' })
    expect(s.tenders).toHaveLength(1)
    expect(s.draft).toMatchObject({ method: 'cash', amount: '' })
  })
  it('quickSet sets (not adds) the quantity, idempotent on re-scan', () => {
    let s = tillReducer(initialState('retail'), { type: 'quickSet', product: p, qty: 3 })
    s = tillReducer(s, { type: 'quickSet', product: p, qty: 3 })
    expect(s.lines).toHaveLength(1)
    expect(s.lines[0].qty).toBe(3)
  })
  it('adding dismisses the post-sale change banner; newSale clears everything but sale type', () => {
    let s = tillReducer(initialState('wholesale'), { type: 'newSale', saleType: 'wholesale', change: 40 })
    expect(s.postSaleChange).toBe(40)
    s = tillReducer(s, { type: 'add', product: p, lots: [] })
    expect(s.postSaleChange).toBe(0)
    s = tillReducer(s, { type: 'newSale', saleType: 'wholesale' })
    expect(s.lines).toEqual([])
    expect(s.saleType).toBe('wholesale')
  })
})
```

`web/src/features/till/cart/persist.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest'
import { loadCart, saveCart } from './persist'
import { initialState, tillReducer } from './store'

describe('cart persistence (auto-lock / reload safety)', () => {
  beforeEach(() => sessionStorage.clear())
  it('restores the same user’s cart', () => {
    const s = tillReducer(initialState('retail'), { type: 'addService', line: { productId: 1, name: 'x', price: 5 } })
    saveCart(7, s)
    expect(loadCart(7)?.lines).toHaveLength(1)
  })
  it('never shows one user’s cart to another', () => {
    saveCart(7, tillReducer(initialState('retail'), { type: 'addService', line: { productId: 1, name: 'x', price: 5 } }))
    expect(loadCart(8)).toBeNull()
  })
  it('ignores corrupt storage and drops transient notices', () => {
    sessionStorage.setItem('ks-till-cart-9', '{bad')
    expect(loadCart(9)).toBeNull()
    saveCart(9, { ...initialState('retail'), notice: { tone: 'error', text: 'x' } })
    expect(loadCart(9)?.notice).toBeNull()
  })
})
```

Run: `cd web && bunx vitest run src/features/till/cart`
Expected: FAIL (modules not found).

- [ ] **Step 2: Implement `lines.ts`.** Create `web/src/features/till/cart/lines.ts`:

```ts
import type { Lot, TillProduct } from '@/api/types.gen'
import { num, qtyText } from '@/lib/money'
import type { CartLine, Notice, SaleType, ServiceLineInput } from './types'

export const MAX_STOCK = Number.MAX_SAFE_INTEGER
export type LinesResult = { lines: CartLine[]; seq: number; key: number; notice: Notice }

const localToday = () => new Date().toLocaleDateString('en-CA') // YYYY-MM-DD, local

/** A lot is expired the day AFTER its printed date; no expiry never expires. */
export const lotExpired = (lot: Lot, today = localToday()) => lot.expiryDate !== '' && lot.expiryDate < today

/** Lots worth asking about: prices disagree, or the lot FEFO sells first is expired. */
export function lotsFor(options: Record<string, Lot[]> | undefined, productId: number, today = localToday()): Lot[] {
  const lots = options?.[String(productId)] ?? []
  if (!lots.length) return []
  if (new Set(lots.map((l) => l.price)).size > 1) return lots
  if (lotExpired(lots[0], today)) return lots
  return []
}

/** The few words that match a lot to the package in the cashier's hand. */
export function lotLabel(lot: Lot) {
  if (lot.expiryDate) return `exp ${lot.expiryDate}`
  if (lot.batchNo) return `batch ${lot.batchNo}`
  return `in ${lot.receivedAt}`
}

export const unitPriceFor = (p: TillProduct, saleType: SaleType) =>
  saleType === 'wholesale' && num(p.wholesalePrice) > 0 ? num(p.wholesalePrice) : num(p.sellingPrice)

/** One serial box per unit, keeping what was typed. */
export function syncSerials(l: CartLine): CartLine {
  if (!l.trackSerial) return l
  const n = Math.max(0, Math.round(l.qty))
  const serials = l.serials.slice(0, n)
  while (serials.length < n) serials.push('')
  return { ...l, serials }
}

function newLine(key: number, p: TillProduct, saleType: SaleType, lot?: Lot): CartLine {
  return syncSerials({
    key, productId: p.id, name: p.name,
    unitPrice: lot ? num(lot.price) : unitPriceFor(p, saleType),
    taxRate: num(p.taxRate), qty: 1,
    stock: p.isService ? MAX_STOCK : num(p.stockQty),
    isService: p.isService, allowDecimal: p.unitAllowDecimal,
    batchId: lot?.batchId ?? 0, batchRemaining: lot ? num(lot.qtyRemaining) : 0, batchLabel: lot ? lotLabel(lot) : '',
    allowOversell: false, discount: '', discountType: 'fixed',
    trackSerial: p.trackSerial, warrantyMonths: p.warrantyMonths, serials: [],
    priceOverride: '', description: '', components: [],
  })
}

/** Add one unit. A line is product AND lot — merging on product alone would re-price. */
export function addProduct(lines: CartLine[], seq: number, p: TillProduct, saleType: SaleType, lot: Lot | undefined, lots: Lot[]): LinesResult {
  const batchId = lot?.batchId ?? 0
  const hit = lines.find((l) => l.productId === p.id && l.batchId === batchId && !l.isService)
  if (hit) return setQty(lines, seq, hit.key, hit.qty + 1, lots)
  const key = seq + 1
  return { lines: [...lines, newLine(key, p, saleType, lot)], seq: key, key, notice: null }
}

export function serviceLine(key: number, d: ServiceLineInput): CartLine {
  const qty = d.qty && d.qty > 0 ? d.qty : 1
  const unit = d.unitPrice ?? d.price
  return {
    key, productId: d.productId, name: d.name, unitPrice: unit, taxRate: 0, qty, stock: MAX_STOCK,
    isService: true, allowDecimal: false, batchId: 0, batchRemaining: 0, batchLabel: '', allowOversell: false,
    discount: '', discountType: 'fixed', trackSerial: false, warrantyMonths: 0, serials: [],
    priceOverride: String(unit), description: d.description ?? '', components: d.components ?? [], plugin: d.plugin,
  }
}

/** Set a line's qty: clamp ≥ 0, whole units round, serials follow, lots spill. */
export function setQty(lines: CartLine[], seq: number, key: number, qty: number, lots: Lot[]): LinesResult {
  const next = lines.map((l) => {
    if (l.key !== key) return l
    let q = Math.max(0, num(qty))
    if (!l.allowDecimal) q = Math.round(q)
    return syncSerials({ ...l, qty: q })
  })
  return spill(next, seq, key, lots)
}

/** A picked lot holding fewer than asked: clamp to it and open a line on the next
 * lot at ITS price — two receipt lines, matching what leaves the shelf. */
function spill(lines: CartLine[], seq: number, key: number, lots: Lot[]): LinesResult {
  const l = lines.find((x) => x.key === key)
  if (!l || !l.batchId || !(l.batchRemaining > 0) || l.qty <= l.batchRemaining) return { lines, seq, key, notice: null }
  const over = l.qty - l.batchRemaining
  const clamped = syncSerials({ ...l, qty: l.batchRemaining })
  let next = lines.map((x) => (x.key === key ? clamped : x))
  const lot = lots.find((o) => o.batchId !== l.batchId && num(o.qtyRemaining) > 0 && !next.some((x) => x.productId === l.productId && x.batchId === o.batchId))
  if (!lot) {
    return { lines: next, seq, key, notice: { tone: 'error', text: `Only ${qtyText(l.batchRemaining)} left at ${l.unitPrice.toFixed(2)}` } }
  }
  const k2 = seq + 1
  next = [...next, syncSerials({ ...l, key: k2, unitPrice: num(lot.price), qty: over, batchId: lot.batchId,
    batchRemaining: num(lot.qtyRemaining), batchLabel: lotLabel(lot), serials: [] })]
  const deeper = spill(next, k2, k2, lots)
  return {
    lines: deeper.lines, seq: deeper.seq, key,
    notice: deeper.notice ?? { tone: 'success', text: `Split: ${qtyText(clamped.qty)} at ${l.unitPrice.toFixed(2)}, ${qtyText(over)} at ${num(lot.price).toFixed(2)}` },
  }
}
```

- [ ] **Step 3: Implement `entry.ts`.** Create `web/src/features/till/cart/entry.ts`:

```ts
export type Entry =
  | { kind: 'empty' }
  | { kind: 'quick'; barcode: string; qty: number } // phone Quick-Sell rush handoff
  | { kind: 'barcode'; code: string } // digits: try an exact barcode first (the scan path)
  | { kind: 'search'; q: string }

export function parseEntry(raw: string): Entry {
  const code = raw.trim()
  if (!code) return { kind: 'empty' }
  const m = /^KQ(\d+)X(\d+)$/i.exec(code)
  if (m) return { kind: 'quick', barcode: m[1], qty: Number(m[2]) }
  if (/^\d+$/.test(code)) return { kind: 'barcode', code }
  return { kind: 'search', q: code }
}
```

- [ ] **Step 4: Implement `held.ts`.** Create `web/src/features/till/cart/held.ts`:

```ts
import type { HeldSale } from '@/api/types.gen'
import { num } from '@/lib/money'
import { MAX_STOCK } from './lines'
import type { CartLine, DiscountType, TillState } from './types'

type HeldBlob = { v: 2; lines: CartLine[]; billDiscountType: DiscountType }

/** What goes into held_sales.cart: an SPA-owned document (the server stores it opaquely). */
export const toHeldBlob = (s: TillState) =>
  JSON.stringify({ v: 2, lines: s.lines, billDiscountType: s.billDiscountType } satisfies HeldBlob)

/** Re-key every line on the way back in (keys must stay unique in this cart). */
export function fromHeld(h: HeldSale, seq: number) {
  let parsed: unknown
  try {
    parsed = JSON.parse(h.cart)
  } catch {
    parsed = []
  }
  const blob = !Array.isArray(parsed) && parsed && typeof parsed === 'object' ? (parsed as Partial<HeldBlob>) : null
  const raw: CartLine[] = Array.isArray(parsed) ? parsed.map(legacyLine) : (blob?.lines ?? [])
  const lines = raw.map((l) => ({ ...l, key: ++seq }))
  return { lines, seq, billDiscountType: (blob?.billDiscountType === 'percent' ? 'percent' : 'fixed') as DiscountType }
}

/** A karots-pos Alpine cart line (restored from a karots-pos backup). */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function legacyLine(x: any): CartLine {
  return {
    key: 0, productId: num(x.id), name: String(x.name ?? ''), unitPrice: num(x.unit_price), taxRate: num(x.tax_rate),
    qty: num(x.qty), stock: x.is_service ? MAX_STOCK : num(x.stock), isService: !!x.is_service, allowDecimal: !!x.allowDecimal,
    batchId: num(x.batch_id), batchRemaining: num(x.batch_remaining), batchLabel: String(x.batch_label ?? ''),
    allowOversell: !!x.allow_oversell, discount: String(x.discount ?? ''), discountType: x.discountType === 'percent' ? 'percent' : 'fixed',
    trackSerial: !!x.track_serial, warrantyMonths: num(x.warranty_months), serials: Array.isArray(x.serials) ? x.serials.map(String) : [],
    priceOverride: String(x.price_override ?? ''), description: String(x.description ?? ''),
    components: Array.isArray(x.components) ? x.components.map((c: { product_id: unknown; quantity: unknown }) => ({ productId: num(c.product_id), quantity: String(c.quantity) })) : [],
  }
}
```

- [ ] **Step 5: Implement `store.ts`.** Create `web/src/features/till/cart/store.ts`:

```ts
import type { HeldSale, Lot, TillProduct } from '@/api/types.gen'
import { fromHeld } from './held'
import { addProduct, serviceLine, setQty, syncSerials } from './lines'
import type { DiscountType, Payment, SaleType, ServiceLineInput, TillState } from './types'

export const blankDraft = (method: string): Payment => ({ method, amount: '', reference: '', accountId: '', meta: {} })

export const initialState = (saleType: SaleType): TillState => ({
  lines: [], seq: 0, lastAddedKey: 0, saleType, customerId: null, billDiscount: '', billDiscountType: 'fixed',
  tenders: [], draft: blankDraft('cash'), keep: '', roundToAccount: false, overLimitApproved: false,
  postSaleChange: 0, notice: null,
})

export type Action =
  | { type: 'add'; product: TillProduct; lot?: Lot; lots: Lot[] }
  | { type: 'quickSet'; product: TillProduct; qty: number }
  | { type: 'addService'; line: ServiceLineInput }
  | { type: 'setQty'; key: number; qty: number; lots: Lot[] }
  | { type: 'remove'; key: number }
  | { type: 'lineDiscount'; key: number; discount: string; discountType: DiscountType }
  | { type: 'serial'; key: number; index: number; value: string }
  | { type: 'approveOversell'; key: number }
  | { type: 'approveConsumables' }
  | { type: 'saleType'; saleType: SaleType }
  | { type: 'customer'; id: number | null }
  | { type: 'billDiscount'; value?: string; discountType?: DiscountType }
  | { type: 'draft'; patch: Partial<Payment> }
  | { type: 'commitDraft' }
  | { type: 'addTender'; payment: Payment }
  | { type: 'removeTender'; index: number }
  | { type: 'removeCreditTenders' }
  | { type: 'keep'; value: string }
  | { type: 'roundToAccount'; on: boolean }
  | { type: 'approveOverLimit' }
  | { type: 'resume'; held: HeldSale }
  | { type: 'newSale'; saleType: SaleType; change?: number }
  | { type: 'clearNotice' }
  | { type: 'restore'; state: TillState }

/** With a customer, don't pre-select cash (a blank line must route to "on account?",
 * never silently complete as paid). Only rewrites the untouched default draft. */
function syncDefaultTender(s: TillState): TillState {
  const d = s.draft
  if (s.tenders.length || d.amount !== '' || d.reference || (d.method !== '' && d.method !== 'cash')) return s
  return { ...s, draft: { ...d, method: s.customerId ? '' : 'cash' } }
}

const mapLine = (s: TillState, key: number, f: (l: TillState['lines'][number]) => TillState['lines'][number]) =>
  ({ ...s, lines: s.lines.map((l) => (l.key === key ? f(l) : l)) })

export function tillReducer(s: TillState, a: Action): TillState {
  switch (a.type) {
    case 'add': {
      const r = addProduct(s.lines, s.seq, a.product, s.saleType, a.lot, a.lots)
      return { ...s, lines: r.lines, seq: r.seq, lastAddedKey: r.key, notice: r.notice, postSaleChange: 0 }
    }
    case 'quickSet': {
      let st = s
      let hit = s.lines.find((l) => l.productId === a.product.id && !l.isService)
      if (!hit) {
        const r = addProduct(s.lines, s.seq, a.product, s.saleType, undefined, [])
        st = { ...s, lines: r.lines, seq: r.seq, lastAddedKey: r.key }
        hit = r.lines.find((l) => l.key === r.key)
      }
      const r = setQty(st.lines, st.seq, hit!.key, a.qty, [])
      return { ...st, lines: r.lines, seq: r.seq, postSaleChange: 0 }
    }
    case 'addService': {
      const key = s.seq + 1
      return { ...s, lines: [...s.lines, serviceLine(key, a.line)], seq: key, lastAddedKey: key, postSaleChange: 0 }
    }
    case 'setQty': {
      const r = setQty(s.lines, s.seq, a.key, a.qty, a.lots)
      return { ...s, lines: r.lines, seq: r.seq, notice: r.notice }
    }
    case 'remove':
      return { ...s, lines: s.lines.filter((l) => l.key !== a.key) }
    case 'lineDiscount':
      return mapLine(s, a.key, (l) => ({ ...l, discount: a.discount, discountType: a.discountType }))
    case 'serial':
      return mapLine(s, a.key, (l) => syncSerials({ ...l, serials: l.serials.map((v, i) => (i === a.index ? a.value : v)) }))
    case 'approveOversell':
      return mapLine(s, a.key, (l) => ({ ...l, allowOversell: true }))
    case 'approveConsumables':
      return { ...s, lines: s.lines.map((l) => (l.isService || l.components.length ? { ...l, allowOversell: true } : l)) }
    case 'saleType':
      return { ...s, saleType: a.saleType }
    case 'customer': {
      let st: TillState = { ...s, customerId: a.id, overLimitApproved: false }
      if (a.id === null) st = { ...st, roundToAccount: false, draft: st.draft.method === 'credit' ? { ...st.draft, method: '' } : st.draft }
      return syncDefaultTender(st)
    }
    case 'billDiscount':
      return { ...s, billDiscount: a.value ?? s.billDiscount, billDiscountType: a.discountType ?? s.billDiscountType }
    case 'draft':
      return { ...s, draft: { ...s.draft, ...a.patch } }
    case 'commitDraft':
      if (!(Number(s.draft.amount) > 0) || !s.draft.method) return s
      return { ...s, tenders: [...s.tenders, s.draft], draft: { ...blankDraft(s.draft.method), accountId: s.draft.accountId } }
    case 'addTender':
      return { ...s, tenders: [...s.tenders, a.payment] }
    case 'removeTender':
      return syncDefaultTender({ ...s, tenders: s.tenders.filter((_, i) => i !== a.index) })
    case 'removeCreditTenders':
      return { ...s, tenders: s.tenders.filter((p) => p.method !== 'credit'), draft: s.draft.method === 'credit' ? blankDraft('') : s.draft }
    case 'keep':
      return { ...s, keep: a.value }
    case 'roundToAccount':
      return { ...s, roundToAccount: a.on }
    case 'approveOverLimit':
      return { ...s, overLimitApproved: true }
    case 'resume': {
      const r = fromHeld(a.held, s.seq)
      const saleType = a.held.saleType === 'wholesale' ? 'wholesale' : 'retail'
      return syncDefaultTender({
        ...initialState(saleType), seq: r.seq, lines: r.lines, customerId: a.held.customerId ?? null,
        billDiscount: Number(a.held.discount) ? String(Number(a.held.discount)) : '', billDiscountType: r.billDiscountType,
      })
    }
    case 'newSale':
      return { ...initialState(a.saleType), seq: s.seq, postSaleChange: a.change ?? 0 }
    case 'clearNotice':
      return { ...s, notice: null }
    case 'restore':
      return a.state
  }
}
```

The `quickSet` branch uses `hit!` after it guarantees a line exists. If the linter objects, replace it with an explicit guard.

- [ ] **Step 6: Implement `persist.ts`.** Create `web/src/features/till/cart/persist.ts`:

```ts
import type { TillState } from './types'

/** Per-user key: an admin unlocking someone else's lock never sees their cart. */
export const cartKey = (userId: number) => `ks-till-cart-${userId}`

/** The in-progress sale survives auto-lock, reload and tab switches (sessionStorage:
 * it dies with the kiosk browser, which is the end of that shift anyway). */
export function loadCart(userId: number): TillState | null {
  try {
    const raw = sessionStorage.getItem(cartKey(userId))
    if (!raw) return null
    const s = JSON.parse(raw) as TillState
    if (!s || !Array.isArray(s.lines) || !s.draft) return null
    return { ...s, notice: null }
  } catch {
    return null
  }
}

export function saveCart(userId: number, s: TillState) {
  try {
    sessionStorage.setItem(cartKey(userId), JSON.stringify({ ...s, notice: null }))
  } catch {
    /* storage full/blocked: the till still works, it just can't recover */
  }
}
```

- [ ] **Step 7: Run the tests.**

Run: `cd web && bunx vitest run src/features/till && bunx tsc -b`
Expected: PASS.

- [ ] **Step 8: Commit.**

```bash
git add web/src/features/till/cart
git commit -m "feat(web): till cart core — lots/FEFO/spill, entry parsing (KQ), held blobs, reducer, per-user persistence"
```

---

### Task 7: Checkout gate, sale request builder and hotkeys (pure TS)

**Files:**
- Create in `web/src/features/till/`:
  - `cart/checkout.ts`, `cart/checkout.test.ts`
  - `hotkeys.ts`, `hotkeys.test.ts`

**Interfaces:**
- Consumes: `totals.ts`, `types.ts`, and the generated `TillCustomer` and `SaleRequest`.
- Produces:
  - `Prompt`, `GateCtx`, `Gate`
  - `effectivePayments(s, total): Payment[]`
  - `checkoutGate(s, ctx): Gate`
  - `buildSaleRequest(s, payments, { deferDrawerKick }): SaleRequest`
  - `hotkeyFor(key, ctx): HotkeyAction | null`

- [ ] **Step 1: Write the failing tests.** `web/src/features/till/cart/checkout.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import type { TillCustomer } from '@/api/types.gen'
import { buildSaleRequest, checkoutGate, effectivePayments, type GateCtx } from './checkout'
import { initialState, tillReducer } from './store'
import type { TillState } from './types'

const withLine = (s: TillState, over: Partial<TillState['lines'][number]> = {}) => {
  const t = tillReducer(s, { type: 'addService', line: { productId: 1, name: 'Item', price: 100 } })
  return { ...t, lines: t.lines.map((l) => ({ ...l, isService: false, stock: 10, priceOverride: '', ...over })) }
}
const cust: TillCustomer = { id: 5, name: 'Nimal', phone: '077', creditLimit: '1000.00', outstanding: '950.00' }
const ctx = (p: Partial<GateCtx> = {}): GateCtx => ({ customer: null, canManageCredit: false, accountKinds: { card: false, online: false }, confirmed: false, ...p })

describe('checkoutGate', () => {
  it('walk-in with a blank cash draft = exact cash', () => {
    const s = withLine(initialState('retail'))
    const g = checkoutGate(s, ctx())
    expect('ok' in g && g.payments).toEqual([expect.objectContaining({ method: 'cash', amount: '100.00' })])
  })
  it('customer + blank tender never completes as cash: it asks to put it on account', () => {
    const s = withLine(tillReducer(initialState('retail'), { type: 'customer', id: 5 }))
    expect(checkoutGate(s, ctx({ customer: cust }))).toEqual({ prompt: { kind: 'shortfall', amount: 100, hasCustomer: true } })
  })
  it('found-at-till is asked first, one line at a time', () => {
    const s = withLine(initialState('retail'), { qty: 11 })
    expect(checkoutGate(s, ctx())).toMatchObject({ prompt: { kind: 'oversell', qty: 11, stock: 10 } })
  })
  it('over-limit is offered only to users who may approve it', () => {
    let s = withLine(tillReducer(initialState('retail'), { type: 'customer', id: 5 }))
    s = tillReducer(s, { type: 'addTender', payment: { method: 'credit', amount: '100', reference: '', accountId: '', meta: {} } })
    expect(checkoutGate(s, ctx({ customer: cust, canManageCredit: true }))).toEqual({ prompt: { kind: 'overlimit', amount: 100, over: 50 } })
    expect('ok' in checkoutGate(s, ctx({ customer: cust }))).toBe(true) // server is the backstop
  })
  it('a confirmed shortfall passes the tender check', () => {
    const s = withLine(tillReducer(initialState('retail'), { type: 'draft', patch: { amount: '40' } }))
    expect('prompt' in checkoutGate(s, ctx())).toBe(true)
    expect('ok' in checkoutGate(s, ctx({ confirmed: true }))).toBe(true)
  })
  it('card needs its account when one is set up; serials must be complete', () => {
    let s = withLine(initialState('retail'))
    s = tillReducer(s, { type: 'draft', patch: { method: 'card', amount: '100' } })
    expect(checkoutGate(s, ctx({ accountKinds: { card: true, online: false } }))).toEqual({ error: 'Choose the account for the card payment' })
    const t = withLine(initialState('retail'), { trackSerial: true, serials: [''] })
    expect(checkoutGate(t, ctx())).toEqual({ error: 'Enter 1 serial number(s) for Item' })
  })
  it('an amount with no method is refused', () => {
    const s = withLine(tillReducer(initialState('retail'), { type: 'draft', patch: { method: '', amount: '100' } }))
    expect(checkoutGate(s, ctx())).toEqual({ error: 'Choose a payment method for each amount' })
  })
})

describe('buildSaleRequest', () => {
  it('sends the kept change only when the shop keeps something', () => {
    let s = withLine(initialState('retail'))
    s = tillReducer(s, { type: 'addTender', payment: { method: 'cash', amount: '150', reference: '', accountId: '', meta: {} } })
    const pays = effectivePayments(s, 100)
    expect(buildSaleRequest(s, pays, { deferDrawerKick: true }).changeGiven).toBe('')
    s = tillReducer(s, { type: 'keep', value: '5' })
    const req = buildSaleRequest(s, pays, { deferDrawerKick: true })
    expect(req).toMatchObject({ changeGiven: '45.00', deferDrawerKick: true, onAccountRounding: false })
    expect(req.items[0]).toMatchObject({ productId: 1, quantity: '1', batchId: 0 })
  })
})
```

`web/src/features/till/hotkeys.test.ts`:

```ts
import { describe, expect, it } from 'vitest'
import { hotkeyFor } from './hotkeys'

describe('till hotkeys', () => {
  const idle = { dialogOpen: false, saleDone: false }
  it('maps the karots-pos keys', () => {
    expect(hotkeyFor('F1', idle)).toBe('reset')
    expect(hotkeyFor('F2', idle)).toBe('search')
    expect(hotkeyFor('F3', idle)).toBe('search')
    expect(hotkeyFor('F4', idle)).toBe('hold')
    expect(hotkeyFor('F9', idle)).toBe('discount')
    expect(hotkeyFor('F10', idle)).toBe('pay')
    expect(hotkeyFor('Escape', idle)).toBe('blur')
    expect(hotkeyFor('Enter', idle)).toBeNull()
  })
  it('hold/pay/escape defer to an open dialog; Enter starts the next sale after completion', () => {
    const dlg = { dialogOpen: true, saleDone: false }
    expect(hotkeyFor('F4', dlg)).toBeNull()
    expect(hotkeyFor('F10', dlg)).toBeNull()
    expect(hotkeyFor('Escape', dlg)).toBeNull()
    expect(hotkeyFor('Enter', { dialogOpen: false, saleDone: true })).toBe('newSale')
  })
})
```

Run: `cd web && bunx vitest run src/features/till/cart/checkout.test.ts src/features/till/hotkeys.test.ts`
Expected: FAIL (modules not found).

- [ ] **Step 2: Implement `checkout.ts`.** Create `web/src/features/till/cart/checkout.ts`:

```ts
import type { SaleRequest, TillCustomer } from '@/api/types.gen'
import { dec, num, r2 } from '@/lib/money'
import { accountTotal, actualChange, changeDue, keptAmount, paidTotal, roundable, saleTotal, tenderIssue } from './totals'
import type { Payment, TillState } from './types'

export type Prompt =
  | { kind: 'oversell'; key: number; name: string; stock: number; qty: number }
  | { kind: 'shortfall'; amount: number; hasCustomer: boolean }
  | { kind: 'overcovered'; amount: number; hasCustomer: boolean }
  | { kind: 'overlimit'; amount: number; over: number }

export type GateCtx = {
  customer: TillCustomer | null
  canManageCredit: boolean
  accountKinds: { card: boolean; online: boolean } // a card / bank locker is set up
  confirmed: boolean // the tender prompt already answered (cannot loop)
}

export type Gate = { prompt: Prompt } | { error: string } | { ok: true; payments: Payment[] }

/** Committed tenders + the draft line. A walk-in's untouched cash draft means
 * "exact cash" so the cashier can just hit Complete. */
export function effectivePayments(s: TillState, due: number): Payment[] {
  const ps = [...s.tenders]
  if (num(s.draft.amount) > 0 || (s.draft.amount !== '' && !s.draft.method)) ps.push(s.draft)
  if (ps.length === 0 && s.draft.method === 'cash' && due > 0) ps.push({ ...s.draft, amount: dec(due) })
  return ps
}

/** Everything that must be true before posting, in karots-pos's order. */
export function checkoutGate(s: TillState, ctx: GateCtx): Gate {
  if (!s.lines.length) return { error: 'Cart is empty' }
  const t = saleTotal(s)
  const payments = effectivePayments(s, t)
  const short = s.lines.find((l) => !l.allowOversell && l.qty > l.stock + 1e-9)
  if (short) return { prompt: { kind: 'oversell', key: short.key, name: short.name, stock: short.stock, qty: short.qty } }
  if (!ctx.confirmed) {
    const issue = tenderIssue(t, paidTotal(payments), accountTotal(payments), s.customerId !== null)
    if (issue) return { prompt: issue }
  }
  if (!s.overLimitApproved && ctx.customer && ctx.canManageCredit) {
    const onAcct = accountTotal(payments)
    const avail = num(ctx.customer.creditLimit) - num(ctx.customer.outstanding)
    if (onAcct > 0.005 && onAcct > avail + 0.005) return { prompt: { kind: 'overlimit', amount: r2(onAcct), over: r2(onAcct - avail) } }
  }
  for (const l of s.lines) {
    if (!l.trackSerial) continue
    const n = Math.max(0, Math.round(l.qty))
    if (l.serials.filter((x) => x.trim() !== '').length < n) return { error: `Enter ${n} serial number(s) for ${l.name}` }
  }
  if (payments.some((p) => num(p.amount) > 0 && !p.method)) return { error: 'Choose a payment method for each amount' }
  for (const p of payments) {
    if ((p.method === 'card' || p.method === 'online') && num(p.amount) > 0 && ctx.accountKinds[p.method] && !p.accountId) {
      return { error: `Choose the account for the ${p.method} payment` }
    }
  }
  return { ok: true, payments }
}

export function buildSaleRequest(s: TillState, payments: Payment[], opts: { deferDrawerKick: boolean }): SaleRequest {
  const t = saleTotal(s)
  const change = changeDue(t, payments)
  const kept = roundable(t, payments) ? keptAmount(s.keep, change) : 0
  return {
    saleType: s.saleType,
    customerId: s.customerId ?? undefined,
    discount: String(num(s.billDiscount)),
    discountType: s.billDiscountType,
    allowOverLimit: s.overLimitApproved,
    deferDrawerKick: opts.deferDrawerKick,
    changeGiven: kept > 0.005 ? dec(actualChange(change, kept)) : '',
    onAccountRounding: kept > 0.005 && s.customerId !== null && s.roundToAccount,
    items: s.lines.map((l) => ({
      productId: l.productId, quantity: String(l.qty), discount: String(num(l.discount)), discountType: l.discountType,
      priceOverride: l.priceOverride, batchId: l.batchId, allowOversell: l.allowOversell,
      serials: l.trackSerial ? l.serials.map((x) => x.trim()) : [], description: l.description, components: l.components,
    })),
    payments: payments.filter((p) => num(p.amount) > 0).map((p) => ({
      method: p.method, amount: String(num(p.amount)), reference: p.reference, accountId: num(p.accountId),
      meta: Object.keys(p.meta).length ? p.meta : undefined,
    })),
  }
}
```

- [ ] **Step 3: Implement `hotkeys.ts`.** Create `web/src/features/till/hotkeys.ts`:

```ts
export type HotkeyAction = 'reset' | 'search' | 'hold' | 'discount' | 'pay' | 'blur' | 'newSale'

/** F1 ⟲ menu (search only — never the cart) · F2/F3 search · F4 hold · F9 bill
 * discount · F10 amount (then Enter completes) · Esc blur · Enter after a sale = next. */
export function hotkeyFor(key: string, ctx: { dialogOpen: boolean; saleDone: boolean }): HotkeyAction | null {
  switch (key) {
    case 'F1':
      return 'reset'
    case 'F2':
    case 'F3':
      return 'search'
    case 'F4':
      return ctx.dialogOpen ? null : 'hold'
    case 'F9':
      return 'discount'
    case 'F10':
      return ctx.dialogOpen ? null : 'pay'
    case 'Escape':
      return ctx.dialogOpen ? null : 'blur' // Radix closes its own dialog
    case 'Enter':
      return ctx.saleDone ? 'newSale' : null
  }
  return null
}
```

- [ ] **Step 4: Run the tests.**

Run: `cd web && bunx vitest run && bunx tsc -b`
Expected: every web test passes.

- [ ] **Step 5: Commit.**

```bash
git add web/src/features/till
git commit -m "feat(web): till checkout gate (oversell → tender → over-limit → serials → method/account), sale request builder, hotkey map"
```

---

### Task 8: Till data layer, slots, providers, top bar and the screenshot tool

**Files:**
- Create:
  - `web/src/features/till/api.ts`
  - `web/src/features/till/print.ts`
  - `web/src/features/till/ui.ts`
  - `web/src/features/till/bits.tsx`
  - `web/src/features/till/TillProvider.tsx`
  - `web/src/features/till/TillUi.tsx`
  - `web/src/features/till/TillScreen.tsx`
  - `web/src/plugins/slots.ts`
  - `web/src/lib/useDebounced.ts`
  - `web/e2e/till-shots.spec.ts`
- Modify:
  - `web/src/routes/_authed/till.tsx`
  - `web/src/routes/_authed/till/index.tsx`
  - `web/src/theme/TillShell.tsx`
  - `web/src/theme/AdminShell.tsx` (the `TILL_OPEN` bounce)
  - `web/package.json` (`motion`)
  - `web/.gitignore` (`shots`)

**Interfaces:**
- Consumes: the generated DTO types, `money.ts`, and the `cart/*` modules.
- Produces, all used by Tasks 9–13:
  - **Queries:**
    - `configQuery`, `drawerQuery`, `groupsQuery`, `groupQuery(id)`, `searchQuery(q)`
    - `priceOptionsQuery`, `quickPicksQuery`, `unitsQuery`, `customersQuery`, `heldQuery`
  - **Helpers:** `lockersOf(state, kind)`, `useRefreshTill()`, `errMsg(e, fallback?)`
  - **Printing:** `printSale(id, kick?)`, `printMoneySlip(id, kick?)`
  - **Class strings:** `PANEL`, `CHIP`, `FIELD`, `TILE`
  - **Components:** `Seg`, `Kbd`
  - **State:** `TillProvider`, `useTill()`
  - **UI context:** `TillUiProvider`, `useTillUi()`, `useTillApi()`, the type `Dialog`
  - **Slots:** `slots` and the types `TillApi`, `TillMenuRoot`, `TillAction`, `DrawerSection`, `TillTender`, `SaleHookCtx`
  - **Screen:** `TillScreen`
  - **Lib:** `useDebounced`

- [ ] **Step 1: Skills and dependency.**
  - In `web/`, run `intent list`, then `intent load @tanstack/react-router#<router skill>` and `intent load @tanstack/react-query#<query skill>`, using the names `intent list` printed.
  - Then run `bun add motion`.
  - Append `shots` to `web/.gitignore`.

- [ ] **Step 2: Data layer.** Create `web/src/features/till/api.ts`:

```ts
import { queryOptions, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import type {
  DrawerState, GroupView, HeldSale, Lot, ProductPage, TillConfig, TillCustomer, TillGroup, TillProduct, Unit,
} from '@/api/types.gen'

export const configQuery = queryOptions({ queryKey: ['till', 'config'], queryFn: () => api<TillConfig>('/till/config'), staleTime: Infinity })
export const drawerQuery = queryOptions({ queryKey: ['till', 'drawer'], queryFn: () => api<DrawerState>('/till/drawer') })
export const groupsQuery = queryOptions({ queryKey: ['till', 'groups'], queryFn: () => api<TillGroup[]>('/till/groups'), staleTime: 60_000 })
export const groupQuery = (id: number) =>
  queryOptions({ queryKey: ['till', 'group', id], queryFn: () => api<GroupView>(`/till/groups/${id}`) })
export const searchQuery = (q: string) =>
  queryOptions({
    queryKey: ['till', 'search', q],
    queryFn: ({ signal }) => api<ProductPage>(`/till/products?search=${encodeURIComponent(q)}`, { signal }),
    enabled: q.trim() !== '',
  })
/** Which products ask "which price?". Best-effort, refreshed every 90s and on focus
 * (another lane receiving stock changes the answer mid-shift). */
export const priceOptionsQuery = queryOptions({
  queryKey: ['till', 'price-options'],
  queryFn: () => api<Record<string, Lot[]>>('/till/price-options'),
  refetchInterval: 90_000, refetchOnWindowFocus: true, retry: false,
})
export const quickPicksQuery = queryOptions({ queryKey: ['till', 'quick-picks'], queryFn: () => api<TillProduct[]>('/till/quick-picks') })
export const unitsQuery = queryOptions({ queryKey: ['till', 'units'], queryFn: () => api<Unit[]>('/till/units'), staleTime: Infinity })
export const customersQuery = queryOptions({ queryKey: ['till', 'customers'], queryFn: () => api<TillCustomer[]>('/till/customers') })
export const heldQuery = queryOptions({ queryKey: ['till', 'held'], queryFn: () => api<HeldSale[]>('/till/held') })

/** Card tenders land in 'card' lockers, online ones in 'bank' lockers. */
export const lockersOf = (d: DrawerState | undefined, kind: 'card' | 'bank') => (d?.lockers ?? []).filter((l) => l.kind === kind)

/** Everything the till caches except its config: refetch after a sale or stock change. */
export function useRefreshTill() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ predicate: (q) => q.queryKey[0] === 'till' && q.queryKey[1] !== 'config' })
}

export const errMsg = (e: unknown, fallback = 'Something went wrong') => (e instanceof ApiError ? e.message : fallback)
```

Create `web/src/features/till/print.ts`:

```ts
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'

const FAIL = "Receipt didn't print — check the printer is on and connected"

async function send(path: string) {
  try {
    await api(path, { method: 'POST' })
    toast.success('Receipt sent to printer')
  } catch (e) {
    toast.error(e instanceof ApiError && e.code === 'NO_PRINTER' ? e.message : FAIL)
  }
}

/** Fire-and-forget: the sale is saved; only the paper is at stake. kick folds the
 * drawer pulse into this ONE printer job (fresh cash sale in auto-print mode). */
export const printSale = (id: number, kick = false) => send(`/till/sales/${id}/print${kick ? '?kick=1' : ''}`)
export const printMoneySlip = (id: number, kick = false) => send(`/till/money-receipts/${id}/print${kick ? '?kick=1' : ''}`)
```

Create `web/src/lib/useDebounced.ts`:

```ts
import { useEffect, useState } from 'react'

export function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}
```

- [ ] **Step 3: Shared looks.** Create `web/src/features/till/ui.ts`:

```ts
/** The three big till surfaces — the only place --glass is allowed (theme.md §2). */
export const PANEL = 'rounded-[var(--radius)] border border-border bg-card shadow-[var(--shadow)] [backdrop-filter:var(--glass)]'
/** Pill buttons: group chips, quick cash, ⟲ Menu. aria-pressed shows the selected look. */
export const CHIP =
  'inline-flex items-center gap-1.5 whitespace-nowrap rounded-full border border-border bg-card px-3 py-1.5 text-[13px] font-semibold aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'
/** Text inputs on the till. */
export const FIELD =
  'h-[var(--control-h)] rounded-[var(--radius-sm)] border border-border bg-muted px-3 text-[15px] text-foreground outline-none placeholder:text-faint focus-visible:ring-2 focus-visible:ring-ring'
/** Product and group cards: no glass (GPU), lift on hover via motion. */
export const TILE =
  'flex min-h-[98px] w-full flex-col justify-between gap-2 rounded-[var(--radius-sm)] border border-border bg-card p-3 text-left shadow-[var(--shadow)]'
```

Create `web/src/features/till/bits.tsx`:

```tsx
import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'

export function Kbd({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <kbd className={cn('rounded-md border border-border bg-muted px-1.5 py-0.5 font-sans text-[11px] font-semibold text-muted-foreground', className)}>
      {children}
    </kbd>
  )
}

/** Two-to-four option segmented control (Retail/Wholesale, Rs/%). */
export function Seg<T extends string>({ label, value, options, onChange }: {
  label: string
  value: T
  options: [T, string][]
  onChange: (v: T) => void
}) {
  return (
    <div role="group" aria-label={label} className="inline-flex rounded-full border border-border bg-muted p-0.5">
      {options.map(([v, text]) => (
        <button key={v} type="button" aria-pressed={value === v} onClick={() => onChange(v)}
          className="rounded-full px-2.5 py-1 text-[12px] font-bold text-muted-foreground aria-pressed:bg-selected aria-pressed:text-selected-foreground">
          {text}
        </button>
      ))}
    </div>
  )
}
```

- [ ] **Step 4: Plugin slots.** Create `web/src/plugins/slots.ts`:

```ts
import type { ComponentType } from 'react'
import type { CartLine, ServiceLineInput } from '@/features/till/cart/types'

/** What a plugin till component may do to the sale in progress. */
export type TillApi = {
  addServiceLine(line: ServiceLineInput): void
  refreshDrawer(): void
}

/** A chip beside the group chips that opens a plugin menu in the catalogue area. */
export type TillMenuRoot = { key: string; label: string; emoji?: string; Panel: ComponentType<{ till: TillApi; onDone: () => void }> }

/** A button in the till's action row (beside Deposit / Withdraw) opening a panel. */
export type TillAction = { key: string; label: string; Panel: ComponentType<{ till: TillApi; onClose: () => void }> }

/** Extra counts in the Open/Close till dialogs (side by side with the cash count).
 * register(save) on mount; save runs around the drawer call — returning false
 * aborts a close (an open is best-effort). */
export type DrawerSection = {
  key: string
  Section: ComponentType<{ which: 'open' | 'close'; register: (save: () => Promise<boolean>) => void }>
}

/** Detail picker for a plugin tender method listed in TillConfig.tenderMethods. */
export type TillTender = {
  value: string
  Detail: ComponentType<{ meta: Record<string, string>; setMeta: (m: Record<string, string>) => void }>
}

export type SaleHookCtx = { saleId: number; lines: CartLine[] }

/** Filled by enabled plugins at the composition root (M7). Core renders whatever is here. */
export const slots = {
  tillMenu: [] as TillMenuRoot[],
  tillActions: [] as TillAction[],
  drawerSections: [] as DrawerSection[],
  tillTenders: [] as TillTender[],
  /** Return an error message to block checkout, or null. */
  tillBeforeCheckout: [] as ((lines: CartLine[]) => Promise<string | null>)[],
  /** Best-effort bookkeeping after a sale commits; a failure never undoes the sale. */
  tillAfterSale: [] as ((ctx: SaleHookCtx) => Promise<void>)[],
}
```

- [ ] **Step 5: State providers.** Create `web/src/features/till/TillProvider.tsx`:

```tsx
import { createContext, useContext, useEffect, useReducer, type Dispatch, type ReactNode } from 'react'
import { toast } from 'sonner'
import { loadCart, saveCart } from './cart/persist'
import { initialState, tillReducer, type Action } from './cart/store'
import type { SaleType, TillState } from './cart/types'

type Ctx = { state: TillState; dispatch: Dispatch<Action> }
const TillCtx = createContext<Ctx | null>(null)

/** The sale in progress. Lives at the till layout so M1b's till tabs keep it, and is
 * saved per user so auto-lock / reload bring it back. */
export function TillProvider({ userId, saleType, children }: { userId: number; saleType: SaleType; children: ReactNode }) {
  const [state, dispatch] = useReducer(tillReducer, undefined, () => loadCart(userId) ?? initialState(saleType))
  useEffect(() => saveCart(userId, state), [userId, state])
  useEffect(() => {
    if (!state.notice) return
    ;(state.notice.tone === 'error' ? toast.error : toast.success)(state.notice.text)
    dispatch({ type: 'clearNotice' })
  }, [state.notice])
  return <TillCtx.Provider value={{ state, dispatch }}>{children}</TillCtx.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTill() {
  const v = useContext(TillCtx)
  if (!v) throw new Error('useTill must be used inside <TillProvider>')
  return v
}
```

Create `web/src/features/till/TillUi.tsx`:

```tsx
import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { createContext, useContext, useMemo, useState, type ReactNode } from 'react'
import type { Lot, TillGroup, TillProduct } from '@/api/types.gen'
import type { TillApi } from '@/plugins/slots'
import { configQuery, drawerQuery } from './api'
import { printMoneySlip } from './print'
import { useTill } from './TillProvider'

/** One till dialog at a time (karots-pos anyModalOpen). */
export type Dialog =
  | { kind: 'withdraw' }
  | { kind: 'deposit' }
  | { kind: 'close'; logout?: boolean }
  | { kind: 'slip'; receiptId: number }
  | { kind: 'held' }
  | { kind: 'clearSale' }
  | { kind: 'quickItem'; barcode: string; name: string }
  | { kind: 'addCustomer' }
  | { kind: 'info'; product: TillProduct }
  | { kind: 'pricePick'; product: TillProduct; lots: Lot[] }
  | { kind: 'suggestion'; product: TillProduct }
  | { kind: 'action'; key: string }

type Ui = {
  dialog: Dialog | null
  open: (d: Dialog) => void
  close: () => void
  search: string
  setSearch: (s: string) => void
  stack: TillGroup[] // group drill-down breadcrumb; [] = top menu
  setStack: (s: TillGroup[]) => void
  menu: string | null // open plugin menu root (slots.tillMenu key)
  setMenu: (k: string | null) => void
  /** F1 / ⟲ Menu: back to the top menu and refocus the scanner. NEVER touches the cart. */
  reset: () => void
  /** After a till cash move: prompt to print the CR- slip, or auto-print it. */
  afterMove: (receiptId: number | undefined, mergeKick?: boolean) => void
}

const UiCtx = createContext<Ui | null>(null)

export function TillUiProvider({ children }: { children: ReactNode }) {
  const cfg = useSuspenseQuery(configQuery).data
  const [dialog, setDialog] = useState<Dialog | null>(null)
  const [search, setSearch] = useState('')
  const [stack, setStack] = useState<TillGroup[]>([])
  const [menu, setMenu] = useState<string | null>(null)
  const value = useMemo<Ui>(() => ({
    dialog, open: setDialog, close: () => setDialog(null),
    search, setSearch, stack, setStack, menu, setMenu,
    reset: () => {
      setSearch('')
      setStack([])
      setMenu(null)
      document.getElementById('till-search')?.focus()
    },
    afterMove: (id, mergeKick = false) => {
      if (!id) return
      if (cfg.askToPrint) setDialog({ kind: 'slip', receiptId: id })
      else void printMoneySlip(id, mergeKick)
    },
  }), [dialog, search, stack, menu, cfg.askToPrint])
  return <UiCtx.Provider value={value}>{children}</UiCtx.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useTillUi() {
  const v = useContext(UiCtx)
  if (!v) throw new Error('useTillUi must be used inside <TillUiProvider>')
  return v
}

/** The narrow surface plugin till components get (slots.ts). */
// eslint-disable-next-line react-refresh/only-export-components
export function useTillApi(): TillApi {
  const { dispatch } = useTill()
  const qc = useQueryClient()
  return useMemo(() => ({
    addServiceLine: (line) => dispatch({ type: 'addService', line }),
    refreshDrawer: () => void qc.invalidateQueries({ queryKey: drawerQuery.queryKey }),
  }), [dispatch, qc])
}
```

- [ ] **Step 6: Screen skeleton and routes.** Create `web/src/features/till/TillScreen.tsx`:

```tsx
import { MotionConfig } from 'motion/react'
import { cn } from '@/lib/utils'
import { TillUiProvider } from './TillUi'
import { PANEL } from './ui'

/** The sell screen (theme.md §7): catalogue | cart | pay at ≥1200px; below that the
 * cart and pay stack in a 360px right column. Sections are filled by Tasks 9–13. */
export function TillScreen() {
  return (
    <MotionConfig reducedMotion="user">
      <TillUiProvider>
        <div className="relative grid h-full min-h-0 grid-cols-[1fr_360px] grid-rows-[minmax(0,1fr)_auto] gap-[14px] p-[14px] [grid-template-areas:'cat_cart'_'cat_pay'] min-[1200px]:grid-cols-[1fr_392px_356px] min-[1200px]:grid-rows-[minmax(0,1fr)] min-[1200px]:[grid-template-areas:'cat_cart_pay']">
          <section aria-label="Catalogue" className="min-h-0 [grid-area:cat]" />
          <section aria-label="Cart" className={cn(PANEL, 'min-h-0 [grid-area:cart]')} />
          <section aria-label="Payment" className={cn(PANEL, 'min-h-0 [grid-area:pay]')} />
        </div>
      </TillUiProvider>
    </MotionConfig>
  )
}
```

Replace `web/src/routes/_authed/till.tsx`:

```tsx
import { createFileRoute, Outlet } from '@tanstack/react-router'
import { useEffect } from 'react'
import { startAutofit } from '@/lib/autofit'
import { configQuery } from '@/features/till/api'
import { TillProvider } from '@/features/till/TillProvider'
import { TillShell } from '@/theme/TillShell'

export const Route = createFileRoute('/_authed/till')({
  loader: ({ context }) => context.queryClient.ensureQueryData(configQuery),
  component: TillLayout,
})

function TillLayout() {
  const { me } = Route.useRouteContext()
  const cfg = Route.useLoaderData()
  useEffect(() => startAutofit(), [])
  return (
    <TillProvider userId={me.user.id} saleType={cfg.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail'}>
      <TillShell me={me} config={cfg}>
        <Outlet />
      </TillShell>
    </TillProvider>
  )
}
```

Replace `web/src/routes/_authed/till/index.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router'
import { TillScreen } from '@/features/till/TillScreen'

export const Route = createFileRoute('/_authed/till/')({
  // ?logout=1: sign-out was refused because the till is open — go straight to the count.
  validateSearch: (s: Record<string, unknown>): { logout?: boolean } =>
    s.logout === true || s.logout === '1' || s.logout === 1 ? { logout: true } : {},
  component: TillScreen,
})
```

If the router plugin's generated `routeTree.gen.ts` doesn't expose `context.queryClient` in `loader`, mirror how `_authed.tsx` reads it in `beforeLoad`. Don't invent a new context field.

- [ ] **Step 7: Top bar.** Replace `web/src/theme/TillShell.tsx`:

```tsx
import { Link, useNavigate } from '@tanstack/react-router'
import { KeyRound, LayoutDashboard, LockIcon, LogOut } from 'lucide-react'
import type { ReactNode } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { Me, TillConfig } from '@/api/types.gen'
import { Kbd } from '@/features/till/bits'
import { ModeToggle } from './ModeToggle'

const TOP_BTN = 'flex h-9 items-center gap-1.5 rounded-[var(--radius-sm)] px-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground'

function handleLock() {
  api('/auth/lock', { method: 'POST' }).finally(() => window.location.assign('/lock'))
}

/** Desktop/kiosk till frame (theme.md §7): 60px top bar + autofit body. `status` is
 * the drawer pill. Tabs: Sell now; M1b adds Returns, Credit, Receipts, … */
export function TillShell({ me, config, status, children }: { me: Me; config: TillConfig; status?: ReactNode; children: ReactNode }) {
  const navigate = useNavigate()
  function signOut() {
    api('/auth/logout', { method: 'POST' })
      .then(() => window.location.assign('/login'))
      .catch((err) => {
        if (err instanceof ApiError && err.code === 'TILL_OPEN') {
          toast.error(err.message)
          void navigate({ to: '/till', search: { logout: true } })
        } else window.location.assign('/login')
      })
  }
  return (
    <div className="flex h-screen min-w-[1024px] flex-col text-foreground">
      <header className="flex h-[60px] shrink-0 items-center gap-4 border-b border-border bg-card px-[18px] [backdrop-filter:var(--glass)]">
        <span className="flex items-center gap-2 font-display text-[17px] font-bold">
          <span className="grid h-[26px] w-[26px] place-items-center rounded-lg bg-brand text-sm font-extrabold text-brand-foreground shadow-[var(--glow)]">K</span>
          {me.shopName}
        </span>
        <span className="text-[13px] text-muted-foreground">{me.user.name}</span>
        {status}
        <nav aria-label="Till sections" className="flex items-center gap-1 text-[13px] font-semibold">
          <Link to="/till" activeOptions={{ exact: true, includeSearch: false }}
            className="rounded-full px-3 py-1.5 text-muted-foreground hover:bg-muted data-[status=active]:bg-selected data-[status=active]:text-selected-foreground">
            Sell
          </Link>
        </nav>
        <div className="ml-auto hidden items-center gap-2.5 text-[12px] text-muted-foreground min-[1200px]:flex" aria-label="Keyboard shortcuts">
          <Kbd>F1</Kbd>Menu <Kbd>F2</Kbd>Search <Kbd>F4</Kbd>Hold <Kbd>F9</Kbd>Discount <Kbd>F10</Kbd>Pay
        </div>
        <div className="ml-auto flex items-center gap-1 min-[1200px]:ml-0">
          <ModeToggle />
          {config.isManager && <Link to="/admin" className={TOP_BTN}><LayoutDashboard size={16} /> Admin</Link>}
          {config.canChangePin && <Link to="/account/pin" className={TOP_BTN}><KeyRound size={16} /> Change PIN</Link>}
          <button type="button" onClick={handleLock} className="flex h-9 items-center gap-1.5 rounded-[var(--radius-sm)] border border-border bg-muted px-3 text-[13px] font-semibold">
            <LockIcon size={16} /> Lock
          </button>
          <button type="button" onClick={signOut} className={TOP_BTN}><LogOut size={16} /> Sign out</button>
        </div>
      </header>
      <div className="relative flex-1 overflow-hidden">{children}</div>
    </div>
  )
}
```

In `web/src/theme/AdminShell.tsx`, find the `TILL_OPEN` branch (`toast.error(err.message)`) and add `window.location.assign('/till?logout=1')` after the toast. karots-pos bounced a blocked logout to the till's close screen, and so does this.

- [ ] **Step 8: Screenshot tool for UI gates.** Create `web/e2e/till-shots.spec.ts`:

```ts
import { expect, test, type Page } from '@playwright/test'

// UI-gate screenshots (not a behaviour test). Needs `make e2e-seed` data.
//   SHOTS=gate1 SHOTS_SCENE=closed bunx playwright test e2e/till-shots.spec.ts
// Scenes: closed (open-till overlay) · sale (mid-sale: lines, lot, customer, tender) · done (sale complete).
// Output: web/shots/<gate>/<scene>-<palette>-<mode>-<width>.png
const gate = process.env.SHOTS
const scene = process.env.SHOTS_SCENE ?? 'sale'
test.skip(!gate, 'set SHOTS=<gate> to take UI-gate screenshots')
test.describe.configure({ mode: 'serial' })

async function login(page: Page, phone: string, pin: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill(pin)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(admin|till)/)
}

async function setPalette(page: Page, palette: string) {
  await login(page, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
  const cur = await (await page.request.get('/api/v1/system/appearance')).json()
  expect((await page.request.put('/api/v1/system/appearance', { data: { ...cur, palette } })).ok()).toBe(true)
  await page.context().clearCookies()
}

async function setScene(page: Page) {
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  const open = !!st.summary.session
  if (scene === 'closed') {
    if (open) await page.request.post('/api/v1/till/drawer/close', { data: { breakdown: {}, destLockerId: 0 } })
    await page.reload()
    return
  }
  if (!open) {
    await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
    await page.reload()
  }
  const s = page.getByLabel('Scan a barcode or search products')
  for (const code of ['4790000000011', '4790000000011']) {
    await s.fill(code)
    await s.press('Enter')
  }
  await s.fill('4790000000028')
  await s.press('Enter')
  await page.getByRole('dialog', { name: 'Which price?' }).getByRole('button', { name: /120\.00/ }).click()
  const amount = page.getByLabel('Amount tendered')
  if (scene === 'done') {
    await amount.fill('1000')
    await amount.press('Enter')
    await amount.press('Enter')
    await page.getByRole('dialog', { name: /complete/ }).waitFor()
    return
  }
  await page.getByRole('button', { name: /Walk-in customer/ }).click()
  await page.getByLabel('Search customers').fill('E2E Credit')
  await page.getByLabel('Search customers').press('Enter')
  await page.getByRole('button', { name: 'Cash', exact: true }).click()
  await amount.fill('500')
}

for (const palette of ['studio', 'night']) {
  for (const mode of ['light', 'dark']) {
    for (const width of [1366, 1024]) {
      test(`${palette} ${mode} ${width}`, async ({ browser }) => {
        const ctx = await browser.newContext({ viewport: { width, height: 768 } })
        const page = await ctx.newPage()
        await setPalette(page, palette)
        await page.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
        await login(page, '0770000002', '1234')
        await page.goto('/till')
        await page.waitForLoadState('networkidle')
        await setScene(page)
        await page.screenshot({ path: `shots/${gate}/${scene}-${palette}-${mode}-${width}.png` })
        await ctx.close()
      })
    }
  }
}
```

Check the `ks-mode` values in `web/src/theme/mode.ts` and use whatever it stores for light and dark. The scenes rely on the controls built in Tasks 9–12. Until those exist, the script only needs to *compile*.

- [ ] **Step 9: Build and check.**

Run: `cd web && bunx tsc -b && bun run test && bun run lint`
Expected: PASS.

Then run `make build` and start `./bin/karots-suite` (with `.env` loaded). Log in as the e2e cashier. `/till` shows the top bar (brand, name, the Sell tab, hotkey hints, Admin hidden for the cashier) and three empty panels. As the admin, the Admin link shows.

- [ ] **Step 10: Commit.**

```bash
git add web
git commit -m "feat(web): till data layer, plugin slots, cart provider (per-user persisted), till top bar + tabs, UI-gate screenshot tool"
```

---

### Task 9: Drawer UI — open, close, withdraw, deposit, no-sale (UI GATE 1)

**Files:**
- Create:
  - `web/src/features/till/drawer/count.ts`, `count.test.ts`
  - `web/src/features/till/drawer/DenomCounter.tsx`, `LockerSelect.tsx`, `sections.tsx`
  - `web/src/features/till/drawer/OpenRegister.tsx`, `CloseRegister.tsx`, `MoneyMove.tsx`
  - `web/src/features/till/drawer/SlipPrompt.tsx`, `DrawerPill.tsx`, `DrawerActions.tsx`
  - `web/src/features/till/TillDialogs.tsx`
- Modify:
  - `web/src/features/till/TillScreen.tsx`
  - `web/src/routes/_authed/till.tsx` (pass `status={<DrawerPill />}`)
  - `web/src/routes/_authed/till/index.tsx` (pass `logout`)

**Interfaces:**
- Consumes (Task 8): `drawerQuery`, `configQuery`, `useTillUi`, `useTillApi`, `PANEL`, `FIELD`, `printMoneySlip`, `errMsg`, `slots.drawerSections`, `slots.tillActions`.
- Produces:
  - `countTotal`, `toBreakdown`, `fromBreakdown`, `lockerDefault`, `loadOpenDraft`, `saveOpenDraft`, `clearOpenDraft`
  - `<DrawerActions />`, which Task 10 renders in the catalogue and Task 11 extends
  - `<TillDialogs />`, which Tasks 10–13 add cases to
  - `<OpenRegister state />`, `<DrawerPill />`

- [ ] **Step 1: Write the failing test.** Create `web/src/features/till/drawer/count.test.ts`:

```ts
import { beforeEach, describe, expect, it } from 'vitest'
import type { Denomination, Locker } from '@/api/types.gen'
import { clearOpenDraft, countTotal, fromBreakdown, loadOpenDraft, lockerDefault, saveOpenDraft, toBreakdown } from './count'

const ds: Denomination[] = [{ value: '500', isNote: true }, { value: '100', isNote: true }, { value: '5', isNote: false }]
const lk = (id: number, kind = 'safe'): Locker => ({ id, name: `L${id}`, kind, balance: '0.00' })

describe('drawer counts', () => {
  it('totals value × count and ignores junk', () => {
    expect(countTotal(ds, { '500': '2', '100': '3', '5': 'x' })).toBe(1300)
  })
  it('breakdown keeps positive whole counts only; round-trips from the last close', () => {
    expect(toBreakdown(ds, { '500': '2', '100': '0', '5': '1.7' })).toEqual({ '500': 2, '5': 1 })
    expect(fromBreakdown({ '500': 2 })).toEqual({ '500': '2' })
    expect(fromBreakdown(undefined)).toEqual({})
  })
  it('locker default: shop default if reachable; cash-out without untracked falls back to the first', () => {
    expect(lockerDefault([lk(1), lk(2)], 2, 'in', true)).toBe('2')
    expect(lockerDefault([lk(1)], 9, 'in', true)).toBe('')
    expect(lockerDefault([lk(1)], 0, 'out', false)).toBe('1')
    expect(lockerDefault([], 0, 'out', false)).toBe('')
  })
})

describe('opening-count draft', () => {
  beforeEach(() => localStorage.clear())
  it('recovers a recent draft, drops a stale one', () => {
    saveOpenDraft({ '500': '2' }, '3')
    expect(loadOpenDraft()).toEqual({ counts: { '500': '2' }, lockerId: '3' })
    expect(loadOpenDraft(Date.now() + 13 * 3600e3)).toBeNull()
    clearOpenDraft()
    expect(loadOpenDraft()).toBeNull()
  })
})
```

Run: `cd web && bunx vitest run src/features/till/drawer`
Expected: FAIL (the module is missing).

- [ ] **Step 2: Implement `count.ts`.** Create `web/src/features/till/drawer/count.ts`:

```ts
import type { Denomination, Locker } from '@/api/types.gen'
import { num } from '@/lib/money'

export type Counts = Record<string, string> // denomination value → typed count

export const countTotal = (ds: Denomination[], c: Counts) => ds.reduce((s, d) => s + num(d.value) * num(c[d.value]), 0)

export function toBreakdown(ds: Denomination[], c: Counts): Record<string, number> {
  const b: Record<string, number> = {}
  for (const d of ds) {
    const q = Math.floor(num(c[d.value]))
    if (q > 0) b[d.value] = q
  }
  return b
}

export function fromBreakdown(b: Record<string, number> | undefined): Counts {
  const c: Counts = {}
  for (const [k, v] of Object.entries(b ?? {})) c[k] = String(v)
  return c
}

/** Pre-selected locker for a money-move dialog: the shop default when this cashier
 * can reach it; for cash OUT with untracked cash off, the first locker (never an
 * option the server would reject); else untracked (""). */
export function lockerDefault(lockers: Locker[], defaultId: number, kind: 'in' | 'out', allowUntracked: boolean) {
  if (defaultId && lockers.some((l) => l.id === defaultId)) return String(defaultId)
  if (kind === 'out' && !allowUntracked && lockers.length) return String(lockers[0].id)
  return ''
}

// The opening count lives only in memory: an accidental reload mid-count used to
// lose it. Keep a draft for 12h while the till is still closed.
const DRAFT = 'till.openDraft'

export function loadOpenDraft(now = Date.now()): { counts: Counts; lockerId: string } | null {
  try {
    const d = JSON.parse(localStorage.getItem(DRAFT) ?? 'null')
    if (d && d.c && Object.keys(d.c).length && now - (d.t || 0) < 12 * 3600e3) return { counts: d.c, lockerId: d.l ?? '' }
  } catch {
    /* unreadable draft */
  }
  return null
}

export function saveOpenDraft(counts: Counts, lockerId: string) {
  try {
    localStorage.setItem(DRAFT, JSON.stringify({ c: counts, l: lockerId, t: Date.now() }))
  } catch {
    /* storage unavailable */
  }
}

export function clearOpenDraft() {
  try {
    localStorage.removeItem(DRAFT)
  } catch {
    /* storage unavailable */
  }
}
```

- [ ] **Step 3: The shared pieces.** Create `web/src/features/till/drawer/DenomCounter.tsx`:

```tsx
import type { Denomination } from '@/api/types.gen'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { FIELD } from '../ui'
import { countTotal, type Counts } from './count'

export function DenomCounter({ denoms, counts, onChange, sym, label = 'Counted' }: {
  denoms: Denomination[]
  counts: Counts
  onChange: (c: Counts) => void
  sym: string
  label?: string
}) {
  return (
    <div>
      <div className="grid grid-cols-2 gap-x-5 gap-y-1.5">
        {denoms.map((d) => (
          <label key={d.value} className="flex items-center gap-2 text-[13.5px]">
            <span className="w-24 text-right font-semibold tabular-nums">{money(sym, d.value)}</span>
            <span className="text-faint">×</span>
            <input aria-label={`Count of ${money(sym, d.value)}`} inputMode="numeric" autoComplete="off"
              value={counts[d.value] ?? ''} onChange={(e) => onChange({ ...counts, [d.value]: e.target.value.replace(/\D/g, '') })}
              className={cn(FIELD, 'h-9 w-16 px-2 text-center')} />
            <span className="ml-auto text-muted-foreground tabular-nums">{money(sym, num(d.value) * num(counts[d.value]))}</span>
          </label>
        ))}
      </div>
      <div className="mt-3 flex items-baseline justify-between border-t border-border pt-3">
        <span className="font-semibold text-muted-foreground">{label}</span>
        <span className="font-display text-2xl font-bold tabular-nums">{money(sym, countTotal(denoms, counts))}</span>
      </div>
    </div>
  )
}
```

Create `web/src/features/till/drawer/LockerSelect.tsx`:

```tsx
import type { Locker } from '@/api/types.gen'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { FIELD } from '../ui'

/** Where the cash comes from / goes to. untrackedLabel omitted = untracked not allowed. */
export function LockerSelect({ label, lockers, value, onChange, untrackedLabel, sym }: {
  label: string
  lockers: Locker[]
  value: string
  onChange: (v: string) => void
  untrackedLabel?: string
  sym: string
}) {
  if (!lockers.length) return null
  return (
    <label className="flex flex-col gap-1 text-[13px] font-semibold text-muted-foreground">
      {label}
      <select value={value} onChange={(e) => onChange(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
        {untrackedLabel && <option value="">{untrackedLabel}</option>}
        {lockers.map((l) => (
          <option key={l.id} value={String(l.id)}>{l.name} ({money(sym, l.balance)})</option>
        ))}
      </select>
    </label>
  )
}
```

Create `web/src/features/till/drawer/sections.tsx`:

```tsx
import { useRef } from 'react'
import { slots } from '@/plugins/slots'

/** Plugin drawer sections for the Open/Close dialog. runAll() saves each; false aborts. */
export function useDrawerSections(which: 'open' | 'close') {
  const saves = useRef(new Map<string, () => Promise<boolean>>())
  const nodes = slots.drawerSections.map((s) => (
    <s.Section key={s.key} which={which} register={(fn) => saves.current.set(s.key, fn)} />
  ))
  async function runAll() {
    for (const save of saves.current.values()) if (!(await save())) return false
    return true
  }
  return { nodes, runAll }
}
```

Create `web/src/features/till/drawer/SlipPrompt.tsx`:

```tsx
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { printMoneySlip } from '../print'
import { useTillUi } from '../TillUi'

/** "Ask before printing" mode: Print or Skip the CR- slip after a cash move. */
export function SlipPrompt({ receiptId }: { receiptId: number }) {
  const ui = useTillUi()
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>Print the receipt?</DialogTitle>
          <DialogDescription>A cash receipt was recorded for this drawer move.</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={ui.close}>Skip</Button>
          <Button autoFocus onClick={() => { void printMoneySlip(receiptId); ui.close() }}>Print</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 4: Open the till.** Create `web/src/features/till/drawer/OpenRegister.tsx`:

```tsx
import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { useEffect, useMemo, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { DrawerState, ReceiptRef } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery, errMsg } from '../api'
import { useTillUi } from '../TillUi'
import { PANEL } from '../ui'
import { clearOpenDraft, fromBreakdown, loadOpenDraft, lockerDefault, saveOpenDraft, toBreakdown, type Counts } from './count'
import { DenomCounter } from './DenomCounter'
import { LockerSelect } from './LockerSelect'
import { useDrawerSections } from './sections'

/** Covers the sell screen until the drawer is counted and opened. */
export function OpenRegister({ state }: { state: DrawerState }) {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const qc = useQueryClient()
  const draft = useMemo(() => loadOpenDraft(), [])
  const [counts, setCounts] = useState<Counts>(draft?.counts ?? {})
  const [lockerId, setLockerId] = useState(draft?.lockerId ?? lockerDefault(state.lockers, cfg.defaultLockerId, 'in', true))
  const [busy, setBusy] = useState(false)
  const sections = useDrawerSections('open')
  const last = state.summary.lastClosing

  useEffect(() => {
    if (draft) toast.success('Recovered your opening count')
  }, [draft])
  useEffect(() => {
    if (Object.keys(counts).length) saveOpenDraft(counts, lockerId)
  }, [counts, lockerId])

  async function open() {
    setBusy(true)
    try {
      const res = await api<ReceiptRef>('/till/drawer/open', {
        json: { breakdown: toBreakdown(state.denominations, counts), sourceLockerId: num(lockerId) },
      })
      clearOpenDraft()
      await sections.runAll() // best-effort for an open: reconciliation carries the last close
      await qc.invalidateQueries({ queryKey: drawerQuery.queryKey })
      toast.success('Till opened')
      ui.afterMove(res.receiptId)
      document.getElementById('till-search')?.focus()
    } catch (e) {
      toast.error(errMsg(e, 'Could not open the till'))
    } finally {
      setBusy(false)
    }
  }

  return (
    <div role="dialog" aria-modal="true" aria-labelledby="open-till-title"
      className="absolute inset-0 z-20 grid place-items-center bg-[color-mix(in_srgb,var(--background)_72%,transparent)] p-6 backdrop-blur-sm">
      <motion.div initial={{ scale: 0.96, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} transition={{ duration: 0.22 }}
        className={cn(PANEL, 'w-full max-w-[760px] p-6')}>
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div>
            <h2 id="open-till-title" className="font-display text-2xl font-bold">Open the till</h2>
            <p className="text-muted-foreground">Count the float in the drawer, then open.</p>
          </div>
          {last && cfg.allowContinueLast && (
            <div className="flex gap-2">
              <Button variant="secondary" onClick={() => setCounts(fromBreakdown(state.summary.lastBreakdown))}>
                Continue last count · {money(cfg.symbol, last)}
              </Button>
              <Button variant="ghost" onClick={() => setCounts({})}>Count fresh</Button>
            </div>
          )}
        </div>
        <div className={cn('mt-5 grid gap-6', sections.nodes.length > 0 && 'grid-cols-2')}>
          <div className="flex flex-col gap-4">
            <DenomCounter denoms={state.denominations} counts={counts} onChange={setCounts} sym={cfg.symbol} label="Opening float" />
            <LockerSelect label="Float taken from" lockers={state.lockers} value={lockerId} onChange={setLockerId}
              untrackedLabel="— already in the drawer —" sym={cfg.symbol} />
          </div>
          {sections.nodes.length > 0 && <div className="flex flex-col gap-4">{sections.nodes}</div>}
        </div>
        <Button className="mt-6 h-12 w-full text-base" disabled={busy} onClick={open}>Open the till</Button>
      </motion.div>
    </div>
  )
}
```

- [ ] **Step 5: Close the till.** Create `web/src/features/till/drawer/CloseRegister.tsx`:

```tsx
import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { CloseResult } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery, errMsg } from '../api'
import { printMoneySlip } from '../print'
import { useTillUi } from '../TillUi'
import { countTotal, lockerDefault, toBreakdown, type Counts } from './count'
import { DenomCounter } from './DenomCounter'
import { LockerSelect } from './LockerSelect'
import { useDrawerSections } from './sections'

export function CloseRegister({ logout }: { logout?: boolean }) {
  const cfg = useSuspenseQuery(configQuery).data
  const d = useQuery(drawerQuery).data
  const ui = useTillUi()
  const qc = useQueryClient()
  const lockers = d?.lockers ?? []
  const denoms = d?.denominations ?? []
  const [counts, setCounts] = useState<Counts>({})
  const [lockerId, setLockerId] = useState(() => lockerDefault(lockers, cfg.defaultLockerId, 'out', cfg.allowUntrackedCash))
  const [result, setResult] = useState<CloseResult | null>(null)
  const [busy, setBusy] = useState(false)
  const sections = useDrawerSections('close')
  const counted = countTotal(denoms, counts)
  const noLocker = !cfg.allowUntrackedCash && !lockers.length

  async function submit() {
    if (!cfg.allowUntrackedCash && !num(lockerId) && counted > 0) {
      toast.error(lockers.length ? 'Choose a locker to bank the cash into' : 'No locker set up — ask an admin to add one first')
      return
    }
    if (!(await sections.runAll())) return
    setBusy(true)
    try {
      const r = await api<CloseResult>('/till/drawer/close', { json: { breakdown: toBreakdown(denoms, counts), destLockerId: num(lockerId) } })
      setResult(r)
      void qc.invalidateQueries({ queryKey: drawerQuery.queryKey })
      if (!cfg.askToPrint && r.receiptId) void printMoneySlip(r.receiptId)
    } catch (e) {
      toast.error(errMsg(e, 'Could not close the till'))
    } finally {
      setBusy(false)
    }
  }

  async function done() {
    ui.close()
    if (logout) {
      await api('/auth/logout', { method: 'POST' }).catch(() => undefined)
      window.location.assign('/login')
    }
  }

  const diff = num(result?.difference)
  return (
    <Dialog open onOpenChange={(o) => !o && (result ? void done() : ui.close())}>
      <DialogContent className="sm:max-w-[700px]">
        {!result ? (
          <>
            {logout && (
              <p className="rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] px-3 py-2 text-sm font-semibold text-warning">
                Close your till before signing out — count the drawer to finish.
              </p>
            )}
            <DialogHeader>
              <DialogTitle>Close the till</DialogTitle>
              <DialogDescription>Count the drawer to close this session. Expected in the drawer: {money(cfg.symbol, d?.summary.expected)}.</DialogDescription>
            </DialogHeader>
            <div className={cn('grid gap-6', sections.nodes.length > 0 && 'grid-cols-2')}>
              <div className="flex flex-col gap-4">
                <DenomCounter denoms={denoms} counts={counts} onChange={setCounts} sym={cfg.symbol} />
                <LockerSelect label="Bank cash into" lockers={lockers} value={lockerId} onChange={setLockerId}
                  untrackedLabel={cfg.allowUntrackedCash ? '— leave cash in drawer —' : undefined} sym={cfg.symbol} />
                {noLocker && <p className="text-sm text-danger">No cash locker set up. Ask an admin to add one to bank the closing cash (untracked cash is turned off).</p>}
              </div>
              {sections.nodes.length > 0 && <div className="flex flex-col gap-4">{sections.nodes}</div>}
            </div>
            <DialogFooter>
              <Button variant="ghost" onClick={ui.close}>Cancel</Button>
              <Button disabled={busy || (noLocker && counted > 0)} onClick={submit}>Close &amp; reconcile</Button>
            </DialogFooter>
          </>
        ) : (
          <>
            <DialogHeader><DialogTitle>Till closed</DialogTitle></DialogHeader>
            <dl className="grid grid-cols-[1fr_auto] gap-y-1.5 text-[15px] tabular-nums">
              <dt className="text-muted-foreground">Cash sales</dt><dd>{money(cfg.symbol, result.cashSales)}</dd>
              <dt className="text-muted-foreground">Expected</dt><dd>{money(cfg.symbol, result.expectedCash)}</dd>
              <dt className="text-muted-foreground">Counted</dt><dd>{money(cfg.symbol, result.counted)}</dd>
              <dt className="font-semibold">Over / short</dt>
              <dd className={cn('font-display text-xl font-bold', diff < 0 ? 'text-danger' : 'text-success')}>{diff > 0 ? '+' : ''}{money(cfg.symbol, diff)}</dd>
            </dl>
            <DialogFooter>
              {cfg.askToPrint && result.receiptId && <Button variant="secondary" onClick={() => void printMoneySlip(result.receiptId!)}>Print slip</Button>}
              <Button autoFocus onClick={() => void done()}>{logout ? 'Sign out' : 'Done'}</Button>
            </DialogFooter>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 6: Withdraw, deposit, the pill and the action row.** Create `web/src/features/till/drawer/MoneyMove.tsx`:

```tsx
import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ReceiptRef } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery, errMsg } from '../api'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'
import { lockerDefault } from './count'
import { LockerSelect } from './LockerSelect'

/** Mid-shift cash out (↑ red) or in (↓ green) — karots' money-direction cue. */
export function MoneyMove({ kind }: { kind: 'withdraw' | 'deposit' }) {
  const out = kind === 'withdraw'
  const cfg = useSuspenseQuery(configQuery).data
  const d = useQuery(drawerQuery).data
  const ui = useTillUi()
  const qc = useQueryClient()
  const lockers = d?.lockers ?? []
  const [amount, setAmount] = useState('')
  const [reason, setReason] = useState('')
  const [lockerId, setLockerId] = useState(() => lockerDefault(lockers, cfg.defaultLockerId, out ? 'out' : 'in', cfg.allowUntrackedCash))
  const [busy, setBusy] = useState(false)
  const untrackedOk = !out || cfg.allowUntrackedCash

  async function submit() {
    const a = num(amount)
    if (a <= 0) return void toast.error(`Enter an amount to ${kind}`)
    if (out && a > num(d?.summary.expected)) return void toast.error("Can't withdraw more than what's in the drawer")
    if (!untrackedOk && !num(lockerId)) {
      return void toast.error(lockers.length ? 'Choose a locker to withdraw into' : 'No locker set up — ask an admin to add one first')
    }
    setBusy(true)
    try {
      const res = await api<ReceiptRef>(`/till/drawer/${kind}`, {
        json: { amount: String(a), reason, lockerId: num(lockerId), deferDrawerKick: !cfg.askToPrint },
      })
      void qc.invalidateQueries({ queryKey: drawerQuery.queryKey })
      toast.success(out ? 'Cash withdrawn' : 'Cash deposited')
      ui.close()
      ui.afterMove(res.receiptId, true) // auto-print merges the drawer pulse into the slip job
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>
            <span className={out ? 'text-danger' : 'text-success'}>{out ? '↑' : '↓'}</span> {out ? 'Withdraw cash' : 'Deposit cash'}
          </DialogTitle>
          <DialogDescription>In the drawer now: {money(cfg.symbol, d?.summary.expected)}</DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-3" onSubmit={(e) => { e.preventDefault(); void submit() }}>
          <input autoFocus aria-label="Amount" placeholder="Amount" inputMode="decimal" autoComplete="off" value={amount}
            onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'font-display text-lg font-bold tabular-nums')} />
          <input aria-label="Reason" placeholder="Reason (optional)" autoComplete="off" value={reason}
            onChange={(e) => setReason(e.target.value)} className={FIELD} />
          <LockerSelect label={out ? 'Put into' : 'Taken from'} lockers={lockers} value={lockerId} onChange={setLockerId}
            untrackedLabel={untrackedOk ? '— untracked (no locker) —' : undefined} sym={cfg.symbol} />
          {!untrackedOk && !lockers.length && <p className="text-sm text-danger">No cash locker set up — ask an admin to add one first.</p>}
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={ui.close}>Cancel</Button>
            <Button type="submit" disabled={busy || (!untrackedOk && !lockers.length)}>{out ? 'Withdraw' : 'Deposit'}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
```

Create `web/src/features/till/drawer/DrawerPill.tsx`:

```tsx
import { useQuery } from '@tanstack/react-query'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery } from '../api'

/** Top-bar drawer status: "Till open · float Rs … · in drawer Rs …". */
export function DrawerPill() {
  const cfg = useQuery(configQuery).data
  const d = useQuery(drawerQuery).data
  if (!cfg || !d) return null
  const s = d.summary.session
  return (
    <span className="flex items-center gap-2 whitespace-nowrap rounded-full border border-border bg-muted px-3 py-1.5 text-[13px]">
      <span className={cn('h-2 w-2 rounded-full', s ? 'bg-success shadow-[0_0_0_4px_color-mix(in_srgb,var(--success)_25%,transparent)]' : 'bg-faint')} />
      {s ? (
        <>Till open · float {money(cfg.symbol, s.openingCash)} · in drawer <b className="tabular-nums">{money(cfg.symbol, d.summary.expected)}</b></>
      ) : (
        'Till closed'
      )}
    </span>
  )
}
```

Create `web/src/features/till/drawer/DrawerActions.tsx`:

```tsx
import { useSuspenseQuery } from '@tanstack/react-query'
import { Archive, Lock as LockIcon } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { slots } from '@/plugins/slots'
import { configQuery, errMsg } from '../api'
import { useTillUi } from '../TillUi'
import { CHIP } from '../ui'

/** The till's action row (bottom of the catalogue): drawer moves + plugin actions. */
export function DrawerActions() {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const noSale = () =>
    api('/till/drawer/no-sale', { method: 'POST' }).then(() => toast.success('Drawer opened')).catch((e) => toast.error(errMsg(e)))
  return (
    <div className="flex flex-wrap items-center gap-2" role="toolbar" aria-label="Till actions">
      <button type="button" className={CHIP} onClick={() => ui.open({ kind: 'deposit' })}>
        <span className="text-lg leading-none text-success">↓</span> Deposit
      </button>
      <button type="button" className={CHIP} onClick={() => ui.open({ kind: 'withdraw' })}>
        <span className="text-lg leading-none text-danger">↑</span> Withdraw
      </button>
      {cfg.openCashDrawer && (
        <button type="button" className={CHIP} onClick={noSale} title="Open the cash drawer without a sale">
          <Archive size={14} /> Open drawer
        </button>
      )}
      {slots.tillActions.map((a) => (
        <button key={a.key} type="button" className={CHIP} onClick={() => ui.open({ kind: 'action', key: a.key })}>{a.label}</button>
      ))}
      <button type="button" className={`${CHIP} ml-auto`} onClick={() => ui.open({ kind: 'close' })}>
        <LockIcon size={14} /> Close till
      </button>
    </div>
  )
}
```

- [ ] **Step 7: Dialog host, screen wiring, logout flow.** Create `web/src/features/till/TillDialogs.tsx`:

```tsx
import { slots } from '@/plugins/slots'
import { CloseRegister } from './drawer/CloseRegister'
import { MoneyMove } from './drawer/MoneyMove'
import { SlipPrompt } from './drawer/SlipPrompt'
import { useTillApi, useTillUi } from './TillUi'

/** Renders whichever till dialog is open. Tasks 10–13 add their cases here. */
export function TillDialogs() {
  const ui = useTillUi()
  const till = useTillApi()
  const d = ui.dialog
  if (!d) return null
  switch (d.kind) {
    case 'withdraw':
    case 'deposit':
      return <MoneyMove kind={d.kind} />
    case 'close':
      return <CloseRegister logout={d.logout} />
    case 'slip':
      return <SlipPrompt receiptId={d.receiptId} />
    case 'action': {
      const a = slots.tillActions.find((x) => x.key === d.key)
      return a ? <a.Panel till={till} onClose={ui.close} /> : null
    }
    default:
      return null
  }
}
```

Replace `web/src/features/till/TillScreen.tsx`. It now takes `logout`, renders the open-till overlay, the drawer action row inside the catalogue section, the dialog host, and the logout-close flow:

```tsx
import { useQuery } from '@tanstack/react-query'
import { MotionConfig } from 'motion/react'
import { useEffect, useRef } from 'react'
import { api } from '@/api/client'
import { cn } from '@/lib/utils'
import { drawerQuery } from './api'
import { DrawerActions } from './drawer/DrawerActions'
import { OpenRegister } from './drawer/OpenRegister'
import { TillDialogs } from './TillDialogs'
import { TillUiProvider, useTillUi } from './TillUi'
import { PANEL } from './ui'

/** Sign-out was refused (TILL_OPEN): open the count in logout mode; if the till is
 * somehow already closed, just finish signing out. */
function LogoutClose({ logout }: { logout: boolean }) {
  const ui = useTillUi()
  const d = useQuery(drawerQuery).data
  const handled = useRef(false)
  useEffect(() => {
    if (!logout || !d || handled.current) return
    handled.current = true
    if (d.summary.session) ui.open({ kind: 'close', logout: true })
    else void api('/auth/logout', { method: 'POST' }).finally(() => window.location.assign('/login'))
  }, [logout, d, ui])
  return null
}

function Body() {
  const drawer = useQuery(drawerQuery).data
  return (
    <div className="relative grid h-full min-h-0 grid-cols-[1fr_360px] grid-rows-[minmax(0,1fr)_auto] gap-[14px] p-[14px] [grid-template-areas:'cat_cart'_'cat_pay'] min-[1200px]:grid-cols-[1fr_392px_356px] min-[1200px]:grid-rows-[minmax(0,1fr)] min-[1200px]:[grid-template-areas:'cat_cart_pay']">
      <section aria-label="Catalogue" className="flex min-h-0 flex-col gap-3 [grid-area:cat]">
        <div className="min-h-0 flex-1" />
        <DrawerActions />
      </section>
      <section aria-label="Cart" className={cn(PANEL, 'min-h-0 [grid-area:cart]')} />
      <section aria-label="Payment" className={cn(PANEL, 'min-h-0 [grid-area:pay]')} />
      {drawer && !drawer.summary.session && <OpenRegister state={drawer} />}
    </div>
  )
}

export function TillScreen({ logout = false }: { logout?: boolean }) {
  return (
    <MotionConfig reducedMotion="user">
      <TillUiProvider>
        <Body />
        <TillDialogs />
        <LogoutClose logout={logout} />
      </TillUiProvider>
    </MotionConfig>
  )
}
```

In `web/src/routes/_authed/till/index.tsx`, change the component to:

```tsx
  component: function TillIndex() {
    const { logout } = Route.useSearch()
    return <TillScreen logout={!!logout} />
  },
```

In `web/src/routes/_authed/till.tsx`, pass `status={<DrawerPill />}` to `TillShell`, importing it from `@/features/till/drawer/DrawerPill`.

- [ ] **Step 8: Test and build.**

Run: `cd web && bunx vitest run && bunx tsc -b && bun run lint`
Expected: PASS.

- [ ] **Step 9: UI GATE 1 — drawer.**
  1. Run `make e2e-seed` (Task 14 Step 1 writes the till seed. If you reach this gate first, do Task 14 Step 1 now; it is independent). Then `make build` and start the binary.
  2. `cd web && SHOTS=gate1 SHOTS_SCENE=closed bunx playwright test e2e/till-shots.spec.ts` gives the open-till overlay in all 4 palette × mode combinations at 1366 and 1024.
  3. With the Playwright MCP browser, capture the **Withdraw**, **Deposit** and **Close the till** (count and result views) dialogs and the top-bar pill (open and closed) in Studio light, Studio dark, Night light and Night dark.
  4. **Stop.** Show the screenshots and wait for approval before Task 10.

  Checklist:
  - token colours only
  - ↑ is red and ↓ is green
  - money in tabular figures
  - totals in Sora
  - Night panels are glass
  - nothing looks like stock shadcn or karots-pos

- [ ] **Step 10: Commit** after approval.

```bash
git add web
git commit -m "feat(web): till drawer — open (float count, continue last, draft recovery), close/reconcile (+logout mode), withdraw/deposit, no-sale, CR- slip prompt, drawer pill"
```

---

### Task 10: Catalogue — scan/search, groups, frequently sold, product info, quick item

**Files:**
- Create in `web/src/features/till/catalog/`:
  - `useAddProduct.ts`, `useEntry.ts`
  - `CatalogPanel.tsx`, `ProductTile.tsx`, `GroupCard.tsx`
  - `ProductInfo.tsx`, `QuickItem.tsx`
- Modify: `web/src/features/till/TillScreen.tsx`, `web/src/features/till/TillDialogs.tsx`

**Interfaces:**
- Consumes:
  - from Task 6: `lotsFor`, `lotExpired`, `parseEntry`, and the actions `add` and `quickSet`
  - from Task 8: `useTillUi`, `useTillApi`, the queries, `slots.tillMenu`, `useDebounced`
  - from Task 9: `DrawerActions`
- Produces:
  - `useAddProduct(): (p: TillProduct) => void`. Task 11's PricePick commits lots directly with `dispatch` and doesn't use it.
  - `<CatalogPanel />`

- [ ] **Step 1: Adding and entry hooks.** Create `web/src/features/till/catalog/useAddProduct.ts`:

```ts
import { useQuery } from '@tanstack/react-query'
import { useCallback } from 'react'
import type { TillProduct } from '@/api/types.gen'
import { priceOptionsQuery } from '../api'
import { lotsFor } from '../cart/lines'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

/** Add one unit of p: ask "which price?" first when its lots disagree or the lot
 * that would sell is expired; offer a plugin sale suggestion on a NEW line. */
export function useAddProduct() {
  const { state, dispatch } = useTill()
  const ui = useTillUi()
  const opts = useQuery(priceOptionsQuery).data
  return useCallback((p: TillProduct) => {
    const lots = lotsFor(opts, p.id)
    if (lots.length) {
      ui.open({ kind: 'pricePick', product: p, lots })
      return
    }
    const isNew = !state.lines.some((l) => l.productId === p.id && l.batchId === 0 && !l.isService)
    dispatch({ type: 'add', product: p, lots })
    if (p.suggestion && isNew) ui.open({ kind: 'suggestion', product: p })
  }, [opts, state.lines, dispatch, ui])
}
```

Create `web/src/features/till/catalog/useEntry.ts`:

```ts
import { useQueryClient } from '@tanstack/react-query'
import { useCallback } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { TillProduct } from '@/api/types.gen'
import { errMsg, searchQuery } from '../api'
import { parseEntry } from '../cart/entry'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { useAddProduct } from './useAddProduct'

const byBarcode = (code: string) => api<TillProduct>(`/till/products/barcode/${encodeURIComponent(code)}`)
const notFound = (e: unknown) => e instanceof ApiError && e.status === 404

/** Enter in the scan/search box. The box is cleared IMMEDIATELY so a scanner's next
 * code never lands on top of this one while the lookup is in flight. */
export function useEntry() {
  const ui = useTillUi()
  const { dispatch } = useTill()
  const add = useAddProduct()
  const qc = useQueryClient()
  return useCallback(async (raw: string) => {
    const e = parseEntry(raw)
    if (e.kind === 'empty') return
    ui.setSearch('')
    if (e.kind === 'quick') {
      // Phone rush handoff: SET the qty (idempotent on re-scan), no lot prompt.
      if (e.qty <= 0) return void toast.error('Quick-sale code has no quantity')
      try {
        const p = await byBarcode(e.barcode)
        dispatch({ type: 'quickSet', product: p, qty: e.qty })
        toast.success(`${p.name} × ${e.qty}`)
      } catch (err) {
        toast.error(notFound(err) ? `No item with barcode ${e.barcode} — add it manually` : errMsg(err, 'Lookup failed'))
      }
      return
    }
    if (e.kind === 'barcode') {
      try {
        return add(await byBarcode(e.code))
      } catch (err) {
        if (!notFound(err)) return void toast.error(errMsg(err, 'Lookup failed'))
      }
    }
    const q = e.kind === 'barcode' ? e.code : e.q
    const page = await qc.fetchQuery(searchQuery(q))
    if (page.items.length === 1) return add(page.items[0])
    if (page.items.length === 0) {
      const digits = /^\d+$/.test(q)
      return ui.open({ kind: 'quickItem', barcode: digits ? q : '', name: digits ? '' : q })
    }
    ui.setSearch(q) // several matches: leave them on screen to tap
  }, [ui, dispatch, add, qc])
}
```

- [ ] **Step 2: Tiles.** Create `web/src/features/till/catalog/ProductTile.tsx`:

```tsx
import { Info } from 'lucide-react'
import { motion } from 'motion/react'
import type { Lot, TillProduct } from '@/api/types.gen'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lotExpired, lotsFor } from '../cart/lines'
import { TILE } from '../ui'

/** A small cue that this product will ask "which price?". */
export function lotNote(opts: Record<string, Lot[]> | undefined, id: number) {
  const lots = lotsFor(opts, id)
  if (!lots.length) return null
  return lotExpired(lots[0]) ? 'Expired lot first' : `${new Set(lots.map((l) => l.price)).size} prices`
}

export function ProductTile({ p, sym, note, onAdd, onInfo }: {
  p: TillProduct
  sym: string
  note: string | null
  onAdd: () => void
  onInfo: () => void
}) {
  const out = !p.isService && num(p.stockQty) <= 0
  return (
    <div className="relative">
      <motion.button type="button" onClick={onAdd} whileHover={{ y: -2 }} whileTap={{ scale: 0.97 }} transition={{ duration: 0.12 }}
        className={cn(TILE, 'pr-8', out && 'opacity-60')}>
        <span className="text-[13.5px] font-semibold leading-tight">
          {p.emoji && <span className="mr-1">{p.emoji}</span>}
          {p.name}
        </span>
        {(note || p.badges.length > 0) && (
          <span className="flex flex-wrap gap-1">
            {note && <span className="rounded-md bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] px-1.5 text-[11px] font-bold text-warning">{note}</span>}
            {p.badges.map((b) => <span key={b} className="rounded-md bg-brand-soft px-1.5 text-[11px] font-bold">{b}</span>)}
          </span>
        )}
        <span className="flex items-baseline justify-between gap-2">
          <span className="text-[15px] font-bold tabular-nums">{money(sym, p.sellingPrice)}</span>
          <span className="text-[11.5px] text-faint">{p.isService ? 'service' : `${qtyText(p.stockQty)} ${p.unitAbbr} left`}</span>
        </span>
      </motion.button>
      <button type="button" aria-label={`About ${p.name}`} onClick={onInfo}
        className="absolute right-1.5 top-1.5 grid h-6 w-6 place-items-center rounded-full text-faint hover:bg-muted hover:text-foreground">
        <Info size={14} />
      </button>
    </div>
  )
}
```

Create `web/src/features/till/catalog/GroupCard.tsx`:

```tsx
import { ChevronRight } from 'lucide-react'
import { motion } from 'motion/react'
import type { TillGroup } from '@/api/types.gen'
import { cn } from '@/lib/utils'
import { TILE } from '../ui'

export function GroupCard({ group, onOpen }: { group: TillGroup; onOpen: () => void }) {
  return (
    <motion.button type="button" onClick={onOpen} whileHover={{ y: -2 }} whileTap={{ scale: 0.97 }} transition={{ duration: 0.12 }}
      className={cn(TILE, 'bg-muted')}>
      <span className="text-2xl leading-none">{group.emoji || '🗂️'}</span>
      <span className="flex items-center justify-between gap-2 text-[14px] font-bold">
        {group.name}
        <span className="flex items-center text-[11.5px] font-semibold text-faint">{group.itemCount} <ChevronRight size={14} /></span>
      </span>
    </motion.button>
  )
}
```

- [ ] **Step 3: The panel.** Create `web/src/features/till/catalog/CatalogPanel.tsx`:

```tsx
import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { ChevronLeft, RotateCcw, Search } from 'lucide-react'
import type { TillGroup, TillProduct } from '@/api/types.gen'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { configQuery, groupQuery, groupsQuery, priceOptionsQuery, quickPicksQuery, searchQuery } from '../api'
import { Kbd } from '../bits'
import { DrawerActions } from '../drawer/DrawerActions'
import { useTillApi, useTillUi } from '../TillUi'
import { CHIP, PANEL } from '../ui'
import { GroupCard } from './GroupCard'
import { lotNote, ProductTile } from './ProductTile'
import { useAddProduct } from './useAddProduct'
import { useEntry } from './useEntry'

export function CatalogPanel() {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const till = useTillApi()
  const q = useDebounced(ui.search, 300)
  const searching = ui.search.trim() !== '' && q.trim() !== ''
  const current = ui.stack.at(-1)
  const top = useQuery(groupsQuery).data ?? []
  const group = useQuery({ ...groupQuery(current?.id ?? 0), enabled: !!current && !searching })
  const results = useQuery({ ...searchQuery(q), enabled: searching })
  const picks = useQuery(quickPicksQuery).data ?? []
  const opts = useQuery(priceOptionsQuery).data
  const add = useAddProduct()
  const submit = useEntry()
  const menuRoot = slots.tillMenu.find((m) => m.key === ui.menu)

  let title = 'Frequently sold'
  let products: TillProduct[] = picks
  let children: TillGroup[] = []
  if (searching) {
    title = `Results for “${q}”`
    products = results.data?.items ?? []
  } else if (current) {
    title = ui.stack.map((g) => g.name).join(' / ')
    products = group.data?.products ?? []
    children = group.data?.children ?? []
  }
  const openTop = (g: TillGroup) => {
    ui.setSearch('')
    ui.setMenu(null)
    ui.setStack([g])
  }

  return (
    <>
      <div className={cn(PANEL, 'flex h-[52px] shrink-0 items-center gap-3 px-4')}>
        <Search size={18} className="text-muted-foreground" />
        <input id="till-search" aria-label="Scan a barcode or search products" placeholder="Scan a barcode or search products"
          autoComplete="off" autoFocus value={ui.search} onChange={(e) => ui.setSearch(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              void submit(ui.search)
            }
          }}
          className="h-full min-w-0 flex-1 bg-transparent text-[15px] outline-none placeholder:text-faint" />
        <Kbd>F2</Kbd>
        <button type="button" onClick={ui.reset} title="Back to the menu — the cart stays" className={CHIP}>
          <RotateCcw size={14} /> Menu <Kbd>F1</Kbd>
        </button>
      </div>

      <div className="flex flex-wrap gap-2" role="toolbar" aria-label="Groups">
        <button type="button" className={CHIP} aria-pressed={!current && !ui.menu && !searching} onClick={ui.reset}>Frequently sold</button>
        {slots.tillMenu.map((m) => (
          <button key={m.key} type="button" className={CHIP} aria-pressed={ui.menu === m.key}
            onClick={() => { ui.setSearch(''); ui.setStack([]); ui.setMenu(m.key) }}>
            {m.emoji} {m.label}
          </button>
        ))}
        {top.map((g) => (
          <button key={g.id} type="button" className={CHIP} aria-pressed={ui.stack[0]?.id === g.id && !searching} onClick={() => openTop(g)}>
            {g.emoji && <span>{g.emoji}</span>}{g.name}
          </button>
        ))}
      </div>

      {menuRoot ? (
        <div className={cn(PANEL, 'min-h-0 flex-1 overflow-auto p-4')}>
          <menuRoot.Panel till={till} onDone={() => ui.setMenu(null)} />
        </div>
      ) : (
        <>
          <div className="flex items-center justify-between text-[12px] font-semibold text-faint">
            <span className="flex items-center gap-2">
              {current && !searching && (
                <button type="button" className={CHIP} onClick={() => ui.setStack(ui.stack.slice(0, -1))}><ChevronLeft size={14} /> Back</button>
              )}
              {title}
            </span>
            <span>{products.length} items</span>
          </div>
          <div className="grid min-h-0 flex-1 auto-rows-min grid-cols-[repeat(auto-fill,minmax(160px,1fr))] gap-2.5 overflow-auto p-0.5">
            {!searching && children.map((g) => <GroupCard key={`g${g.id}`} group={g} onOpen={() => ui.setStack([...ui.stack, g])} />)}
            {products.map((p) => (
              <ProductTile key={p.id} p={p} sym={cfg.symbol} note={lotNote(opts, p.id)} onAdd={() => add(p)}
                onInfo={() => ui.open({ kind: 'info', product: p })} />
            ))}
            {!products.length && !(children.length && !searching) && (
              <p className="col-span-full py-10 text-center text-faint">
                {searching ? 'No product matches. Press Enter to add it as a quick item.' : current ? 'This group is empty.' : 'Frequently sold items appear here once the shop has sales.'}
              </p>
            )}
          </div>
        </>
      )}
      <DrawerActions />
    </>
  )
}
```

- [ ] **Step 4: Info and quick item dialogs.** Create `web/src/features/till/catalog/ProductInfo.tsx`:

```tsx
import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ProductDetail, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, qtyText } from '@/lib/money'
import { configQuery } from '../api'
import { useTillUi } from '../TillUi'
import { useAddProduct } from './useAddProduct'

/** ⓘ popup: what the card knows at once, then /detail (cost only for allowed users). */
export function ProductInfo({ product }: { product: TillProduct }) {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const add = useAddProduct()
  const d = useQuery({ queryKey: ['till', 'detail', product.id], queryFn: () => api<ProductDetail>(`/till/products/${product.id}/detail`) }).data
  const rows: [string, string][] = [
    ['Price', money(cfg.symbol, d?.sellingPrice ?? product.sellingPrice)],
    ['In stock', product.isService ? 'service' : `${qtyText(d?.stockQty ?? product.stockQty)} ${product.unitAbbr}`],
  ]
  if (d?.category) rows.push(['Category', d.category])
  if (d?.barcode) rows.push(['Barcode', d.barcode])
  if (d?.warranty) rows.push(['Warranty', d.warranty])
  if (d?.showCost) {
    rows.push(['Cost', money(cfg.symbol, d.costPrice)])
    if (d.marginPct) rows.push(['Margin', `${d.marginPct}%`])
  }
  for (const r of d?.rows ?? []) rows.push([r.label, r.value])
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>{product.name}</DialogTitle>
          <DialogDescription>{d ? 'Product details' : 'Loading details…'}</DialogDescription>
        </DialogHeader>
        <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1.5 text-[14px]">
          {rows.map(([k, v]) => (
            <div key={k} className="contents"><dt className="text-muted-foreground">{k}</dt><dd className="text-right font-semibold tabular-nums">{v}</dd></div>
          ))}
        </dl>
        <DialogFooter>
          <Button variant="ghost" onClick={ui.close}>Close</Button>
          <Button autoFocus onClick={() => { ui.close(); add(product) }}>Add to cart</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

Create `web/src/features/till/catalog/QuickItem.tsx`:

```tsx
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { GeneratedBarcode, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { errMsg, unitsQuery } from '../api'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'

/** Sell something not in the catalogue yet (it's created for admin review). */
export function QuickItem({ barcode: initialBarcode, name: initialName }: { barcode: string; name: string }) {
  const ui = useTillUi()
  const { dispatch } = useTill()
  const qc = useQueryClient()
  const units = useQuery(unitsQuery).data ?? []
  const [name, setName] = useState(initialName)
  const [price, setPrice] = useState('')
  const [qty, setQty] = useState('1')
  const [barcode, setBarcode] = useState(initialBarcode)
  const [unitId, setUnitId] = useState<number>(0)
  const unit = units.find((u) => u.id === unitId) ?? units.find((u) => u.abbreviation === 'pcs') ?? units[0]

  async function generate() {
    try {
      setBarcode((await api<GeneratedBarcode>('/till/barcode/generate')).barcode)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  async function submit() {
    if (!name.trim()) return void toast.error('Enter an item name')
    if (price.trim() === '' || !(num(price) >= 0)) return void toast.error('Enter a price')
    let q = num(qty)
    if (!unit?.allowDecimal) q = Math.round(q)
    if (!(q > 0)) q = 1
    try {
      const p = await api<TillProduct>('/till/quick-item', {
        json: { name: name.trim(), price: String(num(price)), qty: String(q), barcode: barcode.trim(), unitId: unit?.id ?? 0 },
      })
      dispatch({ type: 'quickSet', product: p, qty: q })
      toast.success('Item added')
      ui.close()
      void qc.invalidateQueries({ queryKey: ['till', 'search'] })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>Quick item</DialogTitle>
          <DialogDescription>Sell an item that isn’t in the catalogue yet. An admin can finish its details later.</DialogDescription>
        </DialogHeader>
        <form className="grid grid-cols-2 gap-3" onSubmit={(e) => { e.preventDefault(); void submit() }}>
          <input autoFocus aria-label="Item name" placeholder="Item name" autoComplete="off" value={name} onChange={(e) => setName(e.target.value)} className={cn(FIELD, 'col-span-2')} />
          <input aria-label="Price" placeholder="Price" inputMode="decimal" autoComplete="off" value={price} onChange={(e) => setPrice(e.target.value.replace(/[^\d.]/g, ''))} className={FIELD} />
          <div className="flex gap-2">
            <input aria-label="Quantity" inputMode="decimal" autoComplete="off" value={qty} onChange={(e) => setQty(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'w-20')} />
            <select aria-label="Unit" value={unit?.id ?? 0} onChange={(e) => setUnitId(Number(e.target.value))} className={cn(FIELD, 'min-w-0 flex-1 appearance-auto')}>
              {units.map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
            </select>
          </div>
          <div className="col-span-2 flex gap-2">
            <input aria-label="Barcode" placeholder="Barcode (optional)" autoComplete="off" value={barcode} onChange={(e) => setBarcode(e.target.value)} className={cn(FIELD, 'min-w-0 flex-1')} />
            <Button type="button" variant="secondary" className="h-[var(--control-h)]" onClick={generate}>Generate</Button>
          </div>
          <DialogFooter className="col-span-2">
            <Button type="button" variant="ghost" onClick={ui.close}>Cancel</Button>
            <Button type="submit">Add to cart</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 5: Wire it in.**
  - In `TillDialogs.tsx`, add these cases:
    - `case 'info': return <ProductInfo product={d.product} />`
    - `case 'quickItem': return <QuickItem barcode={d.barcode} name={d.name} />`
  - In `TillScreen.tsx`, replace the whole `<section aria-label="Catalogue" …>…</section>` with:

```tsx
      <section aria-label="Catalogue" className="flex min-h-0 flex-col gap-3 [grid-area:cat]">
        <CatalogPanel />
      </section>
```

  - Then remove the `DrawerActions` import from `TillScreen.tsx`, because `CatalogPanel` renders it now.

- [ ] **Step 6: Check.**

Run: `cd web && bunx tsc -b && bun run lint && bun run test`
Expected: PASS.

Then check by hand on `make build`: scanning `4790000000011` does nothing visible yet (the cart panel lands in Task 11), but there are no errors in the console. Groups drill in and out, F1 returns to Frequently sold, ⓘ opens info, and an unknown name opens Quick item prefilled.

- [ ] **Step 7: Commit.**

```bash
git add web
git commit -m "feat(web): till catalogue — scan/search box (instant clear for scanners, KQ handoff), group chips + drill-down, frequently sold, ⓘ info, quick item"
```

---

### Task 11: Cart — lines, lots, serials, discounts, held sales (UI GATE 2)

**Files:**
- Create in `web/src/features/till/cart-ui/`:
  - `CartPanel.tsx`, `CartLineRow.tsx`
  - `PricePick.tsx`, `Suggestion.tsx`
  - `HeldSales.tsx`, `useHold.ts`
- Modify:
  - `web/src/features/till/TillScreen.tsx`
  - `web/src/features/till/TillDialogs.tsx`
  - `web/src/features/till/drawer/DrawerActions.tsx`

**Interfaces:**
- Consumes:
  - from Task 6: the reducer actions and `toHeldBlob`
  - from Task 5: `lineDiscount`, `lineGross` and `saleTotal`
  - from Tasks 8–10: `useTillUi` and `useAddProduct`
- Produces:
  - `useHold(): () => Promise<void>` (Task 13 hotkeys and Task 12 buttons use it)
  - `useResume()`
  - `<CartPanel />`

- [ ] **Step 1: A line.** Create `web/src/features/till/cart-ui/CartLineRow.tsx`:

```tsx
import { useQuery } from '@tanstack/react-query'
import { Minus, Plus, Tag, X } from 'lucide-react'
import { motion } from 'motion/react'
import { useEffect, useState } from 'react'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { priceOptionsQuery } from '../api'
import { Seg } from '../bits'
import { lotsFor } from '../cart/lines'
import { lineDiscount, lineGross } from '../cart/totals'
import type { CartLine } from '../cart/types'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'

function DiscountChip({ line, sym }: { line: CartLine; sym: string }) {
  const { dispatch } = useTill()
  const [open, setOpen] = useState(false)
  const on = num(line.discount) > 0
  const set = (discount: string, discountType = line.discountType) => dispatch({ type: 'lineDiscount', key: line.key, discount, discountType })
  if (!open) {
    return (
      <button type="button" onClick={() => setOpen(true)}
        className={cn('flex items-center gap-1 rounded-full border border-dashed border-border px-2 py-0.5 text-[11.5px] font-semibold',
          on && 'border-solid border-transparent bg-[color-mix(in_srgb,var(--success)_18%,transparent)] text-success')}>
        <Tag size={12} />
        {on ? (line.discountType === 'percent' ? `${line.discount}% off` : `${money(sym, line.discount)} off each`) : 'Discount'}
      </button>
    )
  }
  return (
    <span className="flex items-center gap-1">
      <input autoFocus aria-label={`Discount for ${line.name}`} inputMode="decimal" autoComplete="off" value={line.discount}
        onChange={(e) => set(e.target.value.replace(/[^\d.]/g, ''))}
        onKeyDown={(e) => (e.key === 'Enter' || e.key === 'Escape') && setOpen(false)} className={cn(FIELD, 'h-7 w-16 px-2 text-[13px]')} />
      <Seg label="Discount type" value={line.discountType} options={[['fixed', sym], ['percent', '%']]} onChange={(t) => set(line.discount, t)} />
      <button type="button" onClick={() => setOpen(false)} className="px-1 text-[12px] font-semibold text-muted-foreground">Done</button>
    </span>
  )
}

export function CartLineRow({ line, sym }: { line: CartLine; sym: string }) {
  const { dispatch } = useTill()
  const opts = useQuery(priceOptionsQuery).data
  const [text, setText] = useState(qtyText(line.qty))
  useEffect(() => setText(qtyText(line.qty)), [line.qty])
  const setQty = (qty: number) => dispatch({ type: 'setQty', key: line.key, qty, lots: lotsFor(opts, line.productId) })
  const gross = lineGross(line)
  const disc = lineDiscount(line)
  const expired = line.batchLabel.startsWith('exp ') && line.batchLabel.slice(4) < new Date().toLocaleDateString('en-CA')
  return (
    <motion.div initial={{ backgroundColor: 'var(--brand-soft)' }} animate={{ backgroundColor: 'rgba(0,0,0,0)' }} transition={{ duration: 0.7 }}
      className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-x-3 gap-y-1.5 rounded-[var(--radius-sm)] border-t border-dashed border-border px-2 py-2.5 first:border-t-0">
      <div className="text-[14px] font-semibold leading-snug">{line.description || line.name}</div>
      <div className="flex items-center gap-0.5 rounded-full border border-border bg-muted p-0.5">
        <button type="button" aria-label={`Fewer ${line.name}`} onClick={() => setQty(line.qty - 1)} className="grid h-7 w-7 place-items-center rounded-full hover:bg-border"><Minus size={14} /></button>
        <input aria-label={`Quantity of ${line.name}`} inputMode="decimal" autoComplete="off" value={text}
          onChange={(e) => setText(e.target.value.replace(/[^\d.]/g, ''))} onBlur={() => setQty(num(text))}
          onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
          className="w-10 bg-transparent text-center text-[14px] font-bold tabular-nums outline-none" />
        <button type="button" aria-label={`More ${line.name}`} onClick={() => setQty(line.qty + 1)} className="grid h-7 w-7 place-items-center rounded-full hover:bg-border"><Plus size={14} /></button>
      </div>
      <div className="min-w-[84px] text-right text-[14.5px] font-bold tabular-nums">
        {disc > 0 && <s className="block text-[11.5px] font-medium text-faint">{money(sym, gross)}</s>}
        {money(sym, gross - disc)}
      </div>
      <div className="col-span-full flex flex-wrap items-center gap-1.5 text-[12px] text-muted-foreground">
        <span className="tabular-nums">{money(sym, line.unitPrice)} each</span>
        {line.batchLabel && (
          <span className={cn('rounded-md px-1.5 font-semibold', expired ? 'bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger' : 'bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] text-warning')}>
            {line.batchLabel}
          </span>
        )}
        {line.warrantyMonths > 0 && <span className="rounded-md bg-muted px-1.5">{line.warrantyMonths} mo warranty</span>}
        {line.allowOversell && <span className="font-semibold text-warning">found at till</span>}
        <DiscountChip line={line} sym={sym} />
        <button type="button" aria-label={`Remove ${line.name}`} onClick={() => dispatch({ type: 'remove', key: line.key })}
          className="ml-auto grid place-items-center p-0.5 text-faint hover:text-danger"><X size={15} /></button>
      </div>
      {line.trackSerial && line.serials.length > 0 && (
        <div className="col-span-full grid grid-cols-2 gap-1.5">
          {line.serials.map((s, i) => (
            <input key={i} aria-label={`Serial ${i + 1} for ${line.name}`} placeholder={`Serial ${i + 1}`} autoComplete="off" value={s}
              onChange={(e) => dispatch({ type: 'serial', key: line.key, index: i, value: e.target.value })} className={cn(FIELD, 'h-8 text-[13px]')} />
          ))}
        </div>
      )}
    </motion.div>
  )
}
```

- [ ] **Step 2: Which price? and suggestions.** Create `web/src/features/till/cart-ui/PricePick.tsx`:

```tsx
import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useEffect, useState, type KeyboardEvent } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { Lot, TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, errMsg, priceOptionsQuery } from '../api'
import { Kbd } from '../bits'
import { lotExpired, lotLabel } from '../cart/lines'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

/** The cashier reads the sticker: pick the lot (1–9, Enter = the FEFO one). An
 * expired lot asks again: sell anyway / write it off / back. */
export function PricePick({ product, lots: cached }: { product: TillProduct; lots: Lot[] }) {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const { dispatch } = useTill()
  const qc = useQueryClient()
  const [lots, setLots] = useState(cached)
  const [expired, setExpired] = useState<Lot | null>(null)

  function commit(lot: Lot, all: Lot[] = lots) {
    ui.close()
    dispatch({ type: 'add', product, lot, lots: all })
    if (product.suggestion) ui.open({ kind: 'suggestion', product })
  }
  const pick = (lot: Lot) => (lotExpired(lot) ? setExpired(lot) : commit(lot))

  // Show the cached lots at once (a rush can't wait), then correct from the server
  // so nobody picks a lot another lane just emptied.
  useEffect(() => {
    let live = true
    api<Lot[]>(`/till/products/${product.id}/lots`).then((fresh) => {
      const f = fresh.filter((l) => num(l.qtyRemaining) > 0)
      if (!live || !f.length) return
      if (new Set(f.map((l) => l.price)).size === 1 && !lotExpired(f[0])) commit(f[0], f)
      else setLots(f)
    }).catch(() => undefined)
    return () => { live = false }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [product.id])

  async function writeOff() {
    if (!expired) return
    try {
      await api('/till/write-off', { json: { productId: product.id, batchId: expired.batchId, quantity: expired.qtyRemaining, note: `expired ${expired.expiryDate}` } })
      toast.success('Expired stock written off')
      ui.close()
      void qc.invalidateQueries({ queryKey: priceOptionsQuery.queryKey })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  function onKey(e: KeyboardEvent) {
    if (expired) return // buttons handle it; Esc goes back via onOpenChange
    if (e.key === 'Enter') {
      e.preventDefault()
      pick(lots[0])
      return
    }
    const n = Number.parseInt(e.key, 10)
    if (n >= 1 && n <= lots.length) {
      e.preventDefault()
      pick(lots[n - 1])
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && (expired ? setExpired(null) : ui.close())}>
      <DialogContent onKeyDown={onKey} className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>{expired ? 'This lot is expired' : 'Which price?'}</DialogTitle>
          <DialogDescription>
            {expired
              ? `${product.name} — ${lotLabel(expired)}. Sell it anyway, or write it off instead?`
              : `${product.name} has lots at different prices or an expired lot. Read the sticker on the package.`}
          </DialogDescription>
        </DialogHeader>
        {!expired ? (
          <ol className="flex flex-col gap-2">
            {lots.map((l, i) => (
              <li key={l.batchId}>
                <button type="button" onClick={() => pick(l)}
                  className={cn('flex w-full items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-muted p-3 text-left hover:border-brand', lotExpired(l) && 'border-danger')}>
                  <Kbd>{i + 1}</Kbd>
                  <span className="flex-1">
                    <span className="font-display text-lg font-bold tabular-nums">{money(cfg.symbol, l.price)}</span>
                    <span className="ml-2 text-[13px] text-muted-foreground">{lotLabel(l)} · {qtyText(l.qtyRemaining)} left</span>
                  </span>
                  {lotExpired(l) && <span className="rounded-md bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] px-1.5 text-[11px] font-bold text-danger">Expired</span>}
                </button>
              </li>
            ))}
          </ol>
        ) : (
          <DialogFooter>
            <Button variant="ghost" onClick={() => setExpired(null)}>Back</Button>
            <Button variant="destructive" onClick={writeOff}>Write it off</Button>
            <Button autoFocus onClick={() => commit(expired)}>Sell anyway</Button>
          </DialogFooter>
        )}
      </DialogContent>
    </Dialog>
  )
}
```

Create `web/src/features/till/cart-ui/Suggestion.tsx`:

```tsx
import { useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import type { TillProduct } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, num } from '@/lib/money'
import { configQuery } from '../api'
import { unitPriceFor } from '../cart/lines'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

/** A plugin-proposed markdown (e.g. a clearance price), offered once per new line. */
export function Suggestion({ product }: { product: TillProduct }) {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const ui = useTillUi()
  const [key] = useState(state.lastAddedKey)
  const s = product.suggestion
  if (!s) return null
  const pct = s.discountType === 'percent'
  const unit = unitPriceFor(product, state.saleType)
  const next = Math.max(0, pct ? unit * (1 - num(s.discountValue) / 100) : unit - num(s.discountValue))
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>{s.label || 'Suggested discount'}</DialogTitle>
          <DialogDescription>{s.prompt || 'Apply the suggested discount?'}</DialogDescription>
        </DialogHeader>
        <p className="text-center text-[15px] tabular-nums">
          <s className="text-faint">{money(cfg.symbol, unit)}</s> → <b className="font-display text-2xl">{money(cfg.symbol, next)}</b>
        </p>
        <DialogFooter>
          <Button variant="ghost" onClick={ui.close}>Skip</Button>
          <Button autoFocus onClick={() => {
            dispatch({ type: 'lineDiscount', key, discount: s.discountValue, discountType: pct ? 'percent' : 'fixed' })
            ui.close()
          }}>Apply</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 3: Hold and resume.** Create `web/src/features/till/cart-ui/useHold.ts`:

```ts
import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useCallback } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { HeldSale } from '@/api/types.gen'
import { dec, num } from '@/lib/money'
import { configQuery, customersQuery, errMsg, heldQuery } from '../api'
import { toHeldBlob } from '../cart/held'
import { saleTotal } from '../cart/totals'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'

export function useHold() {
  const { state, dispatch } = useTill()
  const cfg = useSuspenseQuery(configQuery).data
  const customers = useQuery(customersQuery).data ?? []
  const qc = useQueryClient()
  return useCallback(async () => {
    if (!state.lines.length) return void toast.error('Cart is empty')
    const label = customers.find((c) => c.id === state.customerId)?.name ?? state.lines[0].name
    try {
      await api('/till/held', {
        json: {
          label, saleType: state.saleType, customerId: state.customerId ?? undefined,
          discount: String(num(state.billDiscount)), cart: toHeldBlob(state), itemCount: state.lines.length, total: dec(saleTotal(state)),
        },
      })
      dispatch({ type: 'newSale', saleType: cfg.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail' })
      toast.success('Sale held')
      void qc.invalidateQueries({ queryKey: heldQuery.queryKey })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }, [state, customers, cfg.defaultSaleType, dispatch, qc])
}

/** Resume a held sale. Refuses while another sale is in the cart (karots-pos silently
 * replaced it — that lost a live sale; deviation flagged in the commit). */
export function useResume() {
  const { state, dispatch } = useTill()
  const ui = useTillUi()
  const qc = useQueryClient()
  return useCallback(async (h: HeldSale) => {
    if (state.lines.length) return void toast.error('Finish or hold the current sale first')
    dispatch({ type: 'resume', held: h })
    ui.close()
    await api(`/till/held/${h.id}`, { method: 'DELETE' }).catch(() => undefined)
    void qc.invalidateQueries({ queryKey: heldQuery.queryKey })
  }, [state.lines.length, dispatch, ui, qc])
}
```

Create `web/src/features/till/cart-ui/HeldSales.tsx`:

```tsx
import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { PauseCircle, X } from 'lucide-react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money } from '@/lib/money'
import { configQuery, errMsg, heldQuery } from '../api'
import { useTillUi } from '../TillUi'
import { useResume } from './useHold'

/** The strip under the cart: held sales as chips (tap to resume). */
export function HeldStrip() {
  const cfg = useSuspenseQuery(configQuery).data
  const held = useQuery(heldQuery).data ?? []
  const resume = useResume()
  const ui = useTillUi()
  if (!held.length) return null
  return (
    <div className="flex items-center gap-2 overflow-hidden border-t border-border px-3.5 py-2.5 text-[12.5px] text-muted-foreground">
      <PauseCircle size={16} className="shrink-0" /> Held
      {held.slice(0, 3).map((h) => (
        <button key={h.id} type="button" onClick={() => void resume(h)}
          className="truncate rounded-full border border-border bg-muted px-2.5 py-1 text-[12px] font-semibold text-foreground">
          {h.label || 'Sale'} · {money(cfg.symbol, h.total)}
        </button>
      ))}
      {held.length > 3 && <button type="button" className="font-semibold underline-offset-2 hover:underline" onClick={() => ui.open({ kind: 'held' })}>+{held.length - 3} more</button>}
    </div>
  )
}

export function HeldSales() {
  const cfg = useSuspenseQuery(configQuery).data
  const held = useQuery(heldQuery).data ?? []
  const resume = useResume()
  const ui = useTillUi()
  const qc = useQueryClient()
  const remove = (id: number) =>
    api(`/till/held/${id}`, { method: 'DELETE' })
      .then(() => { toast.success('Held sale removed'); return qc.invalidateQueries({ queryKey: heldQuery.queryKey }) })
      .catch((e) => toast.error(errMsg(e)))
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[520px]">
        <DialogHeader>
          <DialogTitle>Held sales</DialogTitle>
          <DialogDescription>{held.length ? 'Resume one to finish it.' : 'Nothing is held.'}</DialogDescription>
        </DialogHeader>
        <ul className="flex flex-col gap-2">
          {held.map((h) => (
            <li key={h.id} className="flex items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-muted p-3">
              <span className="flex-1">
                <b>{h.label || 'Sale'}</b>
                <small className="block text-muted-foreground">{h.itemCount} lines · {new Date(h.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</small>
              </span>
              <b className="tabular-nums">{money(cfg.symbol, h.total)}</b>
              <Button size="sm" onClick={() => void resume(h)}>Resume</Button>
              <button type="button" aria-label={`Remove held sale ${h.label}`} onClick={() => void remove(h.id)} className="text-faint hover:text-danger"><X size={16} /></button>
            </li>
          ))}
        </ul>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 4: The panel.** Create `web/src/features/till/cart-ui/CartPanel.tsx`:

```tsx
import { useSuspenseQuery } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { qtyText } from '@/lib/money'
import { configQuery } from '../api'
import { useTill } from '../TillProvider'
import { CartLineRow } from './CartLineRow'
import { HeldStrip } from './HeldSales'

export function CartPanel() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state } = useTill()
  const list = useRef<HTMLDivElement>(null)
  const prev = useRef(state.lines.length)
  // Follow growth only: a freshly scanned line below the fold stays in view, but
  // removing a line never yanks the list around.
  useEffect(() => {
    if (state.lines.length > prev.current && list.current) list.current.scrollTop = list.current.scrollHeight
    prev.current = state.lines.length
  }, [state.lines.length])
  const items = state.lines.reduce((s, l) => s + l.qty, 0)
  return (
    <>
      <header className="flex items-center justify-between px-4 pb-2.5 pt-3.5 font-bold">
        <span>Current sale</span>
        <small className="font-medium text-muted-foreground">{qtyText(items)} items</small>
      </header>
      <div ref={list} className="flex min-h-0 flex-1 flex-col overflow-auto px-2.5 pb-2.5">
        {state.lines.length
          ? state.lines.map((l) => <CartLineRow key={l.key} line={l} sym={cfg.symbol} />)
          : <p className="m-auto p-8 text-center text-faint">Cart is empty. Scan or tap a product to start.</p>}
      </div>
      <HeldStrip />
    </>
  )
}
```

- [ ] **Step 5: Wire it in.**
  - `TillDialogs.tsx`: add these cases:
    - `case 'pricePick': return <PricePick product={d.product} lots={d.lots} />`
    - `case 'suggestion': return <Suggestion product={d.product} />`
    - `case 'held': return <HeldSales />`
  - `TillScreen.tsx`: replace the cart `<section>` with:

```tsx
      <section aria-label="Cart" className={cn(PANEL, 'flex min-h-0 flex-col [grid-area:cart]')}>
        <CartPanel />
      </section>
```

  - `DrawerActions.tsx`: add Hold and Held as the first two buttons of the toolbar. Use `const hold = useHold()` and `const held = useQuery(heldQuery).data ?? []`. The new imports are `useQuery` (from `@tanstack/react-query`), `heldQuery` (from `../api`), `Kbd` (from `../bits`) and `useHold` (from `../cart-ui/useHold`).

```tsx
      <button type="button" className={CHIP} onClick={() => void hold()}>Hold <Kbd>F4</Kbd></button>
      <button type="button" className={CHIP} onClick={() => ui.open({ kind: 'held' })}>Held ({held.length})</button>
```

- [ ] **Step 6: Check.**

Run: `cd web && bunx tsc -b && bun run lint && bun run test`
Expected: PASS.

- [ ] **Step 7: UI GATE 2 — catalogue and cart.**
  1. `make build`, start the binary, and run `make e2e-seed`.
  2. Run `SHOTS=gate2 SHOTS_SCENE=sale bunx playwright test e2e/till-shots.spec.ts`. The pay panel is still empty. That is expected at this gate.
  3. With the Playwright MCP browser, in all four palette × mode combinations, capture:
     - the **Which price?** dialog (Lot Soap)
     - the **expired lot** confirm (Yoghurt)
     - a line with the discount editor open
     - a serial-tracked line, if you add one via the admin DB or a quick SQL update of `track_serial` on E2E Milk
     - the held strip with two held sales
     - ⓘ info
     - Quick item
  4. **Stop.** Show the screenshots and wait for approval.

- [ ] **Step 8: Commit** after approval.

```bash
git add web
git commit -m "feat(web): till cart — qty stepper, line discounts, lot labels + spill, serials, which-price / expired-lot prompt (sell anyway / write off / back), clearance suggestion, hold/resume"
```

---

### Task 12: Pay panel and the checkout engine (UI GATE 3)

**Files:**
- Create:
  - `web/src/features/till/checkout/useCheckout.ts`, `useCheckout.test.tsx`
  - `web/src/features/till/checkout/CheckoutProvider.tsx`, `Prompts.tsx`
  - in `web/src/features/till/pay/`:
    - `PayPanel.tsx`, `CustomerPicker.tsx`, `AddCustomer.tsx`
    - `Tender.tsx`, `Summary.tsx`, `ChangeBanner.tsx`
- Modify: `web/src/features/till/TillScreen.tsx`, `web/src/features/till/TillDialogs.tsx`

**Interfaces:**
- Consumes:
  - from Task 7: `checkoutGate`, `buildSaleRequest`, `effectivePayments`
  - from Task 5: the totals functions
  - from Task 8: `printSale`, `slots.tillBeforeCheckout`, `slots.tillAfterSale`, `slots.tillTenders`
  - from Task 11: `useHold`
- Produces:
  - `useCheckoutCtx(): { run(s?, confirmed?), answer(action|null, confirmed), dismiss(), newSale(), busy, prompt, consumable, done: SaleResult | null }`. Task 13 uses `done` and `newSale`.

- [ ] **Step 1: Write the failing test** (Review Focus 4). Create `web/src/features/till/checkout/useCheckout.test.tsx`:

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DrawerState, SaleResult, TillConfig } from '@/api/types.gen'
import { configQuery, customersQuery, drawerQuery } from '../api'
import { TillProvider, useTill } from '../TillProvider'
import { useCheckout } from './useCheckout'

const cfg: TillConfig = { symbol: 'Rs.', defaultSaleType: 'retail', askToPrint: false, openCashDrawer: false, allowUntrackedCash: true,
  defaultLockerId: 0, allowContinueLast: true, canManageCredit: false, canSeeCost: false, canChangePin: true, isManager: false, tenderMethods: [] }
const drawer: DrawerState = { summary: { cashSales: '0', payIns: '0', withdrawals: '0', expected: '0' }, denominations: [], lockers: [] }
const sale: SaleResult = { id: 42, receiptNo: 'S-1', total: '100.00', paidAmount: '150.00', changeGiven: '50.00', roundingToAccount: '0.00', customerName: '', methods: ['cash'] }

function setup() {
  const qc = new QueryClient()
  qc.setQueryData(configQuery.queryKey, cfg)
  qc.setQueryData(drawerQuery.queryKey, drawer)
  qc.setQueryData(customersQuery.queryKey, [])
  const wrapper = ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}><TillProvider userId={1} saleType="retail">{children}</TillProvider></QueryClientProvider>
  )
  return renderHook(() => ({ till: useTill(), co: useCheckout() }), { wrapper })
}

describe('useCheckout', () => {
  let calls: string[]
  beforeEach(() => {
    sessionStorage.clear()
    calls = []
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      calls.push(url)
      await new Promise((r) => setTimeout(r, 20))
      return new Response(url.endsWith('/till/sales') ? JSON.stringify(sale) : null, { status: url.endsWith('/till/sales') ? 201 : 204 })
    }))
  })
  afterEach(() => vi.unstubAllGlobals())

  it('a double Enter / double click posts exactly one sale', async () => {
    const { result } = setup()
    act(() => result.current.till.dispatch({ type: 'addService', line: { productId: 1, name: 'x', price: 100 } }))
    await act(async () => {
      await Promise.all([result.current.co.run(), result.current.co.run()])
    })
    expect(calls.filter((u) => u.endsWith('/till/sales'))).toHaveLength(1)
  })

  it('auto-print mode: prints with the drawer kick merged, clears the cart, shows the change', async () => {
    const { result } = setup()
    act(() => result.current.till.dispatch({ type: 'addService', line: { productId: 1, name: 'x', price: 100 } }))
    await act(async () => { await result.current.co.run() })
    expect(calls.some((u) => u.endsWith('/till/sales/42/print?kick=1'))).toBe(true)
    expect(result.current.till.state.lines).toEqual([])
    expect(result.current.till.state.postSaleChange).toBe(50)
  })
})
```

Run: `cd web && bunx vitest run src/features/till/checkout`
Expected: FAIL (the module is missing).

- [ ] **Step 2: The engine.** Create `web/src/features/till/checkout/useCheckout.ts`:

```ts
import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useCallback, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { SaleResult } from '@/api/types.gen'
import { num } from '@/lib/money'
import { slots } from '@/plugins/slots'
import { configQuery, customersQuery, drawerQuery, errMsg, lockersOf, useRefreshTill } from '../api'
import { buildSaleRequest, checkoutGate, type Prompt } from '../cart/checkout'
import { tillReducer, type Action } from '../cart/store'
import type { SaleType, TillState } from '../cart/types'
import { printSale } from '../print'
import { useTill } from '../TillProvider'

export function useCheckout() {
  const { state, dispatch } = useTill()
  const cfg = useSuspenseQuery(configQuery).data
  const drawer = useQuery(drawerQuery).data
  const customers = useQuery(customersQuery).data
  const refresh = useRefreshTill()
  const [prompt, setPrompt] = useState<Prompt | null>(null)
  const [consumable, setConsumable] = useState(false)
  const [done, setDone] = useState<SaleResult | null>(null)
  const [busy, setBusy] = useState(false)
  const inFlight = useRef(false)
  const saleType: SaleType = cfg.defaultSaleType === 'wholesale' ? 'wholesale' : 'retail'

  /** Complete the sale. Re-entered with the next state after each prompt answer. */
  const run = useCallback(async (s: TillState = state, confirmed = false) => {
    if (inFlight.current) return
    const customer = (customers ?? []).find((c) => c.id === s.customerId) ?? null
    const gate = checkoutGate(s, {
      customer, canManageCredit: cfg.canManageCredit, confirmed,
      accountKinds: { card: lockersOf(drawer, 'card').length > 0, online: lockersOf(drawer, 'bank').length > 0 },
    })
    if ('prompt' in gate) return setPrompt(gate.prompt)
    if ('error' in gate) return void toast.error(gate.error)
    inFlight.current = true
    setBusy(true)
    try {
      for (const guard of slots.tillBeforeCheckout) {
        const msg = await guard(s.lines)
        if (msg) return void toast.error(msg)
      }
      const res = await api<SaleResult>('/till/sales', { json: buildSaleRequest(s, gate.payments, { deferDrawerKick: !cfg.askToPrint }) })
      void Promise.allSettled(slots.tillAfterSale.map((fn) => fn({ saleId: res.id, lines: s.lines })))
      toast.success('Sale complete')
      if (cfg.askToPrint) {
        setDone(res)
      } else {
        // No modal: print in the background (drawer pulse merged into the ONE job)
        // and reset straight away; the change survives as a banner.
        void printSale(res.id, true)
        dispatch({ type: 'newSale', saleType, change: num(res.changeGiven) })
      }
      void refresh()
    } catch (e) {
      // A job's material reads short: offer found-at-till once per unapproved line.
      if (e instanceof ApiError && e.code === 'CONSUMABLE_SHORT' && s.lines.some((l) => (l.isService || l.components.length > 0) && !l.allowOversell)) {
        setConsumable(true)
        return
      }
      toast.error(errMsg(e, 'Sale failed'))
    } finally {
      inFlight.current = false
      setBusy(false)
    }
  }, [state, customers, cfg, drawer, dispatch, refresh, saleType])

  /** Resolve a prompt: apply the action (if any) and re-run on the NEXT state. */
  const answer = useCallback((action: Action | null, confirmed: boolean) => {
    const next = action ? tillReducer(state, action) : state
    if (action) dispatch(action)
    setPrompt(null)
    setConsumable(false)
    void run(next, confirmed)
  }, [state, dispatch, run])

  const dismiss = useCallback(() => {
    setPrompt(null)
    setConsumable(false)
  }, [])

  const newSale = useCallback(() => {
    setDone(null)
    dispatch({ type: 'newSale', saleType })
    document.getElementById('till-search')?.focus()
  }, [dispatch, saleType])

  return { run, answer, dismiss, newSale, busy, prompt, consumable, done }
}
```

Create `web/src/features/till/checkout/CheckoutProvider.tsx`:

```tsx
import { createContext, useContext, type ReactNode } from 'react'
import { useCheckout } from './useCheckout'

type Co = ReturnType<typeof useCheckout>
const CoCtx = createContext<Co | null>(null)

export function CheckoutProvider({ children }: { children: ReactNode }) {
  return <CoCtx.Provider value={useCheckout()}>{children}</CoCtx.Provider>
}

// eslint-disable-next-line react-refresh/only-export-components
export function useCheckoutCtx() {
  const v = useContext(CoCtx)
  if (!v) throw new Error('useCheckoutCtx must be used inside <CheckoutProvider>')
  return v
}
```

- [ ] **Step 3: Prompts.** Create `web/src/features/till/checkout/Prompts.tsx`:

```tsx
import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { dec, money, qtyText } from '@/lib/money'
import { configQuery, customersQuery } from '../api'
import { blankDraft } from '../cart/store'
import { useTill } from '../TillProvider'
import { useCheckoutCtx } from './CheckoutProvider'

function Confirm({ title, body, yes, no = 'Cancel', onYes, onNo }: {
  title: string
  body: ReactNode
  yes: string
  no?: string
  onYes: () => void
  onNo?: () => void
}) {
  return (
    <Dialog open onOpenChange={(o) => !o && (onNo ?? onYes)()}>
      <DialogContent className="sm:max-w-[440px]">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{body}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          {onNo && <Button variant="ghost" onClick={onNo}>{no}</Button>}
          <Button autoFocus onClick={onYes}>{yes}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Styled in-app confirms (never the browser's confirm()), one at a time. */
export function Prompts() {
  const co = useCheckoutCtx()
  const cfg = useSuspenseQuery(configQuery).data
  const { state } = useTill()
  const name = (useQuery(customersQuery).data ?? []).find((c) => c.id === state.customerId)?.name ?? 'the customer'
  const sym = cfg.symbol
  if (co.consumable) {
    return <Confirm title="Material reads short" body="A job’s material (paper, ink…) reads short on the count. Sell anyway and correct the count up?"
      yes="Sell anyway" onYes={() => co.answer({ type: 'approveConsumables' }, false)} onNo={co.dismiss} />
  }
  const p = co.prompt
  if (!p) return null
  switch (p.kind) {
    case 'oversell':
      return <Confirm title="Sell more than the count shows?"
        body={`The count shows ${qtyText(p.stock)} of ${p.name}; you are selling ${qtyText(p.qty)}. If the customer is holding it, the count is corrected up.`}
        yes={`Sell ${qtyText(p.qty)}`} onYes={() => co.answer({ type: 'approveOversell', key: p.key }, false)} onNo={co.dismiss} />
    case 'shortfall':
      return p.hasCustomer
        ? <Confirm title={`${money(sym, p.amount)} short`} body={`Put ${money(sym, p.amount)} on ${name}’s account?`} yes="Put on account" no="Take more money"
            onYes={() => co.answer({ type: 'addTender', payment: { ...blankDraft('credit'), amount: dec(p.amount) } }, true)} onNo={co.dismiss} />
        : <Confirm title={`${money(sym, p.amount)} short`} body="Take more money, or pick a customer to sell on account." yes="OK" onYes={co.dismiss} />
    case 'overcovered':
      return <Confirm title="On account covers more than the bill"
        body={`The on-account line (${money(sym, p.amount)}) plus the money taken is more than the total. Remove the on-account line?`}
        yes="Remove on-account" onYes={() => co.answer({ type: 'removeCreditTenders' }, true)} onNo={co.dismiss} />
    case 'overlimit':
      return <Confirm title="Over the credit limit"
        body={`This puts ${money(sym, p.amount)} on account — ${money(sym, p.over)} over ${name}’s available credit. Approve this one sale?`}
        yes="Approve" onYes={() => co.answer({ type: 'approveOverLimit' }, true)} onNo={co.dismiss} />
  }
}
```

- [ ] **Step 4: Customer.** Create `web/src/features/till/pay/CustomerPicker.tsx`:

```tsx
import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ChevronDown, User, UserPlus } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { TillCustomer } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, customersQuery, errMsg } from '../api'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'

const initials = (n: string) => n.split(/\s+/).slice(0, 2).map((w) => w[0]?.toUpperCase() ?? '').join('')

export function balanceText(sym: string, c: TillCustomer) {
  const o = num(c.outstanding)
  return o > 0 ? `owes ${money(sym, o)}` : o < 0 ? `advance ${money(sym, -o)}` : 'nothing owed'
}

export function CustomerPicker() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const ui = useTillUi()
  const qc = useQueryClient()
  const customers = useQuery(customersQuery).data ?? []
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const [limit, setLimit] = useState<string | null>(null)
  const box = useRef<HTMLDivElement>(null)
  const cur = customers.find((c) => c.id === state.customerId) ?? null
  const list = useMemo(() => {
    const s = q.trim().toLowerCase()
    return (s ? customers.filter((c) => c.name.toLowerCase().includes(s) || c.phone.includes(s)) : customers).slice(0, 50)
  }, [customers, q])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => box.current && !box.current.contains(e.target as Node) && setOpen(false)
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  const pick = (id: number | null) => {
    dispatch({ type: 'customer', id })
    setOpen(false)
    setQ('')
  }
  async function saveLimit() {
    if (!cur || limit === null || limit.trim() === '' || num(limit) < 0) return void toast.error('Enter a valid credit limit')
    try {
      await api(`/till/customers/${cur.id}/credit-limit`, { method: 'PATCH', json: { creditLimit: limit.trim() } })
      await qc.invalidateQueries({ queryKey: customersQuery.queryKey })
      setLimit(null)
      toast.success('Credit limit updated')
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  return (
    <div ref={box} className="relative">
      <div className="flex gap-2">
        <button type="button" aria-haspopup="listbox" aria-expanded={open} onClick={() => setOpen(!open)}
          className="flex min-w-0 flex-1 items-center gap-2.5 rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2 text-left">
          <span className="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-brand-soft text-[13px] font-extrabold">
            {cur ? initials(cur.name) : <User size={16} />}
          </span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-[14px] font-bold">{cur?.name ?? 'Walk-in customer'}</span>
            <span className="block truncate text-[12px] text-muted-foreground">
              {cur ? `${cur.phone} · ${balanceText(cfg.symbol, cur)} · limit ${money(cfg.symbol, cur.creditLimit)}` : 'Cash sale, no account'}
            </span>
          </span>
          <ChevronDown size={16} className="text-faint" />
        </button>
        <button type="button" aria-label="Add customer" onClick={() => ui.open({ kind: 'addCustomer' })}
          className="grid w-11 place-items-center rounded-[var(--radius-sm)] border border-border bg-muted"><UserPlus size={17} /></button>
      </div>
      {open && (
        <div className="absolute inset-x-0 top-[calc(100%+6px)] z-30 rounded-[var(--radius-sm)] border border-border bg-popover p-1.5 shadow-[var(--shadow-float)]">
          <input autoFocus aria-label="Search customers" placeholder="Name or phone" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') setOpen(false)
              if (e.key === 'Enter' && list[0]) pick(list[0].id)
            }} className={cn(FIELD, 'mb-1 h-9 w-full')} />
          <div role="listbox" aria-label="Customers" className="max-h-72 overflow-auto">
            <button type="button" role="option" aria-selected={!cur} onClick={() => pick(null)} className="w-full rounded-lg px-2.5 py-2 text-left text-[13.5px] hover:bg-muted">Walk-in</button>
            {list.map((c) => (
              <button key={c.id} type="button" role="option" aria-selected={c.id === cur?.id} onClick={() => pick(c.id)}
                className="flex w-full items-center justify-between gap-2 rounded-lg px-2.5 py-2 text-left text-[13.5px] hover:bg-muted">
                <span>{c.name} <small className="text-muted-foreground">{c.phone}</small></span>
                <small className="text-muted-foreground">{balanceText(cfg.symbol, c)}</small>
              </button>
            ))}
          </div>
        </div>
      )}
      {cur && cfg.canManageCredit && (limit === null ? (
        <button type="button" onClick={() => setLimit(num(cur.creditLimit) ? String(num(cur.creditLimit)) : '')}
          className="mt-1 text-[12px] font-semibold text-muted-foreground underline-offset-2 hover:underline">Adjust credit limit</button>
      ) : (
        <div className="mt-1.5 flex gap-1.5">
          <input autoFocus aria-label="New credit limit" inputMode="decimal" autoComplete="off" value={limit}
            onChange={(e) => setLimit(e.target.value.replace(/[^\d.]/g, ''))} onKeyDown={(e) => e.key === 'Enter' && void saveLimit()}
            className={cn(FIELD, 'h-8 min-w-0 flex-1 text-[13px]')} />
          <Button size="sm" onClick={() => void saveLimit()}>Save</Button>
          <Button size="sm" variant="ghost" onClick={() => setLimit(null)}>Cancel</Button>
        </div>
      ))}
    </div>
  )
}
```

Create `web/src/features/till/pay/AddCustomer.tsx`:

```tsx
import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { CustomerCreated } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import { customersQuery, errMsg } from '../api'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'

export function AddCustomer() {
  const ui = useTillUi()
  const { dispatch } = useTill()
  const qc = useQueryClient()
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [limit, setLimit] = useState('')
  async function submit() {
    if (!name.trim()) return void toast.error('Enter a customer name')
    if (!phone.trim()) return void toast.error('Enter a phone number') // the key that stops duplicate credit accounts
    try {
      const r = await api<CustomerCreated>('/till/customers', { json: { name: name.trim(), phone: phone.trim(), creditLimit: limit || '0' } })
      await qc.invalidateQueries({ queryKey: customersQuery.queryKey })
      dispatch({ type: 'customer', id: r.customer.id })
      toast.success(r.existed ? 'Customer already exists — using them' : 'Customer added')
      ui.close()
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>Add customer</DialogTitle>
          <DialogDescription>Phone is required — it stops the same person getting two accounts.</DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-3" autoComplete="off" onSubmit={(e) => { e.preventDefault(); void submit() }}>
          <input autoFocus aria-label="Customer name" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} className={FIELD} />
          <input aria-label="Customer phone" placeholder="Phone" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} className={FIELD} />
          <input aria-label="Credit limit" placeholder="Credit limit (optional)" inputMode="decimal" value={limit}
            onChange={(e) => setLimit(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'tabular-nums')} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={ui.close}>Cancel</Button>
            <Button type="submit">Add customer</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
```

- [ ] **Step 5: Tender, summary, banner.** Create `web/src/features/till/pay/Tender.tsx`:

```tsx
import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Banknote, BookUser, CreditCard, Smartphone, Wallet, X } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { toast } from 'sonner'
import { dec, money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { Button } from '@/components/ui/button'
import { configQuery, drawerQuery, lockersOf } from '../api'
import { blankDraft } from '../cart/store'
import { balanceDue, quickCash, saleTotal } from '../cart/totals'
import { useCheckoutCtx } from '../checkout/CheckoutProvider'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'

export const METHOD_LABEL: Record<string, string> = { cash: 'Cash', card: 'Card', online: 'Online', credit: 'On account' }

export function Tender() {
  const cfg = useSuspenseQuery(configQuery).data
  const drawer = useQuery(drawerQuery).data
  const { state, dispatch } = useTill()
  const co = useCheckoutCtx()
  const card = lockersOf(drawer, 'card')
  const bank = lockersOf(drawer, 'bank')
  const d = state.draft
  const due = balanceDue(saleTotal(state), state.tenders)
  // Card/online buttons appear only once the owner opened a cashier-accessible card/bank account.
  const methods = [
    { value: 'cash', label: 'Cash', Icon: Banknote },
    ...(card.length ? [{ value: 'card', label: 'Card', Icon: CreditCard }] : []),
    ...(bank.length ? [{ value: 'online', label: 'Online', Icon: Smartphone }] : []),
    { value: 'credit', label: 'On account', Icon: BookUser },
    ...cfg.tenderMethods.map((m) => ({ value: m.value, label: m.label, Icon: Wallet })),
  ]
  const accounts = d.method === 'card' ? card : d.method === 'online' ? bank : []
  const PluginDetail = slots.tillTenders.find((t) => t.value === d.method)?.Detail
  const label = (m: string) => METHOD_LABEL[m] ?? cfg.tenderMethods.find((t) => t.value === m)?.label ?? m

  function choose(m: string) {
    const first = m === 'card' ? card[0] : m === 'online' ? bank[0] : undefined
    dispatch({ type: 'draft', patch: { method: m, accountId: first ? String(first.id) : '', meta: {} } })
    requestAnimationFrame(() => document.getElementById('till-amount')?.focus())
  }
  const commit = () => (d.method ? dispatch({ type: 'commitDraft' }) : void toast.error('Choose a payment method'))

  return (
    <div className="flex flex-col gap-2.5">
      <div className="grid grid-cols-4 gap-1.5" role="group" aria-label="Payment method">
        {methods.map(({ value, label: text, Icon }) => {
          const needsCustomer = value === 'credit' && state.customerId === null
          return (
            <button key={value} type="button" aria-pressed={d.method === value} disabled={needsCustomer}
              title={needsCustomer ? 'Pick a customer to sell on account' : undefined} onClick={() => choose(value)}
              className="flex flex-col items-center gap-1 rounded-[var(--radius-sm)] border border-border bg-muted px-1 py-2 text-[12px] font-bold disabled:opacity-40 aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground">
              <Icon size={18} />{text}
            </button>
          )
        })}
      </div>
      {accounts.length > 1 && (
        <select aria-label="Account" value={d.accountId} onChange={(e) => dispatch({ type: 'draft', patch: { accountId: e.target.value } })} className={cn(FIELD, 'appearance-auto')}>
          {accounts.map((a) => <option key={a.id} value={a.id}>{a.name}</option>)}
        </select>
      )}
      {(d.method === 'card' || d.method === 'online') && (
        <input aria-label="Reference" placeholder="Reference (optional)" autoComplete="off" value={d.reference}
          onChange={(e) => dispatch({ type: 'draft', patch: { reference: e.target.value } })} className={FIELD} />
      )}
      {PluginDetail && <PluginDetail meta={d.meta} setMeta={(meta) => dispatch({ type: 'draft', patch: { meta } })} />}
      <div className="flex gap-1.5">
        <input id="till-amount" aria-label="Amount tendered" inputMode="decimal" autoComplete="off"
          placeholder={due > 0 ? money(cfg.symbol, due) : 'Amount'} value={d.amount}
          onChange={(e) => dispatch({ type: 'draft', patch: { amount: e.target.value.replace(/[^\d.]/g, '') } })}
          onKeyDown={(e) => {
            if (e.key !== 'Enter') return
            e.preventDefault()
            e.stopPropagation()
            if (num(d.amount) > 0) commit()
            else void co.run() // F10 → amount → Enter: an empty box completes the sale
          }}
          className={cn(FIELD, 'min-w-0 flex-1 font-display text-lg font-bold tabular-nums')} />
        <Button variant="secondary" className="h-[var(--control-h)]" title="Fill the remaining balance"
          onClick={() => dispatch({ type: 'draft', patch: { amount: due > 0 ? dec(due) : '' } })}>Fill</Button>
        <Button className="h-[var(--control-h)]" disabled={!(num(d.amount) > 0)} onClick={commit}>Add</Button>
      </div>
      {d.method === 'cash' && due > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {quickCash(due).map((v, i) => (
            <button key={v} type="button" onClick={() => dispatch({ type: 'addTender', payment: { ...blankDraft('cash'), amount: dec(v) } })}
              className="flex-1 whitespace-nowrap rounded-full border border-border px-2 py-1.5 text-[12.5px] font-bold tabular-nums hover:bg-muted">
              {i === 0 ? 'Exact ' : ''}{money(cfg.symbol, v)}
            </button>
          ))}
        </div>
      )}
      <div className="flex flex-col gap-1">
        <AnimatePresence initial={false}>
          {state.tenders.map((p, i) => (
            <motion.div key={`${i}-${p.method}-${p.amount}`} initial={{ y: -4, opacity: 0.3 }} animate={{ y: 0, opacity: 1 }} exit={{ opacity: 0 }} transition={{ duration: 0.25 }}
              className="flex items-center gap-2 rounded-[9px] bg-muted px-2.5 py-1.5 text-[13px]">
              {label(p.method)}
              {p.accountId && <small className="text-muted-foreground">{[...card, ...bank].find((a) => String(a.id) === p.accountId)?.name}</small>}
              {p.reference && <small className="text-muted-foreground">#{p.reference}</small>}
              <b className="ml-auto tabular-nums">{money(cfg.symbol, p.amount)}</b>
              <button type="button" aria-label={`Remove ${label(p.method)} payment`} onClick={() => dispatch({ type: 'removeTender', index: i })}
                className="text-faint hover:text-danger"><X size={14} /></button>
            </motion.div>
          ))}
        </AnimatePresence>
      </div>
    </div>
  )
}
```

Create `web/src/features/till/pay/Summary.tsx`:

```tsx
import { useSuspenseQuery } from '@tanstack/react-query'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery } from '../api'
import { Kbd, Seg } from '../bits'
import { billDiscount, itemDiscountTotal, saleTotal, subtotal, taxTotal } from '../cart/totals'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'

export function Summary() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const sym = cfg.symbol
  const ls = state.lines
  const idisc = itemDiscountTotal(ls)
  const bdisc = billDiscount(ls, state.billDiscount, state.billDiscountType)
  const tax = taxTotal(ls)
  return (
    <div className="flex flex-col gap-1.5 text-[13.5px]">
      <div className="flex justify-between text-muted-foreground"><span>Subtotal</span><span className="tabular-nums">{money(sym, subtotal(ls))}</span></div>
      {idisc > 0 && <div className="flex justify-between text-success"><span>Item discounts</span><span className="tabular-nums">−{money(sym, idisc)}</span></div>}
      <div className="flex items-center justify-between gap-2 text-muted-foreground">
        <span className="flex items-center gap-1.5">
          Bill discount
          <input id="till-bill-discount" aria-label="Bill discount" inputMode="decimal" placeholder="0" autoComplete="off" value={state.billDiscount}
            onChange={(e) => dispatch({ type: 'billDiscount', value: e.target.value.replace(/[^\d.]/g, '') })}
            className={cn(FIELD, 'h-7 w-16 px-2 text-right text-[13px] tabular-nums')} />
          <Seg label="Bill discount type" value={state.billDiscountType} options={[['fixed', sym], ['percent', '%']]}
            onChange={(v) => dispatch({ type: 'billDiscount', discountType: v })} />
          <Kbd>F9</Kbd>
        </span>
        <span className="tabular-nums">{bdisc > 0 ? `−${money(sym, bdisc)}` : '—'}</span>
      </div>
      {tax > 0 && <div className="flex justify-between text-muted-foreground"><span>Tax</span><span className="tabular-nums">{money(sym, tax)}</span></div>}
      <div className="mt-1 flex items-baseline justify-between border-t border-border pt-2">
        <span className="font-semibold text-muted-foreground">Total</span>
        <b className="font-display text-[32px] font-extrabold tracking-tight tabular-nums">{money(sym, saleTotal(state))}</b>
      </div>
    </div>
  )
}
```

Create `web/src/features/till/pay/ChangeBanner.tsx`:

```tsx
import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { dec, money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, customersQuery, drawerQuery } from '../api'
import {
  accountTotal, actualChange, balanceDue, changeDue, coinKeep, keptAmount, notesFor, paidTotal, roundable, saleTotal,
} from '../cart/totals'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'

/** Still to pay / change to give (with Keep) / paid in full — plus account and limit cues. */
export function ChangeBanner() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const drawer = useQuery(drawerQuery).data
  const cust = (useQuery(customersQuery).data ?? []).find((c) => c.id === state.customerId)
  const sym = cfg.symbol
  const t = saleTotal(state)
  const pays = [...state.tenders, ...(num(state.draft.amount) > 0 ? [state.draft] : [])]
  const due = balanceDue(t, pays)
  const change = changeDue(t, pays)
  const canKeep = roundable(t, pays)
  const kept = canKeep ? keptAmount(state.keep, change) : 0
  const give = actualChange(change, kept)
  const onAcct = accountTotal(pays)
  const avail = cust ? num(cust.creditLimit) - num(cust.outstanding) : null
  const over = cust && avail !== null && onAcct > 0.005 && onAcct > avail + 0.005
  const coins = coinKeep(change)
  const notes = notesFor(give, (drawer?.denominations ?? []).map((d) => num(d.value)))
  const keepBtn = 'rounded-full border border-border bg-card px-2.5 py-1 text-[12px] font-bold aria-pressed:border-success aria-pressed:bg-success aria-pressed:text-background'

  let tone = 'bg-muted'
  let body
  if (!state.lines.length) {
    body = <p className="font-bold">Nothing to pay</p>
  } else if (due > 0) {
    tone = 'bg-[color-mix(in_srgb,var(--warning)_16%,transparent)]'
    body = (
      <>
        <p className="flex items-baseline justify-between font-bold">Still to pay <b className="font-display text-[22px] text-warning tabular-nums">{money(sym, due)}</b></p>
        <p className="text-muted-foreground">
          {pays.length ? `Paid ${money(sym, paidTotal(pays) + onAcct)} of ${money(sym, t)} · add another payment to split`
            : state.draft.method === 'cash' ? 'Press Enter for exact cash' : 'Pick how they pay'}
        </p>
      </>
    )
  } else if (change > 0) {
    tone = 'bg-[color-mix(in_srgb,var(--success)_16%,transparent)]'
    body = (
      <>
        <p className="flex items-baseline justify-between font-bold">Change to give <b className="font-display text-[22px] text-success tabular-nums">{money(sym, give)}</b></p>
        {notes.length > 0 && <p className="text-muted-foreground tabular-nums">{notes.map((n) => `${n.qty} × ${qtyText(n.value)}`).join(' · ')}</p>}
        {canKeep && (
          <div className="flex flex-wrap items-center gap-1.5">
            <button type="button" className={keepBtn} aria-pressed={kept === 0} onClick={() => dispatch({ type: 'keep', value: '' })}>Give exact {money(sym, change)}</button>
            {coins > 0 && coins < change && (
              <button type="button" className={keepBtn} aria-pressed={kept === coins} onClick={() => dispatch({ type: 'keep', value: dec(coins) })}>Keep {money(sym, coins)} (no coins)</button>
            )}
            <button type="button" className={keepBtn} aria-pressed={kept >= change - 0.005} onClick={() => dispatch({ type: 'keep', value: dec(change) })}>Keep all</button>
            <input aria-label="Amount to keep" placeholder="Keep…" inputMode="decimal" autoComplete="off" value={state.keep}
              onChange={(e) => dispatch({ type: 'keep', value: e.target.value.replace(/[^\d.]/g, '') })} className={cn(FIELD, 'h-7 w-20 px-2 text-[12px]')} />
          </div>
        )}
        {cust && kept > 0 && (
          <label className="flex items-center gap-2 text-[12.5px]">
            <input type="checkbox" checked={state.roundToAccount} onChange={(e) => dispatch({ type: 'roundToAccount', on: e.target.checked })} />
            Put the kept {money(sym, kept)} on {cust.name}’s account (advance)
          </label>
        )}
      </>
    )
  } else {
    body = <p className="flex items-baseline justify-between font-bold">Paid in full <b className="font-display text-[22px] tabular-nums">{money(sym, paidTotal(pays) + onAcct)}</b></p>
  }

  return (
    <div className={cn('flex flex-col gap-1.5 rounded-[var(--radius-sm)] px-3 py-2.5 text-[13px] transition-colors', tone)} aria-live="polite">
      {body}
      {onAcct > 0 && <p className="font-semibold text-warning">On account: {money(sym, onAcct)}</p>}
      {over && cust && (
        <p className="text-[12px] font-semibold text-danger">
          {cust.name} would owe {money(sym, num(cust.outstanding) + onAcct)}, over the {money(sym, cust.creditLimit)} limit.
          {cfg.canManageCredit ? ' You can approve it when you complete.' : ' A manager must approve.'}
        </p>
      )}
      {drawer?.summary.session && <p className="text-[12px] text-faint">In drawer {money(sym, drawer.summary.expected)}</p>}
    </div>
  )
}
```

- [ ] **Step 6: The panel and wiring.** Create `web/src/features/till/pay/PayPanel.tsx`:

```tsx
import { useSuspenseQuery } from '@tanstack/react-query'
import { Button } from '@/components/ui/button'
import { configQuery } from '../api'
import { Kbd, Seg } from '../bits'
import { useHold } from '../cart-ui/useHold'
import { useCheckoutCtx } from '../checkout/CheckoutProvider'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { ChangeBanner } from './ChangeBanner'
import { CustomerPicker } from './CustomerPicker'
import { Summary } from './Summary'
import { Tender } from './Tender'

export function PayPanel() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const co = useCheckoutCtx()
  const hold = useHold()
  const ui = useTillUi()
  return (
    <div className="flex min-h-0 flex-1 flex-col gap-3 overflow-auto p-3.5">
      <div className="flex items-center justify-between">
        <Seg label="Price list" value={state.saleType} options={[['retail', 'Retail'], ['wholesale', 'Wholesale']]}
          onChange={(v) => dispatch({ type: 'saleType', saleType: v })} />
        <span className="text-[12px] text-faint">{cfg.askToPrint ? 'Asks before printing' : 'Prints automatically'}</span>
      </div>
      <CustomerPicker />
      <Tender />
      <Summary />
      <ChangeBanner />
      <div className="mt-auto flex flex-col gap-2">
        <Button className="h-[54px] text-base" disabled={!state.lines.length || co.busy} onClick={() => void co.run()}>
          {co.busy ? 'Completing…' : 'Complete sale'} <Kbd className="border-current bg-transparent text-current">Enter</Kbd>
        </Button>
        <div className="flex gap-2">
          <Button variant="secondary" className="flex-1" onClick={() => void hold()}>Hold <Kbd>F4</Kbd></Button>
          <Button variant="secondary" className="flex-1" onClick={() => (state.lines.length ? ui.open({ kind: 'clearSale' }) : co.newSale())}>New sale</Button>
        </div>
      </div>
    </div>
  )
}
```

Changes to other files:

- **`TillDialogs.tsx`:** add these cases. `useCheckoutCtx` needs `TillDialogs` to render inside `CheckoutProvider`, which the screen below arranges.
  - `case 'addCustomer': return <AddCustomer />`
  - `case 'clearSale': return <ClearSale />`

  Then define `ClearSale` in the same file. The new imports are:
  - `Button`
  - `Dialog`, `DialogContent`, `DialogDescription`, `DialogFooter`, `DialogHeader` and `DialogTitle` from `@/components/ui/dialog`
  - `useCheckoutCtx` from `./checkout/CheckoutProvider`
  - `AddCustomer` from `./pay/AddCustomer`

```tsx
function ClearSale() {
  const ui = useTillUi()
  const co = useCheckoutCtx()
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>Clear this sale?</DialogTitle>
          <DialogDescription>Every line, the customer and any payments are removed. Hold it instead to finish it later.</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={ui.close}>Keep it</Button>
          <Button variant="destructive" autoFocus onClick={() => { ui.close(); co.newSale() }}>Clear sale</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
```

- **`TillScreen.tsx`:**
  - Wrap everything inside `TillUiProvider` in `<CheckoutProvider>`.
  - Render `<Prompts />` after `<TillDialogs />`.
  - Replace the payment `<section>` with:

```tsx
      <section aria-label="Payment" className={cn(PANEL, 'flex min-h-0 flex-col [grid-area:pay]')}>
        <PayPanel />
      </section>
```

- [ ] **Step 7: Test and check.**

Run: `cd web && bunx vitest run && bunx tsc -b && bun run lint`
Expected: PASS, including `useCheckout.test.tsx`.

- [ ] **Step 8: UI GATE 3 — pay.**
  1. `make build`, start the binary, and run `make e2e-seed`.
  2. Run `SHOTS=gate3 SHOTS_SCENE=sale bunx playwright test e2e/till-shots.spec.ts`.
  3. With the Playwright MCP browser, in all four combinations, capture:
     - a walk-in with Rs 5,000 cash (change plus notes plus Keep chips)
     - a split card + cash
     - a customer with an on-account line over the limit (the red warning)
     - the four prompts: found at till, put on account, over the limit, and the material-short consumable prompt (fake it with the React devtools or skip it if unreachable, and say so)
     - Add customer, and the credit-limit edit
  4. **Stop.** Show the screenshots and wait for approval.

- [ ] **Step 9: Commit** after approval.

```bash
git add web
git commit -m "feat(web): till pay panel + checkout engine — customer picker (+add, limit edit), tenders (cash/card/online/on account/plugin), quick cash, fill, split, keep/round-to-account, bill discount, prompts, single-post guard"
```

---

### Task 13: Sale complete, auto-print, change banner and hotkeys (UI GATE 4)

**Files:**
- Create:
  - `web/src/features/till/checkout/SaleComplete.tsx`
  - `web/src/features/till/checkout/Hotkeys.tsx`
  - `web/src/features/till/cart-ui/PostSaleChange.tsx`
- Modify:
  - `web/src/features/till/cart/types.ts`, `web/src/features/till/cart/store.ts` (the `clearPostSaleChange` action)
  - `web/src/features/till/cart-ui/CartPanel.tsx`
  - `web/src/features/till/TillScreen.tsx`

**Interfaces:**
- Consumes:
  - `useCheckoutCtx().done` and `newSale` (Task 12)
  - `hotkeyFor` (Task 7)
  - `useHold` (Task 11)
  - `printSale` (Task 8)
  - `METHOD_LABEL` and `balanceText` (Task 12)
- Produces: the finished sell screen.

- [ ] **Step 1: Write the failing reducer test.** Append to `web/src/features/till/cart/store.test.ts`:

```ts
describe('post-sale change banner', () => {
  it('clears on demand without touching the (already empty) cart', () => {
    let s = tillReducer(initialState('retail'), { type: 'newSale', saleType: 'retail', change: 25 })
    s = tillReducer(s, { type: 'clearPostSaleChange' })
    expect(s.postSaleChange).toBe(0)
  })
})
```

Run: `cd web && bunx vitest run src/features/till/cart/store.test.ts`
Expected: FAIL (a type error, because the action doesn't exist).

- [ ] **Step 2: Implement.** In `store.ts`, add `| { type: 'clearPostSaleChange' }` to `Action` and a case `case 'clearPostSaleChange': return { ...s, postSaleChange: 0 }`. Run the test again. Expected: PASS.

- [ ] **Step 3: The overlay.** Create `web/src/features/till/checkout/SaleComplete.tsx`:

```tsx
import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Check, Printer } from 'lucide-react'
import { motion } from 'motion/react'
import { Button } from '@/components/ui/button'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery } from '../api'
import { Kbd } from '../bits'
import { notesFor } from '../cart/totals'
import { METHOD_LABEL } from '../pay/Tender'
import { printSale } from '../print'
import { PANEL } from '../ui'
import { useCheckoutCtx } from './CheckoutProvider'

/** "Ask before printing" mode: the change shows instantly; Print or start the next sale. */
export function SaleComplete() {
  const co = useCheckoutCtx()
  const cfg = useSuspenseQuery(configQuery).data
  const denoms = (useQuery(drawerQuery).data?.denominations ?? []).map((d) => num(d.value))
  const res = co.done
  if (!res) return null
  const sym = cfg.symbol
  const change = num(res.changeGiven)
  const methods = res.methods.map((m) => METHOD_LABEL[m] ?? cfg.tenderMethods.find((t) => t.value === m)?.label ?? m).join(' + ')
  const bal = res.customerBalanceAfter === undefined ? null : num(res.customerBalanceAfter)
  return (
    <div role="dialog" aria-modal="true" aria-labelledby="sale-done-title"
      className="absolute inset-0 z-30 grid place-items-center bg-[color-mix(in_srgb,var(--background)_70%,transparent)] backdrop-blur-[6px]">
      <motion.div initial={{ scale: 0.7, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} transition={{ duration: 0.35, ease: [0.2, 1.4, 0.4, 1] }}
        className={cn(PANEL, 'flex flex-col items-center gap-2 px-11 py-8 text-center')}>
        <span className="grid h-16 w-16 place-items-center rounded-full bg-success text-background"><Check size={34} strokeWidth={3} /></span>
        <h3 id="sale-done-title" className="mt-1.5 text-[22px] font-bold">Sale {res.receiptNo} complete</h3>
        <p className="text-muted-foreground tabular-nums">
          {money(sym, res.total)} · {methods}
          {num(res.roundingToAccount) > 0 && ` · ${money(sym, res.roundingToAccount)} to account`}
        </p>
        {change > 0 && (
          <>
            <p className="mt-2 text-muted-foreground">Give change</p>
            <p className="font-display text-[40px] font-extrabold leading-none text-success tabular-nums">{money(sym, change)}</p>
            <p className="text-[13px] text-muted-foreground tabular-nums">{notesFor(change, denoms).map((n) => `${n.qty} × ${qtyText(n.value)}`).join(' · ')}</p>
          </>
        )}
        {bal !== null && (
          <p className="text-[13px] text-muted-foreground">
            {res.customerName} {bal >= 0 ? `now owes ${money(sym, bal)}` : `has ${money(sym, -bal)} advance`}
          </p>
        )}
        <div className="mt-3 flex gap-2">
          <Button variant="secondary" onClick={() => void printSale(res.id)}><Printer size={16} /> Print receipt</Button>
          <Button autoFocus onClick={co.newSale}>Start next sale <Kbd className="border-current bg-transparent text-current">Enter</Kbd></Button>
        </div>
      </motion.div>
    </div>
  )
}
```

The completed-sale reprint is deliberately `printSale(res.id)` with no kick. In prompt mode the server already popped the drawer at sale time.

- [ ] **Step 4: The post-sale banner.** Create `web/src/features/till/cart-ui/PostSaleChange.tsx`:

```tsx
import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { useEffect } from 'react'
import { money, num, qtyText } from '@/lib/money'
import { configQuery, drawerQuery } from '../api'
import { notesFor } from '../cart/totals'
import { useTill } from '../TillProvider'

/** Auto-print mode: the last sale's change, kept visible after the reset (no modal)
 * until the next add, the ✕, or 20 seconds. */
export function PostSaleChange() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const denoms = (useQuery(drawerQuery).data?.denominations ?? []).map((d) => num(d.value))
  const amt = state.postSaleChange
  useEffect(() => {
    if (!amt) return
    const t = setTimeout(() => dispatch({ type: 'clearPostSaleChange' }), 20_000)
    return () => clearTimeout(t)
  }, [amt, dispatch])
  return (
    <AnimatePresence>
      {amt > 0 && (
        <motion.div initial={{ height: 0, opacity: 0 }} animate={{ height: 'auto', opacity: 1 }} exit={{ height: 0, opacity: 0 }} transition={{ duration: 0.22 }}
          role="status" className="mx-2.5 mb-2 flex items-center gap-3 overflow-hidden rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--success)_16%,transparent)] px-3 py-2">
          <span className="flex-1">
            <span className="text-[13px] font-semibold">Give change</span>{' '}
            <b className="font-display text-xl text-success tabular-nums">{money(cfg.symbol, amt)}</b>
            <span className="block text-[12px] text-muted-foreground tabular-nums">{notesFor(amt, denoms).map((n) => `${n.qty} × ${qtyText(n.value)}`).join(' · ')}</span>
          </span>
          <button type="button" aria-label="Dismiss change" onClick={() => dispatch({ type: 'clearPostSaleChange' })} className="text-faint hover:text-foreground"><X size={16} /></button>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
```

In `CartPanel.tsx`, render `<PostSaleChange />` directly under the `<header>`.

- [ ] **Step 5: Hotkeys.** Create `web/src/features/till/checkout/Hotkeys.tsx`:

```tsx
import { useEffect, useRef } from 'react'
import { useHold } from '../cart-ui/useHold'
import { hotkeyFor } from '../hotkeys'
import { useTillUi } from '../TillUi'
import { useCheckoutCtx } from './CheckoutProvider'

const focus = (id: string) => {
  const el = document.getElementById(id) as HTMLInputElement | null
  el?.focus()
  el?.select?.()
}

/** Till keyboard (karots-pos parity) + the number-input scroll-wheel guard. */
export function Hotkeys() {
  const ui = useTillUi()
  const co = useCheckoutCtx()
  const hold = useHold()
  const live = useRef({ ui, co, hold })
  live.current = { ui, co, hold }

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const { ui, co, hold } = live.current
      const dialogOpen = !!document.querySelector('[role="dialog"],[role="alertdialog"]')
      const a = hotkeyFor(e.key, { dialogOpen, saleDone: !!co.done })
      if (!a) return
      e.preventDefault() // also stops F1 opening browser help, and Enter double-firing the focused button
      if (a === 'reset') ui.reset()
      else if (a === 'search') focus('till-search')
      else if (a === 'discount') focus('till-bill-discount')
      else if (a === 'pay') focus('till-amount')
      else if (a === 'hold') void hold()
      else if (a === 'newSale') co.newSale()
      else if (a === 'blur') (document.activeElement as HTMLElement | null)?.blur()
    }
    // A focused number input changes on scroll — a silent way to mis-enter money.
    const onWheel = (e: WheelEvent) => {
      const el = e.target as HTMLInputElement | null
      if (el?.tagName === 'INPUT' && el.type === 'number' && el === document.activeElement) el.blur()
    }
    window.addEventListener('keydown', onKey)
    document.addEventListener('wheel', onWheel, { passive: true })
    return () => {
      window.removeEventListener('keydown', onKey)
      document.removeEventListener('wheel', onWheel)
    }
  }, [])
  return null
}
```

- [ ] **Step 6: Final screen.** Replace `web/src/features/till/TillScreen.tsx` with the finished version:

```tsx
import { useQuery } from '@tanstack/react-query'
import { MotionConfig } from 'motion/react'
import { useEffect, useRef } from 'react'
import { api } from '@/api/client'
import { cn } from '@/lib/utils'
import { drawerQuery } from './api'
import { CartPanel } from './cart-ui/CartPanel'
import { CatalogPanel } from './catalog/CatalogPanel'
import { CheckoutProvider } from './checkout/CheckoutProvider'
import { Hotkeys } from './checkout/Hotkeys'
import { Prompts } from './checkout/Prompts'
import { SaleComplete } from './checkout/SaleComplete'
import { OpenRegister } from './drawer/OpenRegister'
import { PayPanel } from './pay/PayPanel'
import { TillDialogs } from './TillDialogs'
import { TillUiProvider, useTillUi } from './TillUi'
import { PANEL } from './ui'

/** Sign-out was refused (TILL_OPEN): open the count in logout mode; if the till is
 * somehow already closed, just finish signing out. */
function LogoutClose({ logout }: { logout: boolean }) {
  const ui = useTillUi()
  const d = useQuery(drawerQuery).data
  const handled = useRef(false)
  useEffect(() => {
    if (!logout || !d || handled.current) return
    handled.current = true
    if (d.summary.session) ui.open({ kind: 'close', logout: true })
    else void api('/auth/logout', { method: 'POST' }).finally(() => window.location.assign('/login'))
  }, [logout, d, ui])
  return null
}

function Body() {
  const drawer = useQuery(drawerQuery).data
  return (
    <div className="relative grid h-full min-h-0 grid-cols-[1fr_360px] grid-rows-[minmax(0,1fr)_auto] gap-[14px] p-[14px] [grid-template-areas:'cat_cart'_'cat_pay'] min-[1200px]:grid-cols-[1fr_392px_356px] min-[1200px]:grid-rows-[minmax(0,1fr)] min-[1200px]:[grid-template-areas:'cat_cart_pay']">
      <section aria-label="Catalogue" className="flex min-h-0 flex-col gap-3 [grid-area:cat]">
        <CatalogPanel />
      </section>
      <section aria-label="Cart" className={cn(PANEL, 'flex min-h-0 flex-col [grid-area:cart]')}>
        <CartPanel />
      </section>
      <section aria-label="Payment" className={cn(PANEL, 'flex min-h-0 flex-col [grid-area:pay]')}>
        <PayPanel />
      </section>
      {drawer && !drawer.summary.session && <OpenRegister state={drawer} />}
      <SaleComplete />
    </div>
  )
}

/** The sell screen (theme.md §7): catalogue | cart | pay. */
export function TillScreen({ logout = false }: { logout?: boolean }) {
  return (
    <MotionConfig reducedMotion="user">
      <TillUiProvider>
        <CheckoutProvider>
          <Body />
          <TillDialogs />
          <Prompts />
          <Hotkeys />
          <LogoutClose logout={logout} />
        </CheckoutProvider>
      </TillUiProvider>
    </MotionConfig>
  )
}
```

- [ ] **Step 7: Test and check.**

Run: `cd web && bunx vitest run && bunx tsc -b && bun run lint`
Expected: PASS.

Then check these by hand on `make build`:
- **F1** with text in search keeps the cart.
- **F4** holds.
- **F9** focuses the bill discount.
- **F10**, then type an amount, **Enter** adds it, and **Enter** again completes.
- **Enter** on the overlay starts the next sale.
- **Esc** closes a dialog.
- **Auto-lock** (set `lockTimeoutMinutes` to 1 via the admin later, or call `POST /api/v1/auth/lock` with curl) followed by unlock brings the same cart back.
- **With `ask_to_print=false`** (`UPDATE settings SET ask_to_print=false`), a sale shows the green change banner in the cart instead of the overlay.

- [ ] **Step 8: UI GATE 4 — the complete flow.**
  1. Run `SHOTS=gate4 SHOTS_SCENE=done bunx playwright test e2e/till-shots.spec.ts`, which captures the sale-complete overlay.
  2. Capture the post-sale change banner (auto-print mode) and the till at 1024 wide (two-column layout) with the MCP browser, in all four combinations.
  3. **Stop.** Show the screenshots and wait for approval.

- [ ] **Step 9: Commit** after approval.

```bash
git add web
git commit -m "feat(web): till sale complete (instant change, print/next), auto-print with single-job kick + change banner, hotkeys (F1/F2/F4/F9/F10/Enter/Esc), scroll-wheel guard"
```

---

### Task 14: Golden till flows (Playwright) and the till seed

**Files:**
- Create: `scripts/e2e-till.sql`, `web/e2e/till.spec.ts`
- Modify: `Makefile` (`e2e-seed`)

**Interfaces:**
- Consumes: every till screen, and the fixed e2e users from `scripts/e2e-users.sql`.
- Produces: the M1a flow checks that `docs/parity.md` rows are ticked against.

- [ ] **Step 1: Seed.** Create `scripts/e2e-till.sql`:

```sql
-- Till fixtures for Playwright. Runs after `-reset` + e2e-users.sql (make e2e-seed).
-- Barcodes are fixed; the specs scan them.
UPDATE settings SET ask_to_print = true, allow_untracked_cash = true, allow_continue_last_count = true, open_cash_drawer = false;

INSERT INTO categories (name) VALUES ('E2E Goods');

INSERT INTO products (name, barcode, category_id, unit_id, cost_price, selling_price)
SELECT v.name, v.barcode, c.id, 1, v.cost, v.price
FROM (VALUES
  ('E2E Milk',     '4790000000011', 80, 100),
  ('E2E Lot Soap', '4790000000028', 50, 100),
  ('E2E Yoghurt',  '4790000000035', 40,  80),
  ('E2E Scarce',   '4790000000042', 50, 250)
) AS v(name, barcode, cost, price), categories c
WHERE c.name = 'E2E Goods';

INSERT INTO stock (product_id, quantity)
SELECT id, CASE name WHEN 'E2E Milk' THEN 500 WHEN 'E2E Lot Soap' THEN 10 WHEN 'E2E Yoghurt' THEN 6 ELSE 1 END
FROM products WHERE name LIKE 'E2E %'
ON CONFLICT (product_id) DO UPDATE SET quantity = EXCLUDED.quantity;

INSERT INTO stock_batches (product_id, qty_received, qty_remaining, cost_price, selling_price, expiry_date)
SELECT p.id, v.qty, v.qty, 50, v.price, v.expiry
FROM (VALUES
  ('E2E Milk',     500, NULL::numeric, NULL::date),
  ('E2E Lot Soap',   5, NULL,          NULL),
  ('E2E Lot Soap',   5, 120,           NULL),
  ('E2E Yoghurt',    3, NULL,          CURRENT_DATE - 1),
  ('E2E Yoghurt',    3, NULL,          CURRENT_DATE + 180),
  ('E2E Scarce',     1, NULL,          NULL)
) AS v(name, qty, price, expiry)
JOIN products p ON p.name = v.name;

INSERT INTO product_groups (name, emoji) VALUES ('E2E Dairy', '🥛');
INSERT INTO product_group_items (group_id, product_id)
SELECT g.id, p.id FROM product_groups g, products p
WHERE g.name = 'E2E Dairy' AND p.name IN ('E2E Milk', 'E2E Yoghurt');

INSERT INTO customers (name, phone, credit_limit) VALUES ('E2E Credit', '0771000001', 1000);
INSERT INTO lockers (name, kind, cashier_access) VALUES ('E2E Card machine', 'card', true);
```

In `Makefile`, replace the `e2e-seed` target:

```make
# Fresh DB + the fixed e2e logins (0770000001 admin, 0770000002 cashier, PIN 1234)
# + the till fixtures (products/lots/customer/card account). Resets the dev DB.
e2e-seed: db-up
	$(ENV) go run ./cmd/server -reset
	docker compose exec -T postgres psql -U suite_user -d suite_db -v ON_ERROR_STOP=1 -f - < scripts/e2e-users.sql
	docker compose exec -T postgres psql -U suite_user -d suite_db -v ON_ERROR_STOP=1 -f - < scripts/e2e-till.sql
```

Run: `make e2e-seed`
Expected: no SQL errors. If a column default differs (for example `lockers.cashier_access` has no default or `customers.outstanding_balance` is `NULL`), set it explicitly in the INSERT. Don't change the schema.

- [ ] **Step 2: The flows.** Create `web/e2e/till.spec.ts`:

```ts
import { expect, test, type Page } from '@playwright/test'

// Golden till flows (spec §9). Needs `make e2e-seed`. Serial: they share one drawer.
test.describe.configure({ mode: 'serial' })

const MILK = '4790000000011' // Rs 100, plenty
const SOAP = '4790000000028' // lots at 100 and 120
const YOGHURT = '4790000000035' // FEFO-first lot expired
const SCARCE = '4790000000042' // 1 on hand

async function login(page: Page, phone: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  if (!page.url().includes('/till')) await page.goto('/till')
  await page.waitForLoadState('networkidle')
}

async function skipSlip(page: Page) {
  const skip = page.getByRole('button', { name: 'Skip' })
  await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
}

async function ensureOpen(page: Page) {
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  if (await dlg.isVisible()) {
    await dlg.getByLabel(/Count of .*500\.00/).fill('2')
    await dlg.getByRole('button', { name: 'Open the till' }).click()
    await expect(dlg).toBeHidden()
    await skipSlip(page)
  }
}

const search = (page: Page) => page.getByLabel('Scan a barcode or search products')
async function scan(page: Page, code: string) {
  await search(page).fill(code)
  await search(page).press('Enter')
}
const cart = (page: Page) => page.getByRole('region', { name: 'Cart' })
const amount = (page: Page) => page.getByLabel('Amount tendered')
const done = (page: Page) => page.getByRole('dialog', { name: /complete/ })

test('open the till with a counted float', async ({ page }) => {
  await login(page, '0770000002')
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  await expect(dlg).toBeVisible()
  await dlg.getByLabel(/Count of .*500\.00/).fill('2')
  await expect(dlg.getByText(/1,000\.00/).first()).toBeVisible()
  await dlg.getByRole('button', { name: 'Open the till' }).click()
  await skipSlip(page)
  await expect(page.getByText(/Till open · float .*1,000\.00/)).toBeVisible()
})

test('three fast scans = qty 3; cash with change; Enter starts the next sale', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  for (let i = 0; i < 3; i++) await scan(page, MILK)
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('3')
  await page.keyboard.press('F10')
  await page.keyboard.type('500')
  await page.keyboard.press('Enter')
  await expect(page.getByText('Change to give')).toBeVisible()
  await amount(page).press('Enter')
  await expect(done(page)).toContainText(/200\.00/)
  await page.keyboard.press('Enter')
  await expect(done(page)).toBeHidden()
  await expect(cart(page).getByText(/Cart is empty/)).toBeVisible()
})

test('F1 clears the search, never the cart; a reload keeps the cart', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await search(page).fill('zzz')
  await page.keyboard.press('F1')
  await expect(search(page)).toHaveValue('')
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
  await page.reload()
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
})

test('split tender: card then cash', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await page.getByRole('button', { name: 'Card', exact: true }).click()
  await amount(page).fill('60')
  await amount(page).press('Enter')
  await page.getByRole('button', { name: 'Cash', exact: true }).click()
  await amount(page).fill('40')
  await amount(page).press('Enter')
  await amount(page).press('Enter')
  await expect(done(page)).toContainText('Card + Cash')
})

test('customer with a blank tender goes on account (never silent cash)', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await page.getByRole('button', { name: /Walk-in customer/ }).click()
  await page.getByLabel('Search customers').fill('E2E Credit')
  await page.getByLabel('Search customers').press('Enter')
  await expect(page.getByRole('button', { name: 'Cash', exact: true })).toHaveAttribute('aria-pressed', 'false')
  await amount(page).press('Enter')
  await page.getByRole('button', { name: 'Put on account' }).click()
  await expect(done(page)).toContainText('On account')
})

test('over the limit: cashier refused, admin approves', async ({ page }) => {
  // E2E Credit owes 100 (previous test), limit 1000: 10 × 100 more is over.
  for (const phone of ['0770000002', '0770000001']) {
    await login(page, phone)
    await ensureOpen(page)
    await scan(page, MILK)
    const qty = cart(page).getByLabel('Quantity of E2E Milk')
    await qty.fill('10')
    await qty.press('Enter')
    await page.getByRole('button', { name: /Walk-in customer/ }).click()
    await page.getByLabel('Search customers').fill('E2E Credit')
    await page.getByLabel('Search customers').press('Enter')
    await amount(page).press('Enter')
    await page.getByRole('button', { name: 'Put on account' }).click()
    if (phone === '0770000002') {
      await expect(page.locator('[data-sonner-toast][data-type="error"]')).toBeVisible()
      await expect(done(page)).toBeHidden()
      await page.getByRole('button', { name: 'New sale' }).click()
      await page.getByRole('button', { name: 'Clear sale' }).click()
      await page.context().clearCookies()
    } else {
      await page.getByRole('button', { name: 'Approve' }).click()
      await expect(done(page)).toBeVisible()
    }
  }
})

test('which price? pick the dearer lot', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, SOAP)
  const dlg = page.getByRole('dialog', { name: 'Which price?' })
  await dlg.getByRole('button', { name: /120\.00/ }).click()
  await expect(cart(page).getByText(/120\.00 each/)).toBeVisible()
})

test('expired lot first: sell anyway', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, YOGHURT)
  await page.getByRole('dialog', { name: 'Which price?' }).getByRole('button', { name: /Expired/ }).click()
  await expect(page.getByRole('dialog', { name: 'This lot is expired' })).toBeVisible()
  await page.getByRole('button', { name: 'Sell anyway' }).click()
  await expect(cart(page).getByText(/^exp /)).toBeVisible()
})

test('found at till: selling more than the count asks, then sells', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, SCARCE)
  await cart(page).getByLabel('More E2E Scarce').click()
  await amount(page).press('Enter')
  await expect(page.getByRole('dialog', { name: 'Sell more than the count shows?' })).toBeVisible()
  await page.getByRole('button', { name: 'Sell 2' }).click()
  await expect(done(page)).toBeVisible()
})

test('hold (F4) and resume', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await page.keyboard.press('F4')
  await expect(cart(page).getByText(/Cart is empty/)).toBeVisible()
  await cart(page).getByRole('button', { name: /E2E Milk/ }).click()
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
})

test('quick item for something not in the catalogue', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, 'Brand new thing')
  const dlg = page.getByRole('dialog', { name: 'Quick item' })
  await expect(dlg.getByLabel('Item name')).toHaveValue('Brand new thing')
  await dlg.getByLabel('Price').fill('50')
  await dlg.getByRole('button', { name: 'Add to cart' }).click()
  await expect(cart(page).getByLabel('Quantity of Brand new thing')).toHaveValue('1')
})

test('deposit, withdraw, then close and reconcile', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await page.getByRole('button', { name: /Deposit/ }).first().click()
  let dlg = page.getByRole('dialog', { name: /Deposit cash/ })
  await dlg.getByLabel('Amount').fill('200')
  await dlg.getByRole('button', { name: 'Deposit' }).click()
  await skipSlip(page)
  await page.getByRole('button', { name: /Withdraw/ }).first().click()
  dlg = page.getByRole('dialog', { name: /Withdraw cash/ })
  await dlg.getByLabel('Amount').fill('100')
  await dlg.getByRole('button', { name: 'Withdraw' }).click()
  await skipSlip(page)
  await page.getByRole('button', { name: 'Close till' }).click()
  dlg = page.getByRole('dialog', { name: 'Close the till' })
  await dlg.getByLabel(/Count of .*500\.00/).fill('2')
  await dlg.getByRole('button', { name: /Close & reconcile/ }).click()
  await expect(page.getByRole('dialog', { name: 'Till closed' })).toContainText('Over / short')
  await page.getByRole('button', { name: 'Done' }).click()
  await expect(page.getByRole('dialog', { name: 'Open the till' })).toBeVisible()
})

test('signing out with the till open goes to the count, then signs out', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await page.getByRole('button', { name: 'Sign out' }).click()
  const dlg = page.getByRole('dialog', { name: 'Close the till' })
  await expect(dlg).toContainText('Close your till before signing out')
  await dlg.getByRole('button', { name: /Close & reconcile/ }).click()
  await page.getByRole('button', { name: 'Sign out' }).last().click()
  await expect(page).toHaveURL(/\/login/)
})
```

- [ ] **Step 3: Run the tests.**

Run:

```bash
lsof -ti:3100,5173 | xargs -r kill   # Playwright reuses whatever listens there (AGENTS.md gotcha)
make e2e-seed && cd web && bunx playwright test
```

Expected: every spec passes, the M0 specs included. `till-shots.spec.ts` shows as skipped.

If a flow fails on a karots-pos behaviour difference, fix the app, not the test, unless the test contradicts karots-pos. Show the failing output in the task report.

- [ ] **Step 4: Commit.**

```bash
git add scripts/e2e-till.sql Makefile web/e2e/till.spec.ts
git commit -m "test(e2e): golden till flows — scan/cash/change, F1, reload, split, on account, over-limit gate, lots, expiry, found-at-till, hold, quick item, drawer, logout-close"
```

---

### Task 15: Parity, docs and ship check

**Files:**
- Modify: `docs/parity.md`, `docs/status.md`, `AGENTS.md`

- [ ] **Step 1: Tick parity.** In `docs/parity.md`, tick exactly these `internal/web/register_cashier.go` rows. Each is ported and flow-checked by Task 14.
  - GET `` (the till)
  - GET `/lockers` (drawer state and card accounts)
  - POST `/quick-item`

  Leave these **unticked**, because they are ported but not flow-checked without a printer and drawer. Task 15 Step 2 records them:
  - POST `/drawer/open` (no-sale)
  - POST `/print/:id`
  - POST `/money-receipts/:id/print`
  - POST `/damage` (M1a only ports the expired-lot write-off; M1b owns the page)

- [ ] **Step 2: Update `docs/status.md`.**
  - Set the status date to today. Say M1a is COMPLETE, awaiting Opus review, and that the next milestone is **M1b Till: counter operations**, with a plan to write.
  - List what M1a delivered in one bullet per area:
    - the drawer
    - the catalogue
    - the cart, lots and prompts
    - pay and tenders
    - the checkout prompts
    - printing
    - hotkeys
    - persistence
    - the slots
  - Under "Inputs for M1b", list the M1b scope from this plan's intro. Add:
    - the tab row in `TillShell`
    - the till state kept in `TillProvider` across tabs (the cart survives switching to Returns)
    - the Z report and browser receipt view that M1b owns
    - the unticked print and no-sale rows, which need a real-printer check (POS80) during review
  - Carry the open items forward:
    - counter name on the pill (no data source yet)
    - `plugin/hooks.go` comments (M7)
    - the karots-pos deviation: resume refuses while a sale is in the cart

- [ ] **Step 3: Update `AGENTS.md` layout.** Add these lines under `web/`:

```
  src/features/till/  sell screen: cart/ (pure logic + reducer, Vitest), drawer/, catalog/, cart-ui/, pay/, checkout/
                      state in TillProvider (per-user sessionStorage); dialogs via TillUi; data via api.ts queries
  src/plugins/slots.ts  typed plugin slots (tillMenu, tillActions, drawerSections, tillTenders, before/after-sale)
```

  Add this line under `internal/api/`: `till*.go — /api/v1/till/* (config, catalog, customers, drawer, sales, held, printing)`.

- [ ] **Step 4: Ship check.**

Run:

```bash
make types && git diff --exit-code web/src/api/types.gen.ts
go vet ./... && make test-fresh
cd web && bun run lint && bunx tsc -b && bun run test && cd ..
make build && make verify-portable
grep -rniE 'recharge|reload|wallet|pharmacy|documents|clearance|combos|aicatalog|productplus|alternatives|repairs|requests' internal --include='*.go' -l | grep -v '^internal/plugin/' ; grep -rniE 'recharge|reload|wallet|pharmacy|clearance|combos' web/src --include='*.ts*' -l | grep -v '^web/src/plugins/'
wc -l internal/api/*.go web/src/features/till/**/*.ts* | sort -n | tail -5
```

Expected:
- the types are unchanged
- every Go and web test passes
- the binary is statically linked
- **the plugin grep prints nothing.** Words such as `reload` in `queryClient` code may show up. Read each hit and make sure it is not a plugin reference.
- no file is over 500 lines

- [ ] **Step 5: Commit.**

```bash
git add docs AGENTS.md
git commit -m "docs: M1a till (sell) complete — parity ticks, status handoff to M1b, AGENTS layout"
```

---

## Self-review notes (done while writing)

- **Spec §6.5 coverage in M1a:**
  - **Sale:** search and scan, the group cards menu, frequently sold, ⓘ info with the cost gate (Tasks 1 and 10).
  - **Stock prompts:** FEFO lots, "which price?", the expiry prompt, found at till including service and recipe lines, clearance suggestions (Tasks 6, 11 and 12).
  - **Payment:** tenders and split payments, card and online to an account, credit as a payment line, the over-limit override, change Keep and round-to-account, instant change, single-job non-blocking print (Tasks 4, 5, 7, 12 and 13).
  - **Held sales** (Tasks 4 and 11).
  - **Drawer:**
    - open and close with side-by-side plugin float (the `drawerSections` slot)
    - continue last
    - cash in and out with the money cue
    - the default locker
    - untracked cash out only
    - the drawer kick (Tasks 3 and 9)
  - **Hotkeys**, including F10 → amount → Enter and the scroll-wheel guard (Tasks 7 and 13).
  - **Kiosk autofit** (M0) and the kiosk Konami (M0).
- **M1b owns the rest:**
  - returns and warranty by receipt
  - damage and conversions pages, labels
  - supplier actions and cashier expenses
  - credit collection and advances and refunds
  - cashier receipts and the Z report
- **Advances.** The picker shows a customer's advance and "Keep all as advance" (round-to-account). Refunding an advance is credit collection, which is M1b.
- **Plugin inertness.** Wallet, reload and document-job behaviour is not in core. It enters through `TenderMethods` (Go), `slots.tillTenders`, `tillBeforeCheckout`, `tillAfterSale`, `tillMenu`, `drawerSections` and `tillActions`.
