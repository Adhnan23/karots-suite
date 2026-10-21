# Receipt styles (Image mode) — design

Date: 2026-10-21. Status: draft for the user's review.
Follows `docs/specs/2026-10-21-print-as-image-design.md` (built: every slip is a React template drawn at printer
dots, printed as a picture, and every preview is that picture).

Mockups (approved in the brainstorm, 2026-10-21):
- `docs/design/mockups/receipt-styles.html` — the ten styles on one busy sale.
- `docs/design/mockups/appearance-receipts.html` — the Appearance page's receipt section (layout only; the
  thumbnails there are rough sketches, the real ones are live renders — user: "in implementation you did well").
Both are brainstorm fragments: open them through the visual companion, or read their inline styles.

## Why

The printed receipt is what makes a shop look like itself — the customer takes it home (user: "the main thing is
the receipts"). Today every shop prints the same slip: `internal/escpos` came over from karots-pos almost
unchanged, and its four "receipt styles" (classic / modern / boxed / compact on the Appearance page) only swap
separators and the header (checked 2026-10-21: items, totals and footer are identical in all four). In Image mode
they do nothing at all. Image mode removed the limit — a receipt can now look like anything we can draw — so the
suite gets real receipt designs, chosen per shop.

This is part 1 of a larger Appearance rework the user asked for (part 2: palettes — hand-written CSS files, a
palette editor with glass / liquid-glass surfaces, custom palettes saved in the shop's DB, export / import; later:
theme packs with other layouts). This document covers only receipts and the bits of the Appearance page they touch.

## Goals

1. **Ten receipt styles for Image mode**, switchable: **Classic** (today's Image templates, unchanged, the
   default) plus **Bold, Minimal, Ticket, Poster, Grid, Stamp, Elegant, Cards, Sections** as drawn in the mockup.
   They don't follow one pattern (user: "there is no standards this needs to look this way").
2. **Every slip follows the style**, not only sales: sale, refund, cash in / out (CR-), credit payment (DP-),
   warranty replacement, the printer test, and plugin slips (Repairs' job slip today). Each at 80 mm and 58 mm.
3. **Every line a slip can have, in every style** (user: "remember the discount lines, warranty lines and other
   stuff"): item discount (with its % label), line total, bill saving, tax, packs ("1 Card + 2"), line groups (deal
   + members), payouts ("Given", a sale below zero ending in GIVEN), warranty + serial lines, the credit block
   (From advance / DUE / TOTAL DUE / ADVANCE LEFT), rounding (On account / Change kept), every tender, change, the
   logo, the shop's local-language name, address / phone / VAT, the shop footer, the Karots credit line. The source
   of truth for what a slip contains is today's `?data=1` (`internal/api/dto_slip.go`) — a style decides how a
   line looks, never whether it exists.
4. **The receipt number as a barcode** on the styles that show one (Bold, Minimal, Ticket, Poster, Grid, Stamp,
   Elegant, Cards, Sections — all but Classic): a real, scannable **Code 128** of the slip's number (S-…, CR-…,
   DP-…, R-… for repairs). Minimal also prints a QR of the same number.
5. **Scan a slip to find it.** At the till's Receipts tab, Returns, Warranty (by receipt) and the admin Sales
   page, scanning the barcode (a keyboard-wedge scanner types the number + Enter into the find box) opens exactly
   that slip.
6. **Appearance → Receipt style** (system user only), per the mockup: ten live thumbnails of *this shop's* receipt
   in each style, a big preview that is the exact print picture, switches for slip type / 80 · 58 mm / sample
   (busy sale · simple · on credit), Save. A note when the shop still prints Classic.
7. **Plugins get a seam** for their slips (user: "plugins need to make a hook or seam"); Repairs' job slip is
   rebuilt on it.

## Non-goals

- Palettes, the palette editor, glass surfaces, theme packs — part 2 and later.
- Labels (TSPL) — unchanged.
- Customising a style per shop (move the logo, hide lines, own footer art) — a style is chosen whole. The shop's
  own footer text, logo and names already flow in.
- New Classic (ESC/POS) designs. Classic printing keeps one look; it is deleted once Image mode is proven
  (user: "for a few weeks; if image mode is proven better we purge those legacy").

## Anti-goals (failure even if it "works")

- A style that drops or hides a line the sale has (a discount, a warranty, a DUE). Money lines must be all there.
- The preview and the paper differ (the print-as-image guarantee holds for every style).
- A barcode that doesn't scan, or a scan that opens the wrong sale (see "Scan to find").
- Anything grey: thermal paper is black or white (1-bit threshold at 160). Shading must be solid black, white, or
  a pattern of black dots that survives the threshold.
- Core naming a plugin; a plugin slip that breaks when a new style is added.

## Decided

- **Who picks:** only the system user, on Appearance, shop-wide (like the palette). Admins don't see it.
- **The Image / Classic switch stays in Settings → Printers** for now (user).
- **Classic printing is classic only.** modern / boxed / compact are deleted from `internal/escpos/style.go`
  (`StyleFor`, `SamplePreview`) and the Appearance page; Classic printing always uses today's classic look and no
  longer reads `receipt_style` (which now names an Image style).
- **Density goes now** (user agreed: it only shrinks ~20 input boxes; `--gap` / `--pad` are unused): the control,
  the setting's use and the tokens. The **brand colour and radius** controls stay until part 2's palette editor
  replaces them.
- **The A4 / PDF receipt page** (`/print/receipt/:id`) draws the sale in the shop's chosen style too.

## The how — constraints for the builder

**Setting.** `settings.receipt_style` is reused for the Image style: values `classic | bold | minimal | ticket |
poster | grid | stamp | elegant | cards | sections`, default `classic`. Old values (`modern`, `boxed`, `compact`,
from old backups or shops) read as `classic` (`ValidReceiptStyle`). Written only by the system-user Appearance
endpoint, as today; returned to the SPA where the print path needs it (`TillConfig` / `?data=1` — builder's
choice, but the style that prints is the shop's saved one, never a client override, except the Appearance preview
which renders an unsaved choice for preview only and never prints).

**Templates.** `web/src/features/print/` today has one kit (`kit.tsx`) and one template per slip kind. It becomes:
- a **style kit** per style: the building blocks a slip is made of — page (margins), header (logo, names,
  address), rules, item row, sub-line, money row, total band, section heading, barcode, footer. Each style
  implements the same kit interface its own way;
- the slip templates (sale, refund, money, debt, warranty, test) written **once against the kit interface**, so a
  slip in Ticket style is the shared sale template drawn with Ticket's kit;
- a style may **override a whole slip** where the shared shape can't express it (Poster's headline total, Grid's
  table, Sections' grouped blocks): `styles/<style>/sale.tsx` wins over the shared one for that style.
- Classic's kit is today's `kit.tsx` and must produce exactly today's pictures (pixel-identical — the
  print-image e2e and a shot comparison guard it).
