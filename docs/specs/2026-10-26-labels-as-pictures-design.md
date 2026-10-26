# Labels as pictures + label styles — design

Date: 2026-10-26. Status: draft for the user's review.
Follows `docs/specs/2026-10-21-print-as-image-design.md` and `docs/specs/2026-10-21-receipt-styles-design.md`
(built: every receipt slip is a React template drawn at printer dots, printed as a picture, previewed as that
picture, in one of ten styles). This does the same for labels.

Mockup (brainstorm, 2026-10-26): `docs/design/mockups/label-styles.html` — the price-label styles, the
First / Second text rule (section 1b), Shelf, the Pharmacy and Repairs kinds, the Appearance section and the
Settings preview. It's a brainstorm fragment, so open it through the visual companion or read its inline styles. The
barcodes and QR codes in it are fake placeholders.

## Why (user)

- The same reasoning as receipts: Image mode lets labels be customised, and **Classic stays as one of the styles**
  inside Image mode. Today's labels don't look bad. The point is **variety, so the suite sells to more kinds of
  shops** (user: "some variations needed if I want to sell more places").
- Some shops have started using QR codes. The user's recommendation to shops is a 2D scanner. A shop without one
  just picks a barcode style.
- Plugins print labels too (Pharmacy dose label, Repairs ticket label). Those kinds should get their own looks
  when the plugin is built in.

## Goals

1. **Labels print as pictures in Image mode.** Every label kind becomes a React template drawn at the label
   printer's dots (8 dots per mm, 203 dpi), turned into a 1-bit picture in the browser and sent to the printer as a
   TSPL `BITMAP`. The preview is that same picture.
2. **A switch "Print labels as: Image (new) / Classic"** in Settings → Printers & labels. **Classic is the default
   and stays** until a real label printer has been tested (user, 2026-10-26: no real printer yet). It's separate
   from the receipt switch (my call: the label printer is a different machine and is proven on its own).
3. **Label styles, picked in Appearance** (system user only, like receipt styles), **per label kind**:
   - **Price label** (core, the barcode label): **Classic, Price Tag, Inverse, QR Tag, Dual, Ticket, Minimal,
     Shelf**, as in the mockup.
   - **Dose label** (Pharmacy plugin): **Classic, Clear, Boxed**.
   - **Repair ticket** (Repairs plugin): **Classic, Job**.
   - Each kind keeps its own style. Label styles are separate from the receipt style (my call: a dose label and a
     receipt shouldn't be forced to match).
4. **First text / Second text stay swappable** (user: "top and bottom row interchangeable"). Every price-label
   style has the same two text slots. The shop still picks what goes in each: Name, Price, any "print on label"
   field, or none. A style only decides **where and how** each slot is drawn. "Top / bottom" never means a literal
   row (user: "whichever text is in top is the top and the other one is bottom"), so the choosers are renamed
   **First text** and **Second text** everywhere (my call).
5. **A live label preview in Settings → Printers & labels**, next to the label size, lines and size controls. It
   follows the switch: the Image picture in the shop's Appearance style, or the Classic geometry (today's
   `LabelSvg`).
6. **Sinhala / Tamil names print on labels** in Image mode. Classic prints `?` for them today.

## Non-goals

- Changing Classic (TSPL text) labels. Classic keeps its one look and its current code, and it's deleted only once
  Image is proven (the user's call, later).
- Electronic shelf labels, GS1 Digital Link URLs, batch / expiry inside the QR. The QR holds the same value as the
  barcode, nothing more.
- Per-shop editing of a style (moving parts around). A style is chosen whole, as with receipts.
- A label designer.

## Anti-goals (failure even if it "works")

- The preview and the paper differ.
- A barcode or QR that doesn't scan off a real 203 dpi label, or a QR that scans to something other than the
  product's code. Scanning a style's QR at Sell must find the product exactly as its barcode does.
- A style that ignores the shop's First / Second text choice, or hides the price when the shop chose to print it.
- Anything grey. Thermal labels are black or white (same 1-bit threshold as slips).
- Core naming a plugin. A plugin label kind that breaks when a core style is added.
- A broken print stopping work at the counter (memory "never stop a sale"). If the picture can't be made, that
  one job prints Classic.

## Decided

**Settings (migration `00012_`).**
- `settings.label_print_as` is `classic | image`, default `classic`. Admin sets it in Settings → Printers &
  labels. It's part of backup / restore like `print_as`.
- `settings.label_styles` is a JSON object `{ "<kind>": "<style>" }`, for example `{"price": "pricetag",
  "pharmacy.dose": "clear"}`. A missing or unknown kind or style reads as `classic`. Only the system-user
  Appearance endpoint writes it (as with `receipt_style`). It's stored as an open map so plugin kinds need no core
  column and core never names them. Old backups restore with `{}`, which means all Classic.

**The existing label controls stay, for both modes** (user agreed 2026-10-26). These are the label size and gap,
the lines (1–4) and the size (XS–L) for each text. In Classic they keep their current meaning. In Image mode,
**lines** is how many lines that text may wrap to before it's cut, and **size** scales that text relative to the
style's own size (M = the style's size; XS / S smaller, L bigger). The style decides the base sizes; the setting
nudges them. When Classic is deleted later, these become Image-only controls and nothing is lost.

