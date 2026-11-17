# Sell page layout Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The system user sets, per screen size seen at the shop, the Sell page's arrangement, section widths, product card size and text size; every till uses the layout for its size, or today's layout when none is set.

**Architecture:** A shop-wide `settings.sell_layouts` map keyed `"<w>x<h>"` (validated in Go) reaches the till in `TillConfig`; pure TS (`till/layout/`) picks the layout for the window and turns it into CSS grid rows / columns / areas on the existing grid in `TillScreen`. Tills report their window size on the presence beat; the server keeps the sizes seen (with who used them) in a small table, which Appearance lists. Plugins are untouched until the last task, which checks each plugin's Sell-page pieces in the narrowest sections and fixes them inside the plugin.

**Tech Stack:** Go (Echo, sqlx, goose), Postgres 17, React + TS, TanStack Query, Tailwind v4 tokens, Vitest, Playwright.

**Spec:** `docs/specs/2026-11-17-sell-page-layout-design.md` (approved mockup: `.superpowers/brainstorm/42445-1794886387/content/sell-layout-v2.html`).

## Global Constraints

- Only the system user can read or change layouts (`/api/v1/system/*`, `RequireSystemUser`); no admin setting, no nav item.
- No saved layout for a size, or one that fails validation → today's layout exactly (the current classes in `TillScreen.tsx:72`, untouched).
- Arrangement ids (the only ones accepted): `PCY CPY PYC CYP YCP P_CY CY_P PY_C C_PY P_YC` (`_` = column break; letters in a column stack top→bottom; P products, C cart, Y pay).
- Minimum section widths at the layout's own width: Payment 330 px, Cart 340 px, Products 300 px; a stacked column needs the larger of its two. Outer padding 14 px each side, gap 14 px.
- Stacked split (top part's share): 0.2–0.8. Card sizes: `L` (today) · `M` · `S` · `T` (list). Text size: `90` · `100` (default) · `110` percent.
- Sell page text comes only from the till type scale (Task 5); no `text-[Npx]` under `web/src/features/till` afterwards (guard test).
- Matching: the saved size nearest the window whose width and height are each within ±10%; else today's.
- Sizes are CSS px of the window (`window.innerWidth × innerHeight`), so Windows scaling is counted.
- Seen sizes are recorded only from the Sell page (`path === '/till'`); one write per device per size change or per hour; listed while seen in the last 90 days (a layout is kept even after its size drops off).
- Core is plugin-inert: nothing under `web/src` outside `plugins/` and no core Go names a plugin. Pocket Cashier is not touched.
- Theme tokens only (`theme/guard.test.ts`), files ≤ 500 lines, `make types` after DTO changes, Appearance works at 360 px.
- Migration is `00036_sell_layouts.sql`.

## Review Focus

- A till whose window changes size mid-sale (resize, zoom, DevTools) re-picks its layout without losing the cart, search focus or an open dialog.
- A saved layout that a later build doesn't know (bad arrangement id, wrong share count, JSON junk) shows today's layout, never a blank or broken page.
- The overlays that cover the whole grid (`OpenRegister`, `SaleComplete`, `Prompts`) still cover all three sections in every arrangement.
- The sales person's Sell page (`SendPanel` in the pay area) works in every arrangement.
- List cards (`T`): ↑ ↓ in search moves the highlight row by row and Enter adds the highlighted product; a scanner still adds straight away.

---

### Task 1: Layouts saved and validated (Go)

**Files:**
- Create: `migrations/00036_sell_layouts.sql`
- Create: `internal/features/settings/sell_layouts.go`, `internal/features/settings/sell_layouts_test.go`
- Modify: `internal/features/settings/settings.go` (field + `SetSellLayouts` on Repository and Service)

**Interfaces:**
- Produces: `type SellLayout struct { Arr string \`json:"arr"\`; Fr []float64 \`json:"fr"\`; Split float64 \`json:"split"\`; Card string \`json:"card"\` }`; `type SellLayouts map[string]SellLayout` (Scan / Value like `LabelStyles`); `func ValidSellLayouts(in map[string]SellLayout) (SellLayouts, error)`; `Settings.SellLayouts SellLayouts \`db:"sell_layouts"\``; `func (s *Service) SetSellLayouts(ctx, SellLayouts) error`.

- [ ] **Step 1:** Migration: `ALTER TABLE settings ADD COLUMN sell_layouts JSONB NOT NULL DEFAULT '{}';` plus the `screen_sizes` table for Task 2: `(w int, h int, users text[] NOT NULL DEFAULT '{}', last_seen timestamptz NOT NULL, PRIMARY KEY (w, h))`. Down drops both. Header comment says what each is.
- [ ] **Step 2: Failing tests** in `sell_layouts_test.go` (table-driven):
  - `{"1366x768": {Arr:"PCY", Fr:[.42,.29,.29], Card:"M"}}` → kept; `Fr` normalised to sum 1.
  - unknown arr `"PXY"` → error; key `"big"` or `"99x99"` → error (key must match `^\d{3,4}x\d{3,4}$`).
  - `Arr:"P_CY"` with 3 shares → error (2 columns need 2).
  - `"1366x768"` `PCY` with Payment share giving 300 px (< 330) → error naming "Payment".
  - `Split` 0.9 on a stacked arrangement → error; Split ignored (set 0) on a 3-column one.
  - `Card` `""` → `"L"`; `"X"` → error. `Text` 0 → 100; 95 → error.
  - Empty map → empty map (Reset of every size).
- [ ] **Step 3:** Run `go test ./internal/features/settings -run SellLayouts` → FAIL.
- [ ] **Step 4:** Implement `ValidSellLayouts` (arrangement table = column lists as in Global Constraints; width per column = share × (W − 28 − 14 × (cols − 1)) against the column's minimum). Errors are `apperr.BadRequest` with a plain message ("Payment is too narrow at 1366 × 768").
- [ ] **Step 5:** `SetSellLayouts` stores the whole map (the Appearance card saves all sizes at once). Run the package tests → PASS.
- [ ] **Step 6:** Commit `feat(settings): Sell page layouts per screen size, checked before they're saved`.

### Task 2: Screen sizes seen at the shop

**Files:**
- Create: `internal/features/screens/screens.go`, `internal/features/screens/screens_test.go`
- Modify: `internal/api/dto_people.go` (`PresenceBeat` gets `ViewW int \`json:"viewW"\``, `ViewH int \`json:"viewH"\``), `internal/api/presence.go`, `internal/app/services.go`
- Modify: `web/src/features/auth/usePresence.ts` (send `viewW: innerWidth, viewH: innerHeight`)

**Interfaces:**
- Produces: `type Size struct { W, H int; Users []string; LastSeen time.Time }`; `func (s *Service) Seen(ctx, device string, w, h int, name string, now time.Time)` (throttled in memory per device: writes when the size changed or an hour passed; upsert adds `name` to `users` once, keeps the last 6 names); `func (s *Service) List(ctx, now time.Time) ([]Size, error)` (last 90 days, most recent first).

- [ ] **Step 1: Failing tests** (DB-gated like the other features): two devices at 1920×1080 by "Shehan" and "Nimal" → one row with both names; the same device beating again within the hour → no second write (count queries); a size last seen 91 days ago → not listed; w or h < 300 or > 8000 → ignored.
- [ ] **Step 2:** Run `go test ./internal/features/screens` → FAIL.
- [ ] **Step 3:** Implement. `PresenceBeat` handler calls `Seen` only when `req.Path == "/till"`, the user isn't the system user, and both sizes are set; errors are logged, never returned (a beat must not fail over this).
- [ ] **Step 4:** `make types`; run the package tests and `go test ./internal/api -run Presence` → PASS.
- [ ] **Step 5:** Commit `feat(presence): tills report their window size; the shop's screen sizes are kept`.

### Task 3: The system API and the till's config

**Files:**
- Modify: `internal/api/dto.go` (DTOs), `internal/api/system.go` (routes), `internal/api/dto_till.go` + the till config handler, `internal/api/system_layout_test.go` (create)

**Interfaces:**
- Consumes: Task 1 `ValidSellLayouts`, `SetSellLayouts`; Task 2 `List`.
- Produces: `GET /api/v1/system/sell-layout` → `SellLayoutPage { layouts: Record<string, SellLayout>; screens: ScreenSize[] }` with `ScreenSize { w, h, users: string[], lastSeen }`; `PUT /api/v1/system/sell-layout` body `{ layouts }` → the same page; `TillConfig.sellLayouts: Record<string, SellLayout>`.

- [ ] **Step 1: Failing tests:** PUT as system user saves and GET returns it; PUT as admin → 403/404 like `/system/appearance`; PUT with a too-narrow Payment → 400 and nothing changes; `GET /till/config` carries `sellLayouts`.
- [ ] **Step 2:** Run `go test ./internal/api -run SellLayout` → FAIL.
- [ ] **Step 3:** Implement; `make types`.
- [ ] **Step 4:** Tests PASS. Commit `feat(api): system user reads and saves Sell page layouts; the till gets them`.

### Task 4: Picking and drawing a layout (pure TS)

**Files:**
- Create: `web/src/features/till/layout/arrangements.ts`, `web/src/features/till/layout/arrangements.test.ts`

**Interfaces:**
- Produces: `ARRANGEMENTS: { id: ArrId; cols: Area[][]; label: string }[]` (labels from the mockup: Today, Cart first, Pay middle, Products right, Mirror, Cart over pay, …on the left, Cart full height, …on the left, Pay over cart); `type Area = 'cat' | 'cart' | 'pay'`; `MIN: Record<Area, number>`; `pickLayout(layouts: Record<string, SellLayout>, w: number, h: number): SellLayout | null`; `gridOf(l: SellLayout): { gridTemplateColumns: string; gridTemplateRows: string; gridTemplateAreas: string } | null` (null for an unknown arr or wrong share count); `todayLayout(w: number): SellLayout` (what Appearance starts from: `PCY` with today's clamp widths at ≥ 1200, `P_CY` with 2:3 rows below); `sizeKey(w, h)`.

- [ ] **Step 1: Failing tests:** `pickLayout` with `1920x1080` and `1366x768` saved: window 1900×1050 → the 1920 one; 1500×768 → null (width 10%+ off both); empty map → null. `gridOf({arr:'PY_C', fr:[.6,.4], split:.55})` → columns `minmax(0,.6fr) minmax(0,.4fr)`, rows `minmax(0,.55fr) minmax(0,.45fr)`, areas `"'cat cart' 'pay cart'"`. `gridOf` of `{arr:'ZZZ'}` → null. `todayLayout(1366).fr` sums to 1 and matches 392 / 356 px for cart / pay.
- [ ] **Step 2:** `cd web && bunx vitest run src/features/till/layout` → FAIL.
- [ ] **Step 3:** Implement. **Step 4:** PASS. **Step 5:** Commit `feat(till): Sell page arrangements and picking a layout for the window`.

### Task 5: One type scale for the Sell page (UI gate)

**Files:**
- Modify: `web/src/theme/tokens.css` (+ `palettes/tokens.ts` if tokens are listed there): `--till-text: 1` and six steps, each `calc(<px> * var(--till-text))`: `--fs-till-xs` 11px, `--fs-till-sm` 12.5px, `--fs-till-body` 13.5px, `--fs-till-lead` 15px, `--fs-till-title` 17px, `--fs-till-price` 22px; the big numbers (`text-2xl`+, 32 / 40 px totals) get `--fs-till-total` 32px and `--fs-till-hero` 40px.
- Modify: every `.tsx` under `web/src/features/till/` that uses `text-[Npx]` or `text-sm|base|lg|xl|2xl|3xl`: map each to the nearest step (11 / 11.5 → xs; 12 / 12.5 / 13 → sm; 13.5 / 14 / 14.5 → body; 15 / 16.5 → lead; 17 / 20 → title; 22 → price). The faint 96 px letter stays as it is.
- Modify: `web/src/theme/guard.test.ts`: fail on `text-\[\d` or `text-(sm|base|lg|xl|2xl|3xl)` under `web/src/features/till/`.

- [ ] **Step 1:** Add the guard rule → it FAILS listing the files.
- [ ] **Step 2:** Add the tokens; replace sizes file by file (one Tailwind utility per step, e.g. `text-(length:--fs-till-sm)`); guard PASSES; `bunx vitest run src/features/till src/theme` PASS.
- [ ] **Step 3: Gate:** before / after screenshots of the Sell page, each counter tab, the pay dialogs and open / close till at 1366×768: same look (±0.5 px is fine). Then set `--till-text: .9` and `1.1` by hand and look: everything moves together, nothing clips.
- [ ] **Step 4:** Commit `refactor(till): one type scale for the Sell page instead of 22 pixel sizes`.

### Task 6: The Sell page uses the layout (UI gate)

**Files:**
- Create: `web/src/features/till/layout/useSellLayout.ts` (window size via `resize` listener → `pickLayout(cfg.sellLayouts, …)`)
- Modify: `web/src/features/till/TillScreen.tsx` (`Body`: when a layout is picked, put `gridOf(...)` in `style` and drop the template classes; else keep today's classes exactly; the three sections and overlays unchanged). Text size: `--till-text: <text/100>` on the till's root (counter tabs too), so dialogs on the till follow it.

- [ ] **Step 1:** Extend `web/e2e/till-fit.spec.ts`: save `{"1366x768": {arr:'C_PY', fr:[.4,.6], split:.6, card:'L'}}` through the system API, open `/till` at 1366×768 → the Cart region's left edge is left of the Catalogue's, Payment's top is below the Catalogue's bottom; at 1600×900 the default order is back. Restore `{}` after.
- [ ] **Step 2:** Run it → FAIL. **Step 3:** Implement. **Step 4:** PASS, with the existing fit tests.
- [ ] **Step 5: Gate:** `make build`, server on :3100, screenshots at 1366×768 of `PCY` (no layout), `CPY`, `PY_C`, `P_YC` with a 7-line cart; the open-register overlay and the sale-complete card in `PY_C`; a sales person (Send panel) in `C_PY`; Studio light + Night dark. Check every edge (no clipped Pay buttons, no horizontal scroll). Restore Studio.
- [ ] **Step 6:** Commit `feat(till): the Sell page follows the layout saved for its screen size`.

### Task 7: Product card sizes (UI gate)

**Files:**
- Modify: `web/src/features/till/catalog/ProductTile.tsx` (`size?: 'L' | 'M' | 'S'`; M drops the faint letter and the facts line, ~half height; S = name (2 lines) · price · stock, status marks and plugin badges kept as small chips / red dot for low stock)
- Create: `web/src/features/till/catalog/ProductRow.tsx` (List: name + local name muted · badges · stock · price; `active` row highlighted and scrolled into view; ⓘ kept)
- Modify: `web/src/features/till/catalog/CatalogPanel.tsx` (card size from the picked layout, default `L`; grid min column per size: L today's, M 170 px, S 140 px; T a single column; `GroupCard` keeps its size), `web/src/features/till/ui.ts` if a TILE variant is needed

- [ ] **Step 1:** Playwright in `till.spec.ts`: with `card:'T'` saved for the test viewport, type a search with ≥ 2 results, ↓ ↓ Enter → the second result is in the cart; with `card:'S'` a frozen product still shows "Frozen" and a low one its mark.
- [ ] **Step 2:** FAIL → implement → PASS.
- [ ] **Step 3: Gate:** screenshots of L / M / S / T at 1366×768 and 1280×1024 with a pharmacy-like list (packs, local names, a low and a frozen product, an Alternatives badge with P=all). Long names stay inside their card / row.
- [ ] **Step 4:** Commit `feat(till): product cards Large, Medium, Small or a list, per screen size`.

### Task 8: Appearance → Sell page layout (UI gate)

**Files:**
- Create: `web/src/features/appearance/SellLayout.tsx` (the card: screens list with "used by …", "this screen", a dot on set sizes; arrangement pictures; card size pills; text size Smaller · Normal · Larger (the preview follows it); Reset this size; Save), `web/src/features/appearance/LayoutPreview.tsx` (wireframe at the size's real shape, scaled; vertical bars between columns and a horizontal bar in a stacked column; dragging clamps at `MIN`; px and % under it), `web/src/features/appearance/sellLayoutQuery.ts`
- Modify: `web/src/routes/_authed/admin/system.appearance.tsx` (a `Fold` for the card, after Receipt style)

**Interfaces:**
- Consumes: Task 3 API; Task 4 `ARRANGEMENTS`, `MIN`, `todayLayout`, `sizeKey`; Task 5 `--till-text`.

- [ ] **Step 1:** Vitest for the drag maths in `LayoutPreview` (pure helper `dragBar(fr, i, dxPx, innerPx, mins) → fr`): never below a minimum, sum stays 1.
- [ ] **Step 2:** Playwright `appearance-layout.spec.ts` (system user): a screen size reported by a till beat appears with the cashier's name; pick "Cart full height", Save; reload → still picked; the till at that size shows it; Reset this size + Save → today's.
- [ ] **Step 3:** FAIL → implement → PASS.
- [ ] **Step 4: Gate:** the card at 1440 and at 360 px (preview scales, pictures wrap, bars still draggable), light and dark. Edges checked.
- [ ] **Step 5:** Commit `feat(appearance): Sell page layout per screen size — arrangement, widths, card size`.

### Task 9: Plugins in every arrangement (gate)

**Files:** only under `plugins/*` and `web/src/plugins/*`, as needed.

- [ ] **Step 1:** `make plugins P=all`, `make build`, a layout per arrangement at 1280×1024 and 1366×768 with each section at its minimum width. Open and screenshot: Print & Copy job panel, Reload & Bills panel, 💊 Patient panel (products area); a combo group, a clearance markdown and a warranty line in the cart; a plugin tender (wallet) in Pay; the bottom row with every plugin action; tile badges in S and T.
- [ ] **Step 2:** Plugin till pieces (menus, hints, tenders, actions) move their text onto the till type scale so Text size reaches them; for each piece that clips, overflows or scrolls sideways at Smaller and Larger: fix it **in that plugin** (wrap, `min-w-0`, line-clamp), never by giving core a plugin-specific case. Pocket Cashier unchanged.
- [ ] **Step 3:** `make check-core`, `cd web && bunx vitest run src/plugins`, the plugin Playwright specs touched. `make core-only`.
- [ ] **Step 4:** Commit per plugin fixed: `fix(<plugin>): its till panel fits a narrow products column`.

### Task 10: Docs, shots, full run

**Files:**
- Modify: `docs/guide/appearance.md` (a "Sell page layout" section: who sets it, sizes seen, arrangements, card sizes, Reset), `docs/guide/till.md` (one line: the layout and card size are set by Karots per screen size), `web/docs-shots/shots.spec.ts` (`appearance/sell-layout`, `till/sell-list` with a List layout), `docs/status.md`

- [ ] **Step 1:** Add the shots; `make docs-shots` for them; look at each.
- [ ] **Step 2:** Full run, detached with logs in `.superpowers/`: `make test`, `make test-web`, `make e2e-seed && cd web && bunx playwright test` (re-run lone failures alone; `plugin-aicatalog:278` is a known flake).
- [ ] **Step 3:** `docs/status.md` Now; delete this plan and the spec; commit `docs: Sell page layout`.
