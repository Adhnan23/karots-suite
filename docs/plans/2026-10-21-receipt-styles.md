# Receipt Styles (Image mode) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ten switchable Image-mode receipt styles (Classic + nine new) for every slip, chosen by the system user on
Appearance with a live preview, plus a scannable receipt-number barcode that finds the slip.

**Architecture:** Each style is a **kit** (header, rules, rows, total band, barcode, footer, page) behind one
`SlipKit` interface; the six slip templates are written once against the kit and a style may override a whole
slip. `picture.ts` picks the shop's saved style (`TillConfig.receiptStyle`) and renders `template(kit)`; the
Appearance page renders an unsaved style for preview only. Plugins register slips through `slots.slipTemplates`
and receive the active kit. Search ranks an exact slip number first and ignores the date range for it.

**Tech Stack:** React + TS, `@fontsource` (Inter, Noto Serif + Sinhala / Tamil), a small Code 128 encoder, `qrcode`
(QR), `@zxing/library` (dev, decode tests), Go (Echo, sqlx), Vitest, Playwright.

**Spec:** `docs/specs/2026-10-21-receipt-styles-design.md` · mockups `docs/design/mockups/receipt-styles.html`,
`docs/design/mockups/appearance-receipts.html`

## Global Constraints

- Styles, exact keys and order: `classic, bold, minimal, ticket, poster, grid, stamp, elegant, cards, sections`;
  default `classic`; stored in `settings.receipt_style`; unknown / old (`modern`, `boxed`, `compact`) → `classic`.
- Only the system user writes the style (`PUT /api/v1/system/appearance`); the printed style is always the saved one.
- Classic printing (ESC/POS) has one look and never reads `receipt_style`.
- Every style carries every line `?data=1` gives (`internal/api/dto_slip.go`); a style changes looks, never content.
- 1-bit paper: black or white only (threshold 160), no grey fills; widths 576 / 384 dots; one-column side margin.
- Fonts bundled only; `rasterize` waits for every font the active style uses.
- Barcode: Code 128 set B of the slip number, module ≥ 2 dots, quiet zone ≥ 10 modules, height ≥ 48 dots; on every
  style except Classic; Minimal also a QR of the same number.
- Slip number pattern for scans: `^(S|CR|DP|R)-\d+$` (case-insensitive).
- Core is plugin-inert; files ≤ 500 lines; `make types` after a `dto*.go` change; no Claude attribution in commits;
  every admin screen works at 360 px and in Studio / Night × light / dark.

## Review Focus

1. **A sale that hits every line kind at once** (discount + pack + group + payout + warranty + serial + credit DUE +
   rounding) in a dense style at 58 mm: nothing clipped, nothing missing. Pinned in Task 2 (`every style renders
   every line`, run for all styles at both widths).
2. **Scanning `S-00231` when `S-002310` exists and the sale is older than the "latest" range:** opens S-00231.
   Pinned in Task 9.
3. **A product whose barcode happens to look like a slip number** at Sell: the slip pattern wins (spec: a slip
   number is never a product barcode) — pinned in Task 9 so the rule is explicit, not accidental.
4. **Switching style changes the next print immediately** (cached pictures keyed by style, not just path). Pinned in
   Task 3 (`slip-picture` key includes style) and Task 8's e2e.
5. **A plugin slip under a style the plugin never heard of** still renders through the kit. Pinned in Task 7.

---

### Task 1: The style setting; Classic printing is classic only; Density goes

**Files:**
- Modify: `internal/features/settings/settings.go` (`ReceiptStyles` = the ten keys), `internal/escpos/style.go`
  (`StyleFor` returns classic only; delete modern / boxed / compact; `SamplePreview` stays classic),
  `internal/escpos/style_test.go`, `internal/api/dto.go` (`Appearance.Density` removed; `ReceiptStyle` comment
  lists the ten), `internal/api/dto_till.go` (`TillConfig.ReceiptStyle string json:"receiptStyle"`),
  `internal/api/till_catalog.go` (fill it), `internal/api/system.go` + `me.go` (drop density),
  `web/src/theme/appearance.ts` (no `data-density`), `web/src/theme/tokens.css` (drop the two density blocks; keep
  `--control-h: 46px` on `:root`), `web/src/theme/appearance.test.ts`, `web/src/routes/_authed/admin/system.appearance.tsx`
  (remove the Density and Receipt-style controls — the new section lands in Task 8)