**First / Second text.**
- The label request carries **which field** each slot holds, not just its text: `first` / `second` = `{ key,
  text }` where `key` is `name | price | f:<label> | none | free`. `free` is used by plugins and other callers
  with no field. This lets a style draw a price as a price.
- **The hero rule** (in the styles with one big slot: Price Tag, QR Tag, Shelf): the slot holding **Price** is the
  hero, whether it's first or second. If neither slot is the price, the **second** slot is the hero (mockup 1b:
  a phone shop's model number).
- **Inverse:** the black band holds the second slot. If the second slot is `none`, the band goes and the barcode
  grows.
- **`none` closes up** in every style; the other parts take the space.
- The per-device memory of the choice stays (`label.slot.top|bottom` in `localStorage`, shared by the counter,
  admin Labels and Intake). Only the visible names change.

**The styles (price label).** Each must work at every offered size (`LABEL_SIZES`: 50×25, 50×30, 40×30, 38×25,
100×50) and at the shop's custom size.
- **Classic:** today's layout, drawn as a picture (first text, barcode + digits, second text, centred).
- **Price Tag:** a huge hero, the other text small, a short barcode with its digits beside it.
- **Inverse:** first text, barcode + digits, the second slot in a full-width black band in white.
- **QR Tag:** a QR on the left, the texts stacked on the right with a rule, digits under them. No 1D barcode.
- **Dual:** a 1D barcode + digits and a QR side by side, texts above.
- **Ticket:** a dashed border with notches, barcode, digits and price in one row.
- **Minimal:** left-aligned, one thin rule, a smaller barcode, lots of white.
- **Shelf:** for wide labels (my call: ≥ 70 mm wide): a big hero, the other text as a heading, barcode + QR, and the
  **date printed** (so old tags are easy to spot). Below 70 mm it draws as Price Tag. The mockup's "Rs 312.50 /
  100 g" price per unit is dropped (my call): products don't carry a net weight. It can come back if they ever
  do.

**Barcodes and QR.** The 1D barcode uses the request's format (CODE128 / EAN13 / EAN8 / UPC / CODE39) through the
already-installed `jsbarcode`. The QR uses the already-installed `qrcode`, holds exactly the code value, and uses
error correction M. Bars and QR modules land on whole dots (module ≥ 2 dots, as in TSPL `narrow = 2` today). Quiet
zones are ≥ 10 modules for 1D and ≥ 2 modules for QR (my call, tighter than the QR spec's 4, because 25 mm labels
are tight; the scan test decides). If a label is too small for a style's code to keep those minimums, the style
draws a smaller code within the minimums rather than an unscannable one. If it still can't fit, it falls back to
Classic for that size, and Appearance shows "too small for this style".

**Plugin label kinds.** This is a new slot `slots.labelTemplates` (mirrors `slipTemplates`):
`{ kind, label, sample, styles: { key, label, Template }[] }`. `Template` gets the plugin's own label data and
the kit (paper size in dots, fonts, barcode / QR drawing, text fitting). `styles` must include `classic`. The plugin
calls a core print helper with `(kind, data, count, size)`. Core looks up the kind's saved style, renders the
template, rasterises it and uploads the picture (Image mode). In Classic mode the plugin's existing request is
sent as today. Core only ever sees kind strings. An unknown kind or a missing template prints Classic.
- **Pharmacy** registers `pharmacy.dose` with structured data (patient, drug, qty, dose, words, perDay, note, date),
  the data `doseLabel()` builds its lines from today. Its **Classic** draws today's lines as a picture. **Clear**
  (sentence case, the dose big, patient and medicine together, a rule; pharmacy-label guidance) and **Boxed**
  (Morning / Noon / Night boxes with the count in each; "when needed" doses with no fixed times draw as Clear)
  follow the mockup. The "doesn't fit: shorten the words" warning stays: a template reports when it had to cut.