- Files ≤ 500 lines; one folder per style.

**Fonts.** Bundled only (`@fontsource`), never the PC's. Styles may use a sans (e.g. Inter) and a serif (e.g.
Noto Serif / Source Serif) besides JetBrains Mono; each must fall back to Noto Sans / Serif Sinhala and Tamil for
local names. `rasterize` waits for every font the active style uses (not just JetBrains Mono).

**Widths.** 80 mm = 576 dots, 58 mm = 384. The one-column side margin (user, 2026-10-21) applies to every style.
Each style must hold at 58 mm: nothing clipped at the right, long names wrap, numbers never wrap.

**Barcode / QR.** Generated in the browser into the slip picture (it's drawn like any other ink), Code 128 (set B
covers S-/CR-/DP-/R- numbers), bars at whole-dot widths (module ≥ 2 dots at 80 mm, ≥ 2 at 58 mm) and a quiet zone
of ≥ 10 modules, height ≥ 48 dots, so a normal till scanner reads it off thermal paper. A small dependency is fine
(e.g. `jsbarcode`, `qrcode`) if bundled; or a short encoder. The test is a decode of the rendered 1-bit picture.

**Scan to find.** Fix the two gaps found while designing:
- the find box matches part of the number, so `S-00231` also matches `S-002310`, and Enter takes the newest hit —
  an **exact receipt-number match must win** (server: rank `receipt_no = q` first, or a dedicated exact lookup on
  Enter);
- the date range ("latest", "today") can hide an old sale — a scanned or typed **exact number ignores the range**.
Same for CR- (cash list), DP- (credit list) on the Receipts tab: scanning a CR- number while on "Sales" switches to
the right list (or the find is kind-agnostic). Where a scan lands: the till's Receipts tab, Returns, Warranty "by
receipt", admin Sales. *(My call: the Sell screen is untouched — a receipt scanned there still searches products;
add "scan a receipt at Sell opens it" only if the user asks.)*

**Appearance section** (mockup): thumbnails are real renders of the shop's sample slip in each style (the same
templates, scaled down — no hand-drawn previews); the big preview is `slipPicture`-equivalent output for the
chosen style, slip kind, width and sample; samples come from the server (the existing `sample-slip` endpoint grows
`?kind=` and `?sample=busy|simple|credit`), never printing. Plugins add their slip kind to the switcher through
the seam. Must work at 360 px (thumbnails become a scroller, the preview fits).

**Plugin seam.** Extend `slots.slipTemplates`: `{ kind, label, sample?: path, Template }` where `Template` gets
the active style's **kit** (and the style key) plus the plugin's `?data=1` JSON — so one plugin template follows
all ten styles. A plugin may add per-style overrides. A plugin slip with no template still prints its decoded
Classic lines. The Appearance switcher lists core kinds + registered plugin kinds; core never names a plugin.
`plugins/repairs` registers `repairs.job` with a sample (a made-up open job) and rebuilds `JobSlip` on the kit.

## Checks the work must leave behind

- Vitest: each style renders every sale-line kind (discount, pack, group, payout, warranty + serial, credit, rounding)
  — at minimum, the line's text is present in the rendered DOM for all ten styles.
- A barcode test: render a slip, rasterise to 1-bit, decode the barcode, get the receipt number back.
- e2e: Appearance (system user) switches style → the preview changes → Save → the next sale prints in that style
  (upload picture differs from Classic's); a scan (type number + Enter) on Receipts opens the exact sale even when a
  longer number exists and the sale is outside the date range.
- Shot review (UI gate, shown to the user): every style × sale (busy) at 80 + 58 mm, plus one non-sale slip per
  style; the Appearance page in both palettes × light / dark, desktop and 360 px.

## Left to the builder

The exact look of each style beyond the mockup (spacing, type sizes, how each draws the credit block and payouts
— the mockup shows a cash + card sale); the kit interface's exact shape; barcode library vs encoder; how samples
are built server-side; the order styles are built in (Classic-equivalence and the kit first, then styles one or two
per commit).
