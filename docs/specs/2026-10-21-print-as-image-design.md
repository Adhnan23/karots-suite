# Print as image — design (2026-10-21)

Idea note: `docs/ideas/print-as-image.md`. Agreed with the user in chat on 2026-10-21. Core feature, not a plugin.

## Why (user)

- In an earlier session a small receipt change **looked fine in the web view but printed totally differently on the
  emulator, with invalid printing**. The user doesn't want to deal with that again. Today the screen and the paper
  are drawn by two separate codes: `web/src/features/counter/receipts/Paper.tsx` is a separate drawing that only
  looks like a receipt, and Go builds the ESC/POS text in `internal/escpos`.
- The printed receipt still looks exactly like karots-pos, because `internal/escpos` is karots-pos's code almost
  unchanged.
- Editing HTML and React is far easier than editing ESC/POS text layouts. Once printing is just "a picture of a
  React page", deep customisation becomes easy **later**.

## Goal of this step

**What you see is what prints, pixel for pixel, for receipts and slips.** A slip becomes a React page drawn at the
printer's dot width. The browser turns it into a 1-bit picture, and the server sends that picture to the printer as
ESC/POS `GS v 0`. The preview shown to the cashier is that same picture.

## Non-goals (this step)

- **No redesign** (user: "we don't want redesign"). The templates copy today's **printed** layout (what
  `internal/escpos/documents.go` and `till_print.go` produce, as seen on the emulator), not the `Paper.tsx` look.
  New looks, logo placement and customisation come later, on top of this.
- **Labels (TSPL) come after** (user: "receipts first, labels after"). Label printing is unchanged in this step.
- The old receipt styles (classic / modern / boxed / compact on the system user's Appearance page) aren't
  reworked. They keep working in Classic mode only. Image mode ignores them (my call: they get rethought in the
  customisation step).

## Anti-goals (failure even if it "works")

- The preview and the paper differ in any way. Image mode exists to make that impossible.
- The output depends on fonts installed on the PC (Sinhala / Tamil missing on one till, a different fallback font
  on another).
- A print breaks the counter. A failed print never blocks or undoes the sale (memory "never stop a sale").
- Classic printing gets worse, or disappears before image printing has proven itself on real shop printers.

## The switch

- Shop-wide setting **Settings → Printing → "Print receipts as: Image (new) / Classic"**, admin-editable.
  **Classic is the default** (existing shops and old backups restore as Classic). New column on `settings`
  (migration `00010_`), included in backup / restore like the other settings. My call: shop-wide rather than per
  printer, which is simpler. Move it per printer later if a shop has mixed printers.
- When Classic has been proven in shops (the user's call, later), the Classic path is deleted in its own step.

## How it works (Image mode)

1. **Templates.** One React template per slip kind, under `web/src/features/print/` (my call for the location):
   sale receipt, refund / return slip, money slip (CR-: till open / close / withdraw / deposit, expenses, supplier
   pay), credit payment (debt) slip, warranty slip, the Settings test print. Each renders at **exact printer
   dots**: 80 mm = 576 px wide, 58 mm = 384 px (`receiptimg.PrinterDots`), honouring the `?size=58|80` override
   reprints already use. 1 CSS px = 1 printer dot. Black on white only, no greys, no palette tokens. It's paper,
   not themed UI (the existing paper / ink exemption).
2. **Same data as Classic.** Each slip's print path (`slipPath.*` in `web/src/features/till/print.ts`, plus the
   plugin paths) answers a new `?data=1` with JSON holding everything the Classic builder uses: the slip's record,
   the shop header (name, sub-name, address, phone, footer), the logo, serials, customer due, width. The template
   gets exactly what the paper needs, and nothing has to be re-derived in TS. DTOs go in `dto.go` + `make types`
   (AGENTS API conventions).
3. **Picture.** The browser renders the template off screen, rasterises it (DOM → canvas with a maintained
   library; my call: `modern-screenshot`, chosen over `html-to-image` for Firefox support. The builder may swap it
   if it fails in Firefox), then **thresholds** to 1-bit. Threshold, not dither: text must stay crisp. Dithering
   can be added later for photos or logos. It uploads a 1-bit PNG.
4. **Fonts are bundled.** The templates' fonts are web fonts served by the binary (the Noto Sinhala / Tamil files
   already in `internal/receiptimg/fonts`, plus one Latin font). Rasterising waits for `document.fonts.ready`. No
   system font is ever used on the paper.
5. **Print.** A new core endpoint (e.g. `POST /api/v1/till/print-image`, body: the PNG + the kick flag) decodes the PNG,
   checks it is exactly the configured dot width (refuse otherwise) and not absurdly tall (my call: cap at about
   4 m of paper, i.e. 32,000 rows), and builds: [drawer kick, see 6] → init → the image as `GS v 0` **in bands** (my call: 256 rows per
   command, because some printers choke on one huge raster) → feed → cut. It sends that through the **same
   transport and printer lookup as today** (`receiptQueue`: the user's own counter printer, else the shop's;
   `printing.Raw`), with the same `NO_PRINTER` / `PRINT_FAILED` errors. Reuse `receiptimg.rasterFromInk`.