- Test: `internal/features/settings/settings_test.go` (or the existing settings test), `internal/api/system_test.go`

**Interfaces:**
- Produces: `settings.ReceiptStyles []string` (ten keys, in order); `TillConfig.ReceiptStyle` → TS
  `receiptStyle: string`; `Appearance` without `density`.

- [x] **Step 1:** Failing tests: `TestValidReceiptStyle` — each of the ten returns itself; `"modern"`, `"boxed"`,
  `"compact"`, `""`, `"x"` return `"classic"`. `TestStyleForIsClassic` — `StyleFor` with `ReceiptStyle: "bold"`
  equals `StyleFor` with `"classic"`. `system_test`: PUT appearance with `receiptStyle: "ticket"` reads back
  `"ticket"`; `/till/config` returns `receiptStyle: "ticket"`.
- [x] **Step 2:** `go test ./internal/features/settings ./internal/escpos ./internal/api -run 'ReceiptStyle|StyleFor|Appearance'` → FAIL.
- [x] **Step 3:** Implement; `make types`; remove density from the SPA (search `density` in `web/src` — only the
  Appearance page, `appearance.ts`, its test and `tokens.css` use it). The DB column stays (no migration).
- [x] **Step 4:** `make test` and `make test-web` → PASS; `cd web && bunx tsc --noEmit` clean.
- [x] **Step 5:** Commit `feat(settings): receipt_style names the Image style (ten); Classic printing is classic only; Density removed`.

### Task 2: The kit interface; Classic becomes a kit with identical output

