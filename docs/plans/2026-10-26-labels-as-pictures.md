# Labels as Pictures + Label Styles Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** In Image mode every label (price, Pharmacy dose, Repairs ticket) is a React template drawn at label dots,
printed as a TSPL `BITMAP` and previewed as that picture, in a per-kind style picked on Appearance; Classic stays
the default and unchanged.

**Architecture:** A label template renders a fixed `w×h` dot box; `rasterize` (shared with slips) turns it into a
1-bit PNG; the existing label endpoints accept that PNG (`?count=&size=`) and wrap it as TSPL. Styles are per kind
(`settings.label_styles` JSON map); core owns kind `price`, plugins register theirs via `slots.labelTemplates`. One
web helper (`printLabel`) decides Image vs Classic, so call sites never branch.

**Tech Stack:** React + TS, `jsbarcode` + `qrcode` (installed), `modern-screenshot` (installed), `@zxing/library`
(dev, decode tests), Go (Echo, sqlx, goose), Vitest, Playwright.

**Spec:** `docs/specs/2026-10-26-labels-as-pictures-design.md` · mockup `docs/design/mockups/label-styles.html`

## Global Constraints

- `settings.label_print_as` ∈ `classic | image`, default `classic`; `settings.label_styles` JSON `{kind: style}`,
  default `{}`; unknown kind/style → `classic`. Migration `00012_label_pictures.sql`.
- Only the system user writes `label_styles` (`PUT /api/v1/system/appearance`, field `labelStyles`, merged per kind).
- 8 dots per mm. A label picture is exactly `widthMm*8 × heightMm*8`; the server refuses any other size (422 `BAD_IMAGE`).
- 1-bit only (the slip threshold, `toOneBit`); fonts bundled (`paper.ts`), Sinhala / Tamil fallbacks.
- Price styles, keys and order: `classic, pricetag, inverse, qrtag, dual, ticket, minimal, shelf`.
  Dose: `classic, clear, boxed`. Ticket: `classic, job`.
- Sizes every price style must draw: `50x25, 50x30, 40x30, 38x25, 100x50` (+ shop custom). Shelf only at width ≥ 70 mm,
  else draws as Price Tag.
- Codes: 1D module ≥ 2 dots, quiet zone ≥ 10 modules; QR module ≥ 2 dots, quiet zone ≥ 2 modules, ECC `M`, payload =
  the code value exactly. Can't fit → smaller code within minimums → else that label draws Classic.
- Hero rule: the slot whose key is `price` is the hero; no price → the second slot. Inverse band = second slot; `none`
  closes up everywhere.
- UI wording: "First text" / "Second text" (not Top / Bottom). `localStorage` keys `label.slot.top|bottom` unchanged.
- A failed picture never blocks: that job is sent as today's Classic request.
- Core is plugin-inert; files ≤ 500 lines; `make types` after `dto*.go`; no Claude attribution; admin screens at 360 px,
  every palette × light / dark, theme tokens only (labels themselves are paper: black / white).

## Review Focus

1. **A 38×25 label in QR Tag / Dual with a 13-digit EAN** — the codes must keep their minimums or fall back, never
   print unscannable. Pinned in Task 6 (`decodes at 38x25`).
2. **Second text = none in Inverse, first = none in Price Tag** — closes up, no empty black band. Task 5.
3. **A long Sinhala name at 2 lines in Price Tag** — wraps, never overflows the label box (picture height stays exact).
   Task 5 (`never taller than the label`).
4. **Image mode with no label printer configured** — same `NO_PRINTER` error and toast as Classic. Task 2.
5. **Intake auto-print with Image mode** — prints the picture without opening any preview, and a raster failure
   falls back to Classic silently (a toast, not a block). Task 4.

---

### Task 1: Settings — label switch + label styles

**Files:**
- Create: `migrations/00012_label_pictures.sql`
- Modify: `internal/features/settings/settings.go` (struct, Update form/SQL, `SetAppearance`), `internal/api/dto.go`
  (`Appearance.LabelStyles`), `internal/api/dto_till.go` (`TillConfig.LabelPrintAs`, `LabelStyles`),
  `internal/api/system.go`, `internal/api/till_catalog.go`, `internal/api/me.go`, Settings form DTO if separate
- Test: `internal/features/settings/settings_db_test.go` (or the existing settings DB test file), `internal/api/system_test.go`