6. **The drawer kick works exactly like karots-pos and Classic today** (user, 2026-10-21: "do like karots pos").
   karots-pos `6665a7f` merged kick + receipt into **one job** on purpose: two USB jobs (kick, then receipt) each
   pay the device open/close + inter-job wait — ~10 s on a POS80. So:
   - **Auto-print** (cash moved at this till: fresh cash sale, drawer move, cash refund / credit / expense / supplier
     pay — the `DeferDrawerKick` cases): the image job is **kick bytes first** (`escpos.DrawerKick(cfg)`, i.e.
     `OpenCashDrawer` + `DrawerKickPin` from Settings) → the picture → cut, to `receiptQueue` — the `kick` flag on the
     image upload, same rule as Classic's `?kick=1` (sale: only when paid in cash — the upload names its slip path, so the server applies the same `paidCash` check as Classic, never trusting the flag alone).
   - **Ask-to-print mode, and moves with no slip:** the server's own standalone kick at the money event, unchanged.
   - Reprints never kick. If the image can't be made and the job falls back to Classic, the kick goes out once.
7. **Preview = the same picture.** `slipOut` / ask-to-print, the Receipts tab, the sale sheet and the money receipt
   sheet show the rendered PNG itself (`image-rendering: pixelated`), the exact pixels that go to the printer.
   Print sends that same PNG; nothing re-renders between preview and print. In **Classic** mode those screens show
   `SlipPaper` (the decoded bytes, already true to the paper). **`Paper.tsx` is deleted**: no screen may show a
   receipt drawing that isn't one of the two true sources.
8. **Plugins.** A plugin slip (repairs job slip today) can register its template through a plugin slot (extend
   `tillReceiptKinds` or a new `slipTemplates` slot keyed by print path; builder's call). A slip kind **without** a
   template prints Classic automatically, even in Image mode, so nothing breaks. In this step, give the repairs slip
   a template too, so every slip that exists today prints as an image. Core stays plugin-inert.
9. **Every slip still goes through `slipOut`** (memory "Slips follow ask-to-print"). Image vs Classic is decided
   inside `slipOut` / `printSlip`, not at the call sites.

## Proving it without a real printer

No real printers yet (user: will get some if needed). The emulator `pos_printer` (escpos-netprinter,
`tcp://localhost:9100`, web view on :8631, `docs/acceptance.md`) decodes `GS v 0`.

- **Go test:** build the image job from a known PNG, decode the bytes back with `escpos.Preview` (it already
  handles `GS v 0`), and assert the bitmap equals the input **pixel for pixel**, bands included.
- **Playwright** (Chromium + Firefox, memory "check prints in Firefox"): print each slip kind in Image mode to the
  emulator, then compare the preview PNG with what the server sent.
- **Shot review:** for each kind, a screenshot of the preview next to the emulator's rendering, at 58 and 80 mm,
  Image vs Classic. The user reviews these before merging. Check that text stays crisp: no strokes thinner than
  2 dots, and nothing smaller than about 20 px (≈ 9 pt at 203 dpi; the idea note's "under ~9–10 px blurs" means
  real-world mm).

## Constraints

- Files ≤ 500 lines, organised by concern. Core is plugin-inert.
- Job size: an 80 mm receipt image is about 50–100 KB against about 2 KB of text. Fine on USB and network. Slow
  serial printers keep Classic via the switch.
- Kiosk Chromium and Firefox both have to work. Rasterising happens in the cashier's browser, so it has to be
  fast: well under a second for a typical receipt on a till PC.
- Thermal printing stays in Go for transport and commands (AGENTS); only the layout moves to React.

## Left to the builder

- Exact DTO shapes for `?data=1`, file split under `web/src/features/print/`, and the slot shape for plugin
  templates.
- The Latin font choice (a clean monospace or sans that matches today's printed columns closely).
- Error UX when rasterising fails in a browser (my call: fall back to Classic for that one job and log it, never a
  dead print).

## Calls made after the builder check (mine — the user can overrule)

- **"No redesign" means same lines, order, alignment and column widths as Classic** — not the printer's built-in
  font pixel for pixel (a web font can't be PC437 Font A). Sinhala / Tamil item names, which Classic can't print,
  now print correctly — welcome, not a redesign.
- **The template copies the `classic` receipt style only.** modern / boxed / compact stay Classic-mode-only and are
  rethought in the customisation step; the switch's help text says a shop on another style will see the classic
  layout in Image mode.
- **Tests don't need the emulator:** e2e captures the job on the server (a test print target) and decodes it with
  `escpos.Preview` against the preview PNG. The emulator is for the hand-checked shot review only.
- **Kick target:** a kick folded into a print job follows that job's printer (`receiptQueue`), as Classic does today.
  Standalone kicks (ask-to-print mode, moves with no slip) still go to the shop's receipt printer, as today — the
  multi-counter case is an existing limit, not changed here.
- `?size=58|80` reprints: the image endpoint checks the PNG width against the job's size, not the setting.
- A fallback to Classic for one job never sends the kick twice.