**Files:**
- Create: `web/src/features/print/styles/kit.ts` (the `SlipKit` type + `StyleDef`), `web/src/features/print/styles/classic/kit.tsx`
  (today's `kit.tsx` moved behind the interface), `web/src/features/print/styles/index.ts` (`STYLES`, `styleOf`),
  `web/src/features/print/styles/sample.ts` (a `SlipData` fixture with every line kind — for tests and thumbnails),
  `web/src/features/print/styles/styles.test.tsx`
- Modify: `web/src/features/print/templates/SaleSlip.tsx`, `templates/CounterSlips.tsx` (take `kit` as a prop, use
  only kit parts), `web/src/features/print/templates.ts` (kind → template), delete `web/src/features/print/kit.tsx`
  (money helpers move to `styles/money.ts`), `web/src/plugins/repairs/JobSlip.tsx` (imports from the new paths)

**Interfaces:**
- Produces:
  ```ts
  type SlipKit = {
    Page: FC<{ d: SlipData | { dots: number; columns: number }; children: ReactNode }>
    Header: FC<{ shop: SlipShop; dots: number }>     // logo, names, address, phone, VAT
    Title: FC<{ children: string }>                    // "REFUND", "CREDIT PAYMENT"…
    Rule: FC<{ heavy?: boolean }>
    Section: FC<{ children: string }>                   // a section heading (Sections draws a black bar; others may render nothing)
    Item: FC<{ name: string; amount?: string; children?: ReactNode }>   // a line + its sub-lines
    Sub: FC<{ children: ReactNode; amount?: string }>  // sub-line under an item (qty x price, discount, warranty, SN)
    Money: FC<{ l: ReactNode; r: ReactNode; bold?: boolean }>
    Total: FC<{ label: string; amount: string }>        // TOTAL / CHANGE / DUE / GIVEN emphasis
    Text: FC<{ children: ReactNode; center?: boolean; bold?: boolean }>
    Barcode: FC<{ value: string }>                       // Classic: renders nothing
    Footer: FC<{ shop: SlipShop; number?: string }>     // number = slip no, for styles that print the barcode there
    fonts: string[]                                     // families rasterize must wait for
  }
  const STYLE_KEYS = ['classic', 'bold', 'minimal', 'ticket', 'poster', 'grid', 'stamp', 'elegant', 'cards', 'sections'] as const
  type StyleKey = (typeof STYLE_KEYS)[number]
  type StyleDef = { key: StyleKey; label: string; kit: SlipKit; overrides?: Partial<Record<string, FC<{ d: SlipData; kit: SlipKit }>>> }
  const STYLES: StyleDef[]                // in the Global Constraints order; Task 2 registers only classic
  function styleOf(key: string): StyleDef // unknown → classic
  templates: Record<SlipKind, FC<{ d: SlipData; kit: SlipKit }>>
  ```
- Consumes: `SlipData` (types.gen), `PAPER_FAMILY` (`paper.ts`).

- [x] **Step 1:** Before moving anything, write `styles.test.tsx` › `classic is unchanged`: for each kind in the
  fixture (`sale` busy, `sale` with payout below zero, `sale` on credit, refund, money, debt, warranty, test) ×
  `dots 576 / 384`, `renderToStaticMarkup(<Template d={…}/>)` equals a stored snapshot
  (`toMatchSnapshot()`). Run it once on the current code to record the snapshots; commit them with the test.
- [x] **Step 2:** Add `styles.test.tsx` › `every style renders every line`: for each `STYLES` entry × width, the busy
  fixture's markup contains: each item name, `-10%` or the discount amount `245.00`, `1 Card + 2`, the group label
  and both member names, `Given`/the payout amount, `12-month warranty` or `2027-10-21`, the serial `SC-88231`,
  `DUE` / the due amount, `TOTAL` or the total amount `6,265.00`, every tender amount, the change, the shop footer
  text, `Karots Business Suite`. Credit fixture: the TOTAL DUE amount. (Text present, case-insensitive.)
- [x] **Step 3:** Implement the interface and move Classic behind it. Run → `classic is unchanged` PASS (identical
  markup), `every style renders every line` PASS for classic.
- [x] **Step 4:** `make test-web`; `cd web && bunx playwright test print-image` (8/8) — the picture path is untouched.
- [x] **Step 5:** Commit `refactor(print): slips are drawn through a style kit (Classic, identical output)`.

### Task 3: Printing uses the shop's style; barcode + QR parts; style fonts

**Files:**
- Create: `web/src/features/print/barcode.ts` (`code128(value): number[]` module widths, bars first, with start,
  checksum, stop), `web/src/features/print/Barcode.tsx` (`<Code128 value height modules>` and `<QR value size>` as
  black `<div>`/SVG rects at whole-dot widths), `web/src/features/print/barcode.test.ts`
- Modify: `web/src/features/print/picture.ts` (style from `configQuery` → `templates[kind]` or
  `style.overrides[kind]` with `kit: style.kit`; the `slip-picture` query key gains the style), `raster.ts`
  (`rasterize(node, dots, fonts: string[])` checks each family), `paper.ts` (register the new `@fontsource`
  imports), `web/package.json` (`@fontsource/inter`, `@fontsource/noto-serif`, `@fontsource/noto-serif-sinhala`,
  `@fontsource/noto-serif-tamil`, `qrcode`, dev `@zxing/library`)

**Interfaces:**
- Produces: `code128(value: string): number[]`; `Code128: FC<{ value: string; module?: number; height?: number }>`
  (defaults module 2, height 48); `QR: FC<{ value: string; size: number }>`; `slipPictureQuery(path, size?, style?)`
  where `style` defaults to the saved one; `rasterize(node, dots, fonts)`.
- Consumes: `STYLES`, `styleOf`, `templates` (Task 2).

- [x] **Step 1:** Failing `barcode.test.ts`: `code128('S-00231')` starts with Start B (`2,1,1,2,1,4`) and ends with
  stop (`2,3,3,1,1,1,2`), sum of modules = 11 × (chars + 3) + 2; decoding the bar row rendered at module 2 with
  `@zxing/library` `Code128Reader` returns `'S-00231'`; same for `CR-000018`, `DP-000002`, `R-0002`.
- [x] **Step 2:** `make test-web` → FAIL. **Step 3:** Implement; picture.ts reads `cfg.receiptStyle`.
- [x] **Step 4:** `make test-web` PASS; `print-image.spec` 8/8 (still Classic).
- [x] **Step 5:** Commit `feat(print): slips print in the shop's style; Code 128 + QR parts; style fonts`.

### Task 4: Bold, Minimal, Ticket

**Files:** Create `web/src/features/print/styles/{bold,minimal,ticket}/kit.tsx` (+ `sale.tsx` only where the shared
sale can't draw the mockup); register in `styles/index.ts`.

- [x] **Step 1:** Build each to match `docs/design/mockups/receipt-styles.html` (Bold: black header band, sans,
  tags for savings, black TOTAL band, barcode + number at the foot; Minimal: light sans, dotted leaders, big quiet
  total, barcode + QR; Ticket: number big in a framed box with its barcode at the top, boxed totals, tear line).
  The credit block, payouts and rounding follow each style's money / total parts.
- [x] **Step 2:** `every style renders every line` passes for the three (it iterates `STYLES`).
- [x] **Step 3:** Shot review in the browser (decoded-free: just the pictures) of the busy sale, a credit sale and
  a refund at 80 + 58 mm for each; nothing clipped, savings visible, barcode at whole dots. Fix until clean.
- [x] **Step 4:** Commit `feat(print): Bold, Minimal and Ticket receipt styles`.

### Task 5: Poster, Grid, Stamp

**Files:** `web/src/features/print/styles/{poster,grid,stamp}/` (Poster and Grid need a `sale.tsx` override: headline
total / table), register.

- [x] **Step 1:** Build to the mockup (Poster: "YOU PAID" headline total first, shop name sideways down the edge —
  at 58 mm the sideways name may move to the top; Grid: QTY / ITEM / OFF / AMT table with ruled cells, totals block
  right; Stamp: outlined shop name, tilted round "PAID" stamp in the header corner — never over a number).
- [x] **Step 2–4:** as Task 4 (tests iterate `STYLES`; shot review; commit `feat(print): Poster, Grid and Stamp receipt styles`).

### Task 6: Elegant, Cards, Sections

**Files:** `web/src/features/print/styles/{elegant,cards,sections}/` (Sections needs a `sale.tsx`: ITEMS / YOU
SAVED / PAYMENT blocks), register.

- [x] **Step 1:** Build to the mockup (Elegant: Noto Serif, centred, ◆ ornaments, italic item names, total between
  double rules; Cards: each item in a rounded box, black rounded total pill; Sections: black section bars, every
  saving collected under YOU SAVED).
- [x] **Step 2–4:** as Task 4; commit `feat(print): Elegant, Cards and Sections receipt styles`.

### Task 7: Plugin seam; Repairs on the kit

**Files:**
- Modify: `web/src/plugins/slots.ts` (`SlipTemplate = { kind: string; label: string; sample?: string; Template: FC<{ d: unknown; kit: SlipKit; style: string }> ; overrides?: Partial<Record<StyleKey, FC<…>>> }`),
  `web/src/features/print/picture.ts` (plugin kinds get the kit), `web/src/plugins/repairs/JobSlip.tsx` (kit parts
  only), `web/src/plugins/repairs/index.tsx` (label "Repair", sample path), `plugins/repairs/api_print.go` + a
  `POST /x/repairs/sample-slip` (`?data=1` only: a made-up open job with parts, a charge, an estimate and a deposit;
  anything else → 422, never prints), `AGENTS.md` slots line
- Test: `plugins/repairs/print_db_test.go` (sample-slip), `web/src/features/print/styles/styles.test.tsx`

- [x] **Step 1:** Failing tests: Go — `sample-slip?data=1` returns kind `repairs.job` with a ticket and an estimate;
  without `data=1` → 4xx. Vitest — `a plugin slip follows every style`: a fake registered `{ kind: 'x.test', Template }`
  built from kit parts renders for every `STYLES` key without throwing and contains its text.
- [x] **Step 2:** Run → FAIL. **Step 3:** Implement. **Step 4:** `make plugins P=all`, `go test ./plugins/repairs`,
  `make test-web`, `bunx playwright test plugin-repairs`, then `make core-only && make check-core` → OK.
- [x] **Step 5:** Commit `feat(print): plugin slips get the style kit (slipTemplates); Repairs follows every style`.

### Task 8: Appearance → Receipt style (system user)

**Files:**
- Create: `web/src/features/appearance/ReceiptStyles.tsx` (thumbnails grid + preview + switches),
  `web/e2e/appearance-receipts.spec.ts`
- Modify: `web/src/routes/_authed/admin/system.appearance.tsx` (the section, per
  `docs/design/mockups/appearance-receipts.html`), `internal/api/admin_settings.go` (`sample-slip` takes
  `?kind=sale|refund|money|debt|warranty|test` and `?sample=busy|simple|credit`; busy = the Task 2 fixture's lines,
  credit = a customer sale with DUE and TOTAL DUE), `internal/api/slip_data_test.go`

**Interfaces:**
- Consumes: `STYLES`, `slipPictureQuery(path, size, style)` (preview of an unsaved style), plugin `slipTemplates`
  (`label`, `sample`).

- [x] **Step 1:** Failing tests: Go — `sample-slip?kind=refund&data=1` → kind `refund`; `?kind=sale&sample=credit&data=1`
  → `sale.credit == true`. e2e (system user, 2273 dev PIN): open Appearance; ten thumbnails (`img` named after
  each style); pick Ticket → the preview `img` src changes; switch slip Refund and 58 mm → preview natural width 384;
  Save → `/till/config` has `receiptStyle: ticket`; with Image mode on, a cash sale's print upload differs from the
  Classic one taken before (bytes). Reset style to classic and print_as in `afterAll`.
- [x] **Step 2:** Run → FAIL. **Step 3:** Implement (the yellow "prints Classic" note when `printAs` is classic;
  thumbnails are `slipPicture` renders of the busy sample at 80 mm scaled to the card; at 360 px the grid becomes a
  horizontal scroller and the preview sits under it).
- [x] **Step 4:** Tests pass; shots: the page in Studio / Night × light / dark, desktop and 360 px (no side scroll).
- [x] **Step 5:** Commit `feat(appearance): pick the receipt style with a live preview of every slip`.

### Task 9: Scan a slip to find it

**Files:**
- Modify: `internal/features/sales/store_query.go` (`List`: when `q` equals a sale's `receipt_no`, ignoring case,
  that sale is first and the date range doesn't apply to it), the money-receipt and customer-payment list queries
  (same rule for `CR-` / `DP-`), `web/src/routes/_authed/till/receipts.tsx` (`?no=` search param → the find is filled
  and the slip opens in the right list), `web/src/features/counter/receipts/ReceiptsTab.tsx`,
  `web/src/features/till/catalog/useEntry.ts` (a barcode entry matching the slip pattern navigates to
  `/till/receipts?no=<value>` instead of a product lookup), the Returns `SaleFind` and the Warranty "by receipt" box
  (Enter on a slip number opens it — they share `SaleFind`, check Warranty's own box)
- Test: `internal/api/counter_returns_test.go` (or a new `scan_find_test.go`), `web/e2e/scan-find.spec.ts`

- [ ] **Step 1:** Failing tests: Go — two sales `S-…n` and a later one whose number contains it (insert
  `receipt_no` = `<n>0` directly), `GET /till/sales?q=<n>&preset=today` with the first sale's `created_at` moved
  to last year → first row is the exact sale; same for `/admin/sales`; CR- and DP- lists likewise. e2e — at Sell,
  type a sale's number + Enter in the scan box → URL is `/till/receipts?no=…` and that receipt is open; at Returns,
  type it + Enter → that sale is picked; a product whose barcode is `S-99999` (test data) still opens the receipt
  route (the slip pattern wins — spec).
- [ ] **Step 2:** Run → FAIL. **Step 3:** Implement. **Step 4:** Go + e2e pass; full `bunx playwright test` green.
- [ ] **Step 5:** Commit `feat(till): scan a slip's barcode to open it — exact number first, any date; Sell jumps to Receipts`.

### Task 10: A4 page in the shop's style; close-out

**Files:** Modify `web/src/routes/_authed/print/receipt.$id.tsx` (the shop's style kit), `docs/status.md`,
`docs/design/theme.md` (receipt styles section, link the mockups), `AGENTS.md` (Layout: `features/print/styles/`).

- [ ] **Step 1:** The A4 page renders `templates.sale` (or the style's override) with the saved style; check it in
  Firefox's print layout.
- [ ] **Step 2:** `make verify`, `make build && make verify-portable`, full Playwright run green.
- [ ] **Step 3:** Final shot set for the user (UI gate): every style × busy sale at 80 + 58 mm and one non-sale slip
  each; the Appearance page. Record the result in `docs/status.md`; update memory.
- [ ] **Step 4:** Commit `docs: status — receipt styles built`.