**Interfaces:**
- Produces: `settings.ValidLabelStyles(raw map[string]string) map[string]string` (drops empty keys/values, keeps
  plugin kinds as-is, trims, max 32 entries); `Settings.LabelPrintAs string`, `Settings.LabelStyles map[string]string`
  (jsonb); TS `TillConfig.labelPrintAs: string`, `TillConfig.labelStyles: Record<string,string>`,
  `TillConfig.labelFirstLines/labelSecondLines: number`, `labelFirstSize/labelSecondSize: string` (from the existing
  `label_name_lines`, `label_bottom_lines`, `label_name_font`, `label_bottom_font`), `Appearance.labelStyles: Record<string,string>`.

- [ ] **Step 1: Failing tests** — `TestLabelSettingsDefault` (fresh DB: `LabelPrintAs=="classic"`, `LabelStyles` empty
  map); `TestSetAppearanceMergesLabelStyles` (PUT `{labelStyles:{price:"qrtag"}}` then `{labelStyles:{"pharmacy.dose":"clear"}}`
  → both kept; `{price:""}` removes price); `TestLabelPrintAsValidated` (`"bitmap"` → 400 VALIDATION).
- [ ] **Step 2:** `make test` → FAIL.
- [ ] **Step 3:** Migration: `label_print_as TEXT NOT NULL DEFAULT 'classic' CHECK (label_print_as IN ('classic','image'))`,
  `label_styles JSONB NOT NULL DEFAULT '{}'`; Down drops both. Settings form field `label_print_as`
  (`validate:"required,oneof=classic image"`). Backup/restore: confirm settings are dumped whole (add columns if the
  backup lists them). Expose on `TillConfig` and Appearance; `make types`.
- [ ] **Step 4:** `make test` → PASS.
- [ ] **Step 5:** Commit `feat(labels): label print-as switch + per-kind label styles setting`.

### Task 2: Server — print a label picture as TSPL BITMAP

**Files:**
- Create: `internal/tspl/image.go`, `internal/tspl/image_test.go`
- Modify: `internal/api/counter_labels.go` (CounterLabels, CounterTextLabel; `/admin/labels/send` reuses CounterLabels)
- Test: `internal/api/counter_labels_test.go` (create if absent; follow the existing api test fixture)

**Interfaces:**
- Produces: `tspl.ImageDocument(img image.Image, widthMM, heightMM, gapMM, count int) ([]byte, error)`;
  `tspl.ErrBadImage`. HTTP: `POST /till/labels?count=N&size=WxH` and `/till/labels/text?count=N&size=WxH` (and
  `/admin/labels/send`) with `Content-Type: image/png` print the picture; JSON body unchanged = Classic.

- [ ] **Step 1: Failing tests** — `TestImageDocumentRoundTrip`: a 400×200 PNG with a known pattern →
  output contains `SIZE 50 mm, 25 mm`, `GAP 2 mm, 0 mm`, `CLS`, `BITMAP 0,0,50,200,0,` + 10 000 bytes, `PRINT 3,1`;
  decoding the bitmap bytes back (bit **0 = ink**, 1 = paper; MSB first) equals the input pixels.
  `TestImageDocumentRefusesWrongSize` (401×200 → `ErrBadImage`). API: `TestLabelImagePrint` posts a 400×200 PNG with
  `?count=2&size=50x25` to a test printer target and gets 200; a 576-wide PNG → 422 `BAD_IMAGE`; no label printer →
  same `NO_PRINTER` as the JSON path.
- [ ] **Step 2:** `go test ./internal/tspl ./internal/api -run 'Image|LabelImage'` → FAIL.
- [ ] **Step 3:** Implement. Decode with `escpos.ReadImage`-style limits (reuse its PNG size cap; height must equal
  `heightMM*8`). Ink rule: luminance < 160 or alpha ≥ 128 as in `escpos.ImageJob`. Width bytes = `ceil(w/8)`, pad bits
  are paper. Handler branch on `Content-Type` like `slipImage`; size from `labelSize(q.size, cfg…)`, count clamped 1–200
  (text: 1–50), gap from settings. Audit the same as the Classic path.
- [ ] **Step 4:** Tests PASS.
- [ ] **Step 5:** Commit `feat(labels): print a label picture as a TSPL bitmap`.

### Task 3: Web — label picture pipeline + Classic price template

**Files:**
- Modify: `web/src/features/print/raster.ts` (`rasterize(node, dots, fonts, height?)` — when given, the canvas is
  exactly that tall and must match)
