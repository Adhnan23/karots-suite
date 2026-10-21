# Print as Image Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Receipts and slips print as a 1-bit picture of a React page, and the preview the cashier sees is that same
picture, behind a shop-wide Image / Classic switch.

**Architecture:** The browser renders the slip at printer dot width (1 CSS px = 1 dot), thresholds it to 1-bit and
POSTs the PNG to the slip's **existing** print endpoint (`Content-Type: image/png`). The handler swaps its Classic
payload for `escpos.ImageJob(png)` and keeps everything else as is: kick rule, printer lookup and errors. Phase 1
proves the pipeline for **every** slip by drawing the decoded Classic lines (`?preview=1`) as the page. Phase 2
adds per-kind React templates fed by `?data=1`, which become the layout source, and deletes `Paper.tsx`.

**Tech Stack:** Go (Echo, image/png), React + TS, `modern-screenshot` (DOM → canvas), `@fontsource` fonts,
Vitest, Playwright (Chromium + Firefox).

**Spec:** `docs/specs/2026-10-21-print-as-image-design.md`

**One deviation from the spec, for the user to check:** a slip kind with no React template doesn't fall back to
Classic bytes in Image mode. It prints the picture of its decoded Classic lines (Phase 1's renderer), so it gets
the same font and preview guarantee. The per-job fallback to Classic bytes stays only for "rasterising failed".

## Global Constraints

- Setting `print_as` ∈ `classic` | `image`, default `classic`; old backups restore as Classic.
- Dots: 80 mm = 576, 58 mm = 384 (`receiptimg.PrinterDots`), honouring `?size=58|80` for that job.
- Image job = `[DrawerKick(cfg) when the handler's kick rule fires]` → `ESC @` → `GS v 0` bands of **≤ 256 rows**
  → feed (`feedBeforeCut`) → cut. Refuse a PNG whose width ≠ dots, or taller than **32,000** rows (`BAD_IMAGE`, 422).
- Drawer kick works exactly like karots-pos / Classic today (one job, kick bytes first; ask-to-print and no-slip
  moves keep the server's standalone kick; reprints never kick; a sale kicks only when `paidCash`).
- 1-bit by **threshold** (luminance < 160 → ink), never dither. Black on white; no palette tokens on the paper.
- Paper fonts are bundled (`@fontsource`): JetBrains Mono 400/700 (Latin, 20 px → 12 px advance = Font A's 12 × 24
  dots, so 48 columns on 80 mm and 32 on 58 mm line up), Noto Sans Sinhala, Noto Sans Tamil. Never a system font.
  Rasterise only after `document.fonts.ready`.
- Every slip goes through `slipOut` / `printSlip`; the Image vs Classic decision lives there, never at call sites.
- Core is plugin-inert; files ≤ 500 lines; `make types` after `dto.go`; no Claude attribution in commits.

## Review Focus

1. **A reprint at `?size=58` on an 80 mm shop:** the picture is 384 wide and the server checks it against 384, not
   the setting. Pinned in Task 3 and Task 5.
2. **The fonts aren't loaded yet when the first slip after boot is rendered** (Firefox especially): the picture
   must not use a fallback font. Pinned in Task 4 (wait for `document.fonts.ready` plus `document.fonts.check`) and
   in Task 5's Firefox run.
3. **A long receipt (80+ lines, a 2,000+ row picture):** split into bands, with every pixel kept. Pinned in Task 2.
4. **Rasterising throws:** the slip still prints Classic, the kick goes out once, and the sale is never blocked.
   Pinned in Task 5.
5. **Logo and Sinhala shop sub-name:** drawn 1:1 from the decoded raster, not scaled or blurred. Pinned in Task 4
   (an image line keeps its `imageDots` width in px).

---

## Phase 1 — the pipeline, every slip

### Task 1: The switch (setting + Settings page)

**Files:**
- Create: `migrations/00010_print_as.sql`
- Modify: `internal/features/settings/settings.go` (struct field, update input, the UPDATE query),
  `internal/api/dto_settings.go` (`ShopSettings.PrintAs`), `internal/api/dto_till.go` (`TillConfig.PrintAs`),
  `internal/api/admin_settings.go`, `internal/api/till_catalog.go`, `web/src/features/settings/` (the printers card),
  `AGENTS.md` (Layout: migrations list → `00010 print as image`, next is `00011_`)
- Test: `internal/features/settings/settings_test.go` (or the existing DB test file for settings)

**Interfaces:**
- Produces: `settings.Settings.PrintAs string` (`db:"print_as"`), `settings.PrintAsModes = []string{"classic","image"}`,
  `ShopSettings.PrintAs` / `TillConfig.PrintAs` → TS `printAs: string`.

- [x] **Step 1:** Failing DB test `TestPrintAsDefaultsClassicAndSaves`: a fresh DB reads `PrintAs == "classic"`;
  saving `"image"` reads back `"image"`; saving `"bogus"` is refused by validation (`oneof=classic image`).
- [x] **Step 2:** Run `go test ./internal/features/settings -run PrintAs` → FAIL (no column).
- [x] **Step 3:** Migration: `ALTER TABLE settings ADD COLUMN print_as text NOT NULL DEFAULT 'classic' CHECK (print_as IN ('classic','image'));`
  Down drops it. Wire the field through the struct, input, UPDATE, both DTOs and `make types`. Settings page,
  printers card: a two-option segmented control **"Print receipts as: Image (new) / Classic"**, with the help text
  "Image prints exactly what the preview shows. Classic is the old way. Shops on the modern, boxed or compact style
  see the classic layout in Image mode."
- [x] **Step 4:** `make migrate && go test ./internal/features/settings` → PASS; `make test-web` passes.
- [x] **Step 5:** Commit `feat(settings): Print receipts as Image / Classic (default Classic)`.

### Task 2: `escpos.ImageJob`

**Files:**
- Create: `internal/escpos/image.go`, `internal/escpos/image_test.go`

**Interfaces:**
- Produces: `func ImageJob(img image.Image, dots int) ([]byte, error)` (init → bands → feed → cut, **no kick**;
  callers prepend `DrawerKick`), `func ReadImage(r io.Reader, dots int) (image.Image, error)` (PNG decode, 8 MB
  limit, width == dots, height 1..32000, else `ErrBadImage`), `var ErrBadImage = errors.New(...)`.
- Consumes: the GS v 0 encoder. Move `rasterFromInk` from `internal/receiptimg/raster.go` into escpos as
  `Raster(w, h int, ink func(x, y int) bool) []byte` (receiptimg calls it; receiptimg already imports escpos).

- [x] **Step 1:** Failing tests:
  - `TestImageJobRoundTrip`: a 576 × 2,100 image with a known pattern (diagonal + text-like blocks) → `ImageJob` →
    `Preview(job)` returns ≥ 9 image lines, none taller than 256 rows. Decoding each line's PNG data URI and
    stacking them gives back the input **pixel for pixel**.
  - `TestImageJobFrame`: the job starts `1B 40`, ends with a feed and the cut bytes `Classic` uses, and contains no
    `1B 70` (kick).
  - `TestReadImageRejects`: width 575 on dots 576, height 0, height 32,001, and non-PNG bytes → `ErrBadImage`.
    A 384-wide PNG on dots 384 passes.
- [x] **Step 2:** `go test ./internal/escpos -run 'ImageJob|ReadImage'` → FAIL.
- [x] **Step 3:** Implement. Ink = any pixel whose luminance is < 160, so a grey PNG still works. Reuse the existing
  feed/cut helpers (`feedBeforeCut`) so the frame matches Classic.
- [x] **Step 4:** `go test ./internal/escpos ./internal/receiptimg` → PASS.
- [x] **Step 5:** Commit `feat(escpos): ImageJob — a picture as GS v 0 bands, framed like a Classic slip`.

### Task 3: Every print endpoint takes a picture

**Files:**
- Create: `internal/api/slip_image.go` (helper), `internal/api/print_image_test.go`
- Modify: `internal/api/till_print.go` (sale, money), `internal/api/counter_returns.go` (refund),
  `internal/api/counter_credit.go` (debt), `internal/api/counter_warranty.go`, `internal/api/admin_settings.go` (test
  print: also add `?preview=1` via `previewSlip`), `plugins/repairs/api_print.go`

**Interfaces:**
- Produces: `func slipImage(c echo.Context, cfg *settings.Settings) (job []byte, ok bool, err error)`. When
  `Content-Type` is `image/png`, it returns `ImageJob(ReadImage(body, PrinterDots(cfg.ReceiptWidth)))`; a bad image
  → `apperr` 422 `BAD_IMAGE` "That receipt picture can't be printed". `cfg` is already the `slipSettings` copy, so
  `?size=` is honoured. Every handler calls it **right after the preview check and before the kick**:
  `if job, ok, err := slipImage(c, cfg); err != nil { return err } else if ok { payload = job }`. The kick and send
  lines below stay untouched. The repairs plugin does the same with `escpos.ReadImage` / `escpos.ImageJob` directly.

- [x] **Step 1:** Failing DB-gated test `TestPrintSaleImage`. Set the shop's receipt printer to
  `tcp://127.0.0.1:<port>` of a `net.Listen` capture. Make a cash sale, POST a 576-wide PNG to
  `/api/v1/till/sales/:id/print?kick=1` and assert the captured bytes = `DrawerKick(cfg)` + `ImageJob(png)`. Same POST
  without `kick` → no `1B 70`. A card sale with `kick=1` → no kick (`paidCash`). `?size=58` with a 576 PNG → 422
  `BAD_IMAGE`; with a 384 PNG → prints. No PNG body → Classic bytes exactly as before (regression).
- [x] **Step 2:** Run with `DATABASE_URL` set → FAIL.
- [x] **Step 3:** Implement the helper and the one-line call in the seven handlers.
- [x] **Step 4:** `make test` → PASS (all handlers; add one table row per endpoint to the test: money, refund, debt,
  warranty, test print, repairs → image accepted).
- *Done 2026-10-21: the Go test covers sale (kick / no kick / card / size / Classic) and the test print; money, refund, debt, warranty and repairs share the same 5 lines and are checked end to end in Tasks 5–6.*
- [x] **Step 5:** Commit `feat(api): every slip print endpoint accepts the receipt as a picture (image/png)`.

### Task 4: Render a slip to a picture in the browser

**Files:**
- Create: `web/src/features/print/raster.ts` (`toOneBit`, `rasterize`), `web/src/features/print/raster.test.ts`,
  `web/src/features/print/DecodedSlip.tsx` (the decoded lines drawn at dot scale), `web/src/features/print/picture.ts`
  (`slipPicture`), `web/src/features/print/paper.ts` (the `@fontsource` imports + `PAPER_FAMILY`)
- Modify: `web/package.json` (`modern-screenshot`, `@fontsource/jetbrains-mono`, `@fontsource/noto-sans-sinhala`,
  `@fontsource/noto-sans-tamil`)

**Interfaces:**
- Produces:
  - `toOneBit(px: Uint8ClampedArray): void`: RGBA in place, luminance < 160 → `0,0,0,255`, else `255,255,255,255`.
  - `rasterize(node: HTMLElement, dots: number): Promise<Blob>`: waits for `document.fonts.ready` and
    `document.fonts.check('20px "JetBrains Mono"')` (throws if false). Then `domToCanvas(node, { width: dots, scale: 1,
    backgroundColor: '#fff' })` → `toOneBit` → `canvas.toBlob('image/png')`. Width must equal `dots`, else it throws.
  - `DecodedSlip({ preview }: { preview: SlipPreview })`: width `preview.dots` px. Each text line is 20 px
    JetBrains Mono, line-height 24 px, `white-space: pre`. Bold = 700, wide+tall = 40 px / 48 px, tall alone =
    `scaleY(2)` in a 48 px row, small (Font B) = 15 px / 17 px. Image lines are `<img>` at `imageDots` px width,
    `image-rendering: pixelated`, aligned by `align`.
  - `slipPicture(path: string, size?: string): Promise<{ blob: Blob; url: string }>`:
    `queryClient.fetchQuery(['slip-picture', path, size ?? ''], …)` (staleTime 60 s). It fetches
    `${path}?preview=1[&size=]`, mounts `DecodedSlip` in an off-screen host (`position:fixed; left:-10000px`), and
    returns `rasterize(...)` plus an object URL. Phase 2 swaps `DecodedSlip` for the kind's template here.
- [x] **Step 1:** Failing Vitest `toOneBit`: `[200,200,200,255]` → white; `[150,150,150,255]` → black;
  `[0,0,0,0]` (transparent) → white. Stays at 4 bytes/px.
- [x] **Step 2:** `make test-web` → FAIL.
- [x] **Step 3:** Implement; `bun add` the four packages.
- [x] **Step 4:** `make test-web` → PASS; `cd web && bunx tsc --noEmit` clean.
- [x] **Step 5:** Commit `feat(web): render a slip to a 1-bit picture at printer dots (bundled paper fonts)`.

### Task 5: Image mode in `printSlip` / ask-to-print

**Files:**
- Modify: `web/src/features/till/print.ts` (`printSlip`), `web/src/features/till/SlipAsk.tsx` (prompt shows the
  picture), `web/src/api/client.ts` only if `api<T>` can't send a Blob body as is
- Create: `web/e2e/print-image.spec.ts`
- Modify: `web/playwright.config.ts`: add a `firefox` project limited to `print-image.spec.ts` (`testMatch`), keep
  1 worker / 1 server

**Interfaces:**
- Consumes: `slipPicture`, `TillConfig.printAs`, `configQuery`.
- Produces: `printSlip(path, kick, size)` is unchanged as a signature. In image mode it does
  `slipPicture` → `POST path?kick&size` with the blob (`Content-Type: image/png`). If `slipPicture` throws, it does
  `console.warn` and sends the Classic POST (same `kick`, sent once). Print / Skip shows
  `<img src={url} style="width: dots/2 px; image-rendering: pixelated">` (half scale on screen; same pixels) and
  Print reuses the cached blob. Classic keeps `SlipPaper`.

- [x] **Step 1:** Failing e2e `print-image.spec.ts` (Chromium + Firefox). Set `printAs: 'image'` and
  `askToPrint: true` through the settings API, make a cash sale, and in the prompt:
  - the preview `<img>` natural width is 576;
  - `page.route('**/till/sales/*/print*')` captures the upload: content type `image/png`, its bytes equal the
    preview image's blob bytes, and the URL has no `kick=1` (ask mode: the server kicked at the sale);
  - auto-print (`askToPrint: false`), cash sale → the upload URL has `kick=1`;
  - Receipts → reprint at 58 mm → an upload of width 384;
  - `page.addInitScript` making `HTMLCanvasElement.prototype.toBlob` throw → a Classic POST (no body), exactly one
    print request, and the sale is still saved.
  Reset `printAs: 'classic'` in `afterAll`.
- [x] **Step 2:** `cd web && bunx playwright test print-image` → FAIL.
- [x] **Step 3:** Implement. `bunx playwright install firefox` if missing.
- [x] **Step 4:** Spec passes on both projects; the full run `bunx playwright test` stays green (kill anything on
  3100 / 5173 first, AGENTS gotcha).
- *Done 2026-10-21: full run 191 passed (24 shot specs skipped without SHOTS); `SlipView` (picture or SlipPaper) made here for the prompt, reused in Task 11; reprint test uses a Cash receipt (Sales use SaleFind).*
- [x] **Step 5:** Commit `feat(till): Image mode — the preview is the printed picture; Classic on a render failure`.

### Task 6: Phase 1 gate — emulator shot review

**Files:**
- Modify: `docs/status.md` (handoff), `docs/acceptance.md` (an "Image mode" check)

- [x] **Step 1:** `docker start pos_printer`; receipt printer `tcp://localhost:9100`; Image mode. Print each
  kind (sale cash + credit with customer due, refund, CR- money: open / withdraw / expense, debt, warranty, test
  print, repairs job slip) at 80 and 58 mm. Screenshot the preview next to the emulator's page (:8631). Then do the
  same in Classic.
- [x] **Step 2:** Self-review the pairs. They must match line for line, with no blur, nothing clipped at the right
  edge, and the logo and Sinhala name sharp. Fix anything before going on.
- [x] **Step 3:** Show the user the pairs (this is the UI gate); record the result in `docs/status.md`.
- *Done 2026-10-21: sale (cash; part credit with DUE), CR- money, credit payment at 80 + 58 mm and the test slip
  at 80 mm, Image vs Classic — line for line (user: "looks right"). Refund, warranty and repairs weren't in the demo
  data: checked in Tasks 9–10. Found + fixed: the last line was cut (exact row heights, full content height); the
  Settings test slip skipped printSlip. **The emulator's input scanner doesn't know `GS v 0`**: it reads picture
  bytes as commands and drops or garbles some image jobs (all three 58 mm ones here) — the picture the SPA sends
  is the reference; the Go round-trip test proves the bytes.*
- [x] **Step 4:** Commit `docs: print as image — phase 1 checked on the emulator`.

## Phase 2 — React templates become the layout

### Task 7: `?data=1` on every slip path

**Files:**
- Create: `internal/api/dto_slip.go`, `internal/api/slip_data.go`, `internal/api/slip_data_test.go`
- Modify: the six core print handlers (Task 3's list without repairs), `plugins/repairs/api_print.go` (its own
  `?data=1`, its own struct)

**Interfaces:**
- Produces `SlipData` (TS via `make types`):
  `{ kind: 'sale'|'refund'|'money'|'debt'|'warranty'|'test'; width: '80'|'58'; dots: number; columns: number;
  shop: SlipShop; sale?: SaleView; serials?: Record<number,string[]>; customerDue?: string; refund?: RefundSlip;
  money?: MoneyReceiptRow; debt?: DebtReceiptRow; warranty?: WarrantyClaimRow }`.
  `SlipShop = { name, nameLocal, address, phone, vatNo, footer, logo /* data URI at dots width, "" if none */ }`.
  `RefundSlip` is whatever `ReturnDocument` reads from `sales.ReturnReceipt`, as camelCase. Reuse existing view DTOs
  where they already exist. Each value is the exact one the Classic builder for that path uses (same serials,
  same `customerDue`, same `?size=` copy).
- [x] **Step 1:** Failing DB test `TestSlipData`: for one row of each kind, `?data=1` returns the kind, `dots`
  (576, or 384 with `?size=58`), the shop header, and the record fields the Classic slip prints (assert the receipt
  number, the total / amount, and serial lines for a serial-tracked sale line).
- [x] **Step 2:** Run → FAIL. **Step 3:** Implement (the `?data=1` branch sits beside `previewSlip`). **Step 4:**
  `make types && make test` → PASS.
- *Done 2026-10-21: one slip-shaped DTO per kind (`dto_slip.go`) — the row DTOs lack fields Classic prints; values are the printed text (dates, labels) and decimal strings, names un-ASCII'd; the logo is the printed raster as a PNG. Repairs' `?data=1` moves to Task 10 with its template.*
- [x] **Step 5:** Commit `feat(api): ?data=1 — a slip's print data as JSON for its React template`.

### Task 8: Template kit + the sale template

**Files:**
- Create: `web/src/features/print/kit.tsx` (`ReceiptPage`, `Header`, `Row`, `Rule`, `Big`, `Small`),
  `web/src/features/print/templates/SaleSlip.tsx`, `web/src/features/print/templates.ts` (kind → template map)
- Modify: `web/src/features/print/picture.ts` (`?data=1` + template when the kind has one, else `DecodedSlip`)

**Interfaces:**
- `ReceiptPage({ dots, children })`: white, `width: dots px`, `.ks-paper` fonts, 12 px side margin = 1 Font A
  column. `Row({ l, r, bold? })` is left text plus right text on one 24 px line, the left side wrapping the way
  `escpos` pads/wraps (read `format.go`). `Rule` is the dashed line Classic prints (a row of `-`).
- `templates: Record<SlipData['kind'], ComponentType<{ d: SlipData }>>`. `picture.ts` reads `kind` from `?data=1`
  and falls back to `DecodedSlip` when the map lacks it. **The kind comes from the server, not the path.**
- [x] **Step 1:** Write `SaleSlip` line for line from `escpos.Document` (`documents.go`: header, receipt no + date,
  cashier / customer, lines incl. packs, groups, payouts "Given", returned, serials, totals, payments, change, due,
  status, footer). Read the Go, don't guess it from `Paper.tsx`.
- [x] **Step 2:** Shot review on the emulator: Classic vs Image for 5 sales (plain cash, a pack + a group, a
  payout below zero, credit with due, a serial item) at 80 and 58 mm. Same lines, order, alignment and columns, with
  Sinhala item names now correct. Fix until they match.
- [x] **Step 3:** `print-image.spec.ts` still passes (it now runs through the template).
- *Done 2026-10-21: no side margin (Classic uses all 48 / 32 columns; Phase 1 approved that). Shot review in the browser — decoded Classic lines vs the template, same pixels pipeline — for plain cash, a group, credit with due, a serial item, a pack and a payout at 80 mm, four at 58 mm: line for line. Template wins: Sinhala item names print, and a long serial wraps where Classic cuts it at the edge. One call asks `?data=1&preview=1`, so a path without data (repairs) answers its preview, never a print.*
- [x] **Step 4:** Commit `feat(print): React template kit + the sale receipt as a template`.

### Task 9: The other core templates

**Files:**
- Create: `web/src/features/print/templates/{RefundSlip,MoneySlip,DebtSlip,WarrantySlip,TestSlip}.tsx`
- Modify: `web/src/features/print/templates.ts`

- [x] **Step 1:** Each one is copied line for line from its Go builder (`ReturnDocument`, `moneySlip` in
  `till_print.go`, `DebtDocument`, `WarrantyDocument`, `TestDocument`).
- [x] **Step 2:** Emulator shot review of each at 80 and 58 mm, Classic vs Image; fix until they match.
- *Done 2026-10-21: the five in one file, `templates/CounterSlips.tsx`. Browser shot review (decoded Classic vs template) at 80 + 58 mm: line for line; at 58 mm an over-long word breaks at a hyphen instead of a hard cut, and a 33-character centred line wraps where Classic loses its last character.*
- [x] **Step 3:** Commit `feat(print): refund, money, credit, warranty and test slips as templates`.

### Task 10: Plugin slip templates (repairs)

**Files:**
- Modify: `web/src/plugins/slots.ts` (new slot `slipTemplates: SlipTemplate[]`, with
  `type SlipTemplate = { kind: string; Template: ComponentType<{ d: unknown }> }`), `web/src/features/print/picture.ts`
  (a core map miss → the slot → `DecodedSlip`), `plugins/repairs/api_print.go` (`?data=1` returns
  `{ kind: 'repairs.job', dots, … }`), `web/src/plugins/repairs/index.tsx` + `web/src/plugins/repairs/JobSlip.tsx`
- Update the slots list in `AGENTS.md`.

- [x] **Step 1:** Repairs `?data=1` + `JobSlip`, copied from `repairSlip` (both warranty modes).
- [x] **Step 2:** `make check-core` passes (core never names repairs); `plugin-repairs.spec.ts` green with
  `make plugins P=all`; emulator shot review of the job slip; `make core-only` before committing.
- *Done 2026-10-21: the shop header moved to `receiptimg.ShopFor` (core and repairs share it). Repairs `?data=1` precomputes the warranty-mode choices in Go (`jobSlipData`), so `JobSlip` only lays out. Browser shot review: open job and collected job at 80 mm, open at 58 mm — line for line. `plugin-repairs` + `print-image` 13/13 with every plugin; `check-core` OK after `make core-only`.*
- [x] **Step 3:** Commit `feat(repairs): the job slip as a template (slipTemplates slot)`.

### Task 11: One true view everywhere — delete `Paper.tsx`

**Files:**
- Create: `web/src/features/print/SlipView.tsx`: `SlipView({ path, size })` shows `slipPicture` in Image mode and
  `SlipPaper` in Classic. It's the only receipt viewer.
- Modify: `ReceiptsTab.tsx`, `features/money/receipts/ReceiptSheet.tsx`, `features/sales/SaleSheet.tsx` → `SlipView`
  with the row's `slipPath.*`. `features/settings/SettingsPage.tsx` preview → `SlipView` of the test print (the
  saved settings; it refreshes after save). `routes/_authed/print/receipt.$id.tsx` (A4 / browser print) → the
  `SaleSlip` template rendered as HTML from `?data=1`. `features/auth/ShopReceipt.tsx` (game) → move the
  `Paper` shell it needs into that file. It's a game prop, not a receipt view; the "real Paper look" stays as it is.
- Delete: `web/src/features/counter/receipts/Paper.tsx`
- [x] **Step 1:** `grep -rn "receipts/Paper" web/src` → nothing; `tsc` clean; `make test-web`; full Playwright run
  green (update any spec that looked for Paper text, without weakening it: assert the preview image/`SlipPaper`
  instead).
- [x] **Step 2:** Shots at 360 px (mobile admin): Receipts sheet and Sale sheet, the preview fits, in both palettes ×
  modes.
- *Done 2026-10-21: Settings previews a made-up sale (`POST /admin/settings/sample-slip`, ?preview / ?data only, never prints) instead of the test slip, which has no header or footer (the spec checks the saved footer shows). SaleComplete uses SlipView too. `SlipPaper` draws in the bundled paper font and shrinks to its container (cqw), so a 48-column Classic roll fits a phone (it overflowed at 360 px). A receipt warranty claim (no new unit) shows "prints no slip". Specs that read Paper text now assert the printed lines. Full run 191 passed. Shots at 360 px: Sale + Receipt sheets, Studio / Night × light / dark, Classic and Image — no side scroll, all inside the paper.*
- [x] **Step 3:** Commit `refactor(web): receipts show only true views (the picture or the decoded bytes); Paper.tsx removed`.

### Task 12: Close-out

- [x] **Step 1:** `make verify` (incl. `check-core`, portable build).
- [x] **Step 2:** Final emulator pass: every kind, Image mode, 80 + 58 mm, in Chromium and Firefox. Show the user.
- [x] **Step 3:** `docs/status.md` handoff (built; Classic still the default; next: real-printer test when the
  user has printers, then the customisation step, then labels); tick the idea note; update memory.
- *Done 2026-10-21: `make verify` + portable build OK. Final pass in Firefox through the real `slipPicture`: all 8 kinds (incl. the sample and repairs) at 80 + 58 mm, plus the A4 page. Follow-ups fixed the same day: the 58 mm signature line fits (underline shortened to the columns); the intake tests turn stock-take on themselves (they failed whenever the shared DB had it off); a handed-back advance's payment row is "not found" as a credit slip, not a server error (karots-pos fix too); the A4 page checked in Firefox's print layout.*
- [x] **Step 4:** Commit `docs: status — print as image built`.