- **Repairs** registers `repairs.ticket` with the job (ticket no + code, device, customer, phone, received, ready-by
  if set). **Classic** is today's ticket label. **Job** follows the mockup: a big R- number, device, customer and
  dates, and a QR of the ticket code, which the Repairs plugin's existing claim already opens when scanned at Sell.
- The core **text label** (`/till/labels/text`, used only by Pharmacy today) keeps working. In Image mode a text
  label with no template draws its lines in the Classic look.

**Printing (Image mode).** Both `/till/labels` and `/till/labels/text` also accept the picture (`Content-Type
image/png`, with `count` and `size` as query parameters), the same pattern as slips (`slipImage`). The server
checks the PNG is exactly the label's width and height in dots (8 per mm) and refuses otherwise. It builds TSPL
`SIZE` / `GAP` / `DIRECTION` / `CLS` / `BITMAP` / `PRINT count`, and sends it through the same label-printer lookup
and errors as today (`NO_PRINTER`, `PRINT_FAILED`). Copies are one picture with `PRINT n`, not n uploads. If the
picture can't be made in the browser, that job is sent as today's Classic request.

**Previews.** Every label preview (counter Labels tab, admin Labels page, Intake, Repairs' LabelAsk, Pharmacy's
dose preview, Settings) shows the picture in Image mode and today's `LabelSvg` in Classic. Print sends the
previewed picture; nothing re-renders between preview and print.

**A4 label sheet** (`/print/labels`). In Image mode it draws the same React template at real size as vector (not
the 203 dpi picture), because an office printer prints it crisply. In Classic it stays `LabelSvg`.

**Appearance → Labels** (system user; mockup section 5). It has a tab per label kind: Price label always, and
plugin kinds only when built in. Each tab has live thumbnails of the kind's styles drawn from a sample (core: a
made-up product with name, price and an EAN-13; plugins: their `sample`) at the shop's label size, a big preview
that is the exact picture, a size switch over `LABEL_SIZES`, and Save. A yellow note shows when the shop prints
labels as Classic (as for receipts). It must work at 360 px: the thumbnails become a scroller and the preview fits.

**Settings → Printers & labels preview** (mockup section 6). It sits beside the label controls and uses the same
made-up product. It has **First text / Second text** choosers for the sample (Name / Price / none, saved as this
device's choice, the same `localStorage` keys). It changes live with the size, gap, lines and size controls and
the Image / Classic switch, before Save.

## Constraints

- Files ≤ 500 lines, organised by concern. Core is plugin-inert. Theme tokens on every screen. Labels are paper
  (black and white only, the paper / ink exemption).
- Fonts are bundled only and never come from the PC, with Sinhala / Tamil fallbacks, as for slips.
- Reuse the slip pipeline (`rasterize`, `toOneBit`, the picture query pattern) rather than a second one.
- Rasterising a 50×25 label must be fast at the counter: Intake auto-prints after each item.
- Kiosk Chromium and Firefox both work (memory: check prints in Firefox).

## Checks the work must leave behind

- Go: the image label job decoded back gives the input bitmap pixel for pixel, with the right `SIZE` / `GAP` /
  `PRINT n`, and a wrong-size PNG is refused.
- Vitest: each price style × First / Second combinations (name+price, price+name, field+none, none+price) renders
  both chosen texts (or closes up for `none`) and follows the hero rule. The dose and ticket styles render every
  field they're given.
- A decode test: render a label, rasterise to 1-bit, decode the 1D barcode and the QR, and get the code back (at
  50×25 and 38×25).
- e2e: switch labels to Image → print from the counter Labels tab → the server receives a picture of the right
  dots. Appearance switches the price style → the counter preview changes. Classic mode still sends today's TSPL.
- Shot review (UI gate): every price style at 50×25, 38×25 and 100×50, including a Sinhala name; the dose and
  ticket styles; Appearance → Labels and the Settings preview in two palettes × light / dark, desktop and 360 px.

## Left to the builder

- The exact look of each style beyond the mockup (spacing, type sizes) and how each adapts to the five sizes.
- The kit interface for label templates (it may share pieces with the slip kits), and file layout under
  `web/src/features/print/labels/` or similar.
- The sample product's data, and how Appearance gets plugin samples.
- The build order. My suggestion: the switch + the picture pipeline + Classic-as-picture first, then the First /
  Second change, then the styles one or two per commit, then the plugin slot with Pharmacy and Repairs, then
  Appearance, then the Settings preview.