- Create: `web/src/features/print/labels/kit.ts` (types + `LabelKit`), `labels/codes.tsx` (`Bars`, `Qr` drawn at whole
  dots, `fitCode`), `labels/fit.ts` (text wrap to N lines at a size, reports cut), `labels/price/classic.tsx`,
  `labels/price/index.ts` (`PRICE_STYLES`), `labels/registry.ts` (`labelKinds()`, `labelStyleOf(kind, key)`),
  `labels/picture.ts` (`labelPicture`, `labelPictureQuery`), `labels/LabelView.tsx`, `labels/print.ts` (`printLabel`)
- Test: `web/src/features/print/labels/classic.test.tsx`, `labels/picture.test.ts`

**Interfaces:**
- Consumes: Task 1 `TillConfig.labelPrintAs/labelStyles`; Task 2 endpoints.
- Produces:
  ```ts
  type Slot = { key: string; text: string }            // key: 'name'|'price'|'f:<label>'|'none'|'free'
  type PriceLabel = { first: Slot; second: Slot; code: string; format: string; firstLines: number; secondLines: number;
                      firstSize: string; secondSize: string }   // sizes '1'..'4' as today's fonts (XS..L)
  type LabelPaper = { wMm: number; hMm: number; w: number; h: number }   // w,h in dots
  type LabelTemplate<D> = ComponentType<{ d: D; paper: LabelPaper; onCut?: () => void }>
  type LabelStyle<D> = { key: string; label: string; Template: LabelTemplate<D>; minWidthMm?: number; fallback?: string }
  type LabelKind<D = unknown> = { kind: string; label: string; sample: D; styles: LabelStyle<D>[] }
  labelPicture(kind: string, data: unknown, paper: LabelPaper, style?: string): Promise<{ blob: Blob; url: string; cut: boolean }>
  printLabel(o: { kind: string; data: unknown; size: string; count: number; classic: { path: string; body: unknown } }): Promise<void>
  <LabelView kind data size classic={LabelRequest-or-text-request} />   // picture in Image mode, LabelSvg in Classic
  ```
  `printLabel` in Image mode: picture → `POST classic.path?count&size` as PNG; on raster error → `api(classic.path, {json: classic.body})`
  + `console.warn`. Classic mode: the JSON post as today. Fonts: `PAPER_FONTS`/`SANS_FONTS` from `paper.ts`.
- [ ] **Step 1: Failing tests** — Classic price template at 50×25 renders first text, digits, second text in that
  order; `none` omits a slot; a 3-line name with `firstLines: 1` reports cut; `labelPicture` output (jsdom-mocked
  `domToCanvas`) is requested at exactly 400×200.
- [ ] **Step 2:** `cd web && bun run test labels` → FAIL.
- [ ] **Step 3:** Implement. Classic draws today's geometry (center, margins 16/12 dots, JetBrains Mono at the TSPL
  font sizes) so Image-Classic looks like the Classic print. The root element is `w×h` px, `overflow:hidden`, white.
- [ ] **Step 4:** Tests PASS; `bun run typecheck` clean.
- [ ] **Step 5:** Commit `feat(labels): label pictures — pipeline, Classic template, LabelView, printLabel`.

### Task 4: Wire every price-label caller + First / Second text

**Files:**
- Modify: `web/src/features/counter/labels/LabelsTab.tsx`, `slots.ts`, `LabelPreview.tsx` (keep `LabelSvg`/`useLabelLayout`
  for Classic), `web/src/features/admin/intake/LabelBlock.tsx`, `IntakePage.tsx`, `web/src/features/admin/labels/LabelsPage.tsx`
  (if it prints itself), `internal/api/dto_counter.go` (`LabelRequest.TopKey`, `BottomKey` — json `topKey`, `bottomKey`,
  ignored by Go, carried for the A4 sheet's URL)
- Test: `web/src/features/counter/labels/slots.test.ts`

**Interfaces:**
- Consumes: Task 3 `printLabel`, `LabelView`, `PriceLabel`.
- Produces: `toPriceLabel(req: LabelRequest, cfg: TillConfig): PriceLabel` in `labels/price/index.ts` (lines /
  sizes from Task 1's `TillConfig.labelFirst*` / `labelSecond*`; free-text custom
  labels use key `free`).
- [ ] **Step 1: Failing test** — `toPriceLabel` maps `topKey:'price'` → `first.key === 'price'`, custom label →
  `free`, `none` → empty text.
- [ ] **Step 2:** FAIL → **Step 3:** Rename visible "Top line / Bottom line" to "First text / Second text" (aria-labels
  too; update e2e selectors that use them). Counter, admin Labels, Intake print through `printLabel` (Intake uses
  `/admin/labels/send` as `classic.path`); previews through `LabelView`.
- [ ] **Step 4:** `bun run test` + `bun run typecheck` PASS; the existing label e2e specs pass (Classic default).
- [ ] **Step 5:** Commit `feat(labels): every price label prints through printLabel; First / Second text`.

### Task 5: Price styles — Price Tag, Inverse, Ticket, Minimal

**Files:**
- Create: `web/src/features/print/labels/price/{pricetag,inverse,ticket,minimal}.tsx`, `price/hero.ts`
- Modify: `price/index.ts`
- Test: `web/src/features/print/labels/price/styles.test.tsx`

**Interfaces:**
- Produces: `hero(d: PriceLabel): { hero: Slot; other: Slot; heroIsFirst: boolean }` (price wins, else second).
- [ ] **Step 1: Failing tests** (each style × sizes `50x25, 38x25, 100x50`): renders both chosen texts; the hero
  element (`data-hero`) holds the price for `price+name` *and* `name+price`; `name+f:Model` → model is hero; Inverse with
  second `none` has no `[data-band]`; Price Tag with first `none` renders only the hero; root never taller than
  `paper.h` (scrollHeight ≤ h) with a 60-char Sinhala name at 2 lines.
- [ ] **Step 2:** FAIL → **Step 3:** Implement per the mockup sections 1 and 1b. Size setting scales: `'1'..'4'` →
  ×0.75 / ×0.9 / ×1 / ×1.2 of the style's base size (Classic keeps the TSPL sizes).
- [ ] **Step 4:** PASS. **Step 5:** Commit `feat(labels): Price Tag, Inverse, Ticket, Minimal styles`.

### Task 6: Price styles — QR Tag, Dual, Shelf + scan decode

**Files:**
- Create: `web/src/features/print/labels/price/{qrtag,dual,shelf}.tsx`
- Modify: `price/index.ts`, `labels/codes.tsx`
- Test: `web/src/features/print/labels/price/decode.test.ts` (zxing on the rendered + 1-bit canvas; follow the
  receipt barcode decode test's setup), `styles.test.tsx`

- [ ] **Step 1: Failing tests** — `decodes at 50x25` and `decodes at 38x25`: for `qrtag` (QR) and `dual` (QR and EAN-13
  `4792024012345`) the decoded values equal the code; when the minimum can't fit, the label falls back to Classic
  (`labelStyleOf` reports `fallback`), never a sub-minimum code. Shelf at 50×25 renders Price Tag; at 100×50 shows the
  printed date `dd Mon yyyy` and both codes.
- [ ] **Step 2:** FAIL → **Step 3:** Implement (QR via `qrcode` `create()` modules drawn as whole-dot rects).
- [ ] **Step 4:** PASS. **Step 5:** Commit `feat(labels): QR Tag, Dual and Shelf styles; codes decode at 38×25`.

### Task 7: Plugin label kinds — slot + Pharmacy dose + Repairs ticket

**Files:**
- Modify: `web/src/plugins/slots.ts` (`labelTemplates: [] as LabelKind[]`), `labels/registry.ts` (core `price` +
  `slots.labelTemplates`)
- Create: `web/src/plugins/pharmacy/labels/{classic,clear,boxed}.tsx`, `web/src/plugins/repairs/labels/{classic,job}.tsx`
- Modify: `web/src/plugins/pharmacy/{index.tsx,LabelPreview.tsx,api.ts,label.ts}` (structured `DoseLabel` data; Classic
  = today's `doseLabel()` lines), `web/src/plugins/repairs/{index.tsx,api.ts,JobView.tsx,LabelAsk.tsx}`
- Test: `web/src/plugins/pharmacy/labels.test.tsx`, `web/src/plugins/repairs/labels.test.tsx`

**Interfaces:**
- Consumes: Task 3 `LabelKind`, `printLabel`, `LabelView`.
- Produces: kinds `pharmacy.dose` (data = `DoseLabel` from `label.ts`), `repairs.ticket` (data = `{ ticketNo, code,
  device, customer, phone, received, readyBy? }`). Classic paths: `/till/labels/text` (dose, body `{lines,count,size}`),
  `/till/labels` (ticket, today's `ticketLabel()`).
- [ ] **Step 1: Failing tests** — Clear renders patient, drug, qty, dose and note in sentence case (no all-caps text
  node); Boxed with `1-0-1` shows Morning 1 / Noon – / Night 1, with `sos` draws as Clear; Job shows `R-142`-style
  number, device, customer and a QR decoding to the ticket code; the "doesn't fit" warning still shows when a template
  cuts.
- [ ] **Step 2:** FAIL → **Step 3:** Implement; `make check-core` still passes (core names no plugin).
- [ ] **Step 4:** PASS. **Step 5:** Commit `feat(labels): plugin label kinds; Pharmacy dose (Clear, Boxed), Repairs ticket (Job)`.

### Task 8: Appearance → Labels

**Files:**
- Create: `web/src/features/appearance/LabelStyles.tsx`
- Modify: `web/src/routes/_authed/admin/system.appearance.tsx`
- Test: `web/e2e/label-styles.spec.ts`

- [ ] **Step 1: Failing e2e** — system user: Appearance shows a "Labels" section with a tab per kind (Price label
  always; plugin tabs when built in — Playwright builds all plugins); pick Price Tag → big preview changes → Save →
  the counter Labels tab preview (Image mode on) shows Price Tag; with labels on Classic a note "This shop prints labels
  as Classic" shows.
- [ ] **Step 2:** FAIL → **Step 3:** Implement like `ReceiptStyles.tsx`: thumbnails are `labelPicture` of the kind's
  sample in each style at the shop size; size switch over `LABEL_SIZES`; "too small for this style" on fallback;
  Save = `PUT /system/appearance` with `labelStyles: {[kind]: style}`, then invalidate `configQuery`. 360 px: scroller.
- [ ] **Step 4:** PASS. **Step 5:** Commit `feat(appearance): label styles per label kind`.

### Task 9: Settings → Printers & labels — switch + live preview; A4 sheet

**Files:**
- Modify: `web/src/features/settings/SettingsPage.tsx` (switch + preview; extract the label block to
  `web/src/features/settings/LabelSettings.tsx` to stay ≤ 500 lines), `web/src/routes/_authed/print/labels.tsx`
- Test: `web/e2e/label-settings.spec.ts`

- [ ] **Step 1: Failing e2e** — Settings: "Print labels as" Image / Classic; the label preview changes live when
  lines / size / First-Second / label size change (before Save); Classic shows the SVG geometry, Image the picture.
  The A4 sheet in Image mode renders the template (`[data-label-style]` present), in Classic `LabelSvg`.
- [ ] **Step 2:** FAIL → **Step 3:** Implement. The sheet renders the template element at `mm` size via CSS
  `width: Wmm; height: Hmm` with an inner `transform: scale()` from dots (vector, not the PNG).
- [ ] **Step 4:** PASS. **Step 5:** Commit `feat(settings): label print switch + live label preview; A4 sheet follows the style`.

### Task 10: End-to-end print check, shot review, docs

**Files:**
- Modify: `web/e2e/print-image.spec.ts` (or new `label-print.spec.ts`), `docs/status.md`, `AGENTS.md` (migration list
  → next `00013_`; `slots.labelTemplates` in the slots list; `features/print/labels`), `docs/design/theme.md` (label
  styles section), `docs/acceptance.md` (real-printer label check)

- [ ] **Step 1:** e2e — Image mode: counter Labels → Print 3 → the captured job (test print target) starts with
  `SIZE 50 mm, 25 mm`, has a `BITMAP` and `PRINT 3,1`; Classic: the job has `TEXT`/`BARCODE` as today; Pharmacy dose
  and Repairs ticket each print once in Image mode.
- [ ] **Step 2:** Full runs: `make test`, `make test-web`, `make check-core`, Playwright (kill 3100/5173 first).
- [ ] **Step 3:** Shot review → `web/shots/lbl/`: every price style at 50×25, 38×25, 100×50 (+ a Sinhala name); dose
  and ticket styles; Appearance Labels + Settings preview in Studio and Night × light / dark, desktop + 360 px. Look at
  each: nothing clipped at card edges (memory: shot review edges).
- [ ] **Step 4:** Docs; commit `docs: labels as pictures — status, AGENTS, theme, acceptance`.
