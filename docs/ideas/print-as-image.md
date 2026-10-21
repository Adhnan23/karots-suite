# Print as image — what you see is what prints (idea, 2026-10-21 — next, before Demo Room)

Planning only so far (user + Claude, chat). Core feature.

## The problem (user)

Printing works everywhere (Go builds ESC/POS for receipts, TSPL for labels, and sends raw bytes through CUPS
`lp -o raw`, the Windows spooler, or `tcp://host:9100` — `internal/printing`, `internal/escpos`, `internal/tspl`),
but **what we see is not what prints**: the on-screen preview (`web/src/features/counter/receipts/Paper.tsx`, label
previews) and the printer bytes are built separately, so they drift; every layout change is Go code; customising is
weak. The receipt styles (classic / modern / boxed / compact — `internal/escpos/style.go`, system user's
Appearance page) were "not implemented well" (user).

**The real reason (user, 2026-10-21): the printed receipt still looks exactly like karots-pos.** `internal/escpos`
is karots-pos's code almost unchanged (12 lines differ in escpos.go, 2 in style.go), styles included — the screens
are the suite's own, the paper the customer takes home is not (AGENTS: "must not look like karots-pos"). Image
printing is how the suite gets **its own receipt look** (theme-matched, logo, its fonts, Sinhala / Tamil anywhere);
today's layout stays available as one option.

## Agreed in chat

1. **Print a picture of the preview.** Draw the receipt / label / slip as a React page at the printer's dot width
   (58 mm = 384 dots, 80 mm = 576; labels 8 dots per mm, e.g. 50 × 25 mm = 400 × 200), turn it into a 1-bit image in
   the browser (`html-to-image` or `modern-screenshot` → canvas → threshold / dither), send it to the Go server, which
   wraps it in ESC/POS `GS v 0` (receipts) or TSPL `BITMAP` (labels) and sends it through the **same transport as
   today**. Cut and cash-drawer kick stay plain commands around the image.
2. **A switch, not a replacement (user):** Settings → printing: **Image (new)** / **Classic**. Classic stays until
   image printing has proven itself on real printers in shops; then the legacy path is deleted.
3. **Customising becomes page design:** the receipt kinds (sale, refund, credit / debt slips, Z report, plugin slips,
   labels) each a React template; logo, fonts, QR codes, Sinhala / Tamil anywhere. Rethink the receipt-style
   customisation page on top of this (the current four styles were weak).

## Constraints to keep

- A text-mode receipt redesign was rejected before (memory "Receipt layout: keep") — so the image version first
  reproduces today's receipt exactly (proves the pipeline), then the suite's own designs come as choices the owner
  picks; the user approves each look on a mockup before it ships.
- Every slip still goes through `slipOut` / ask-to-print; labels exempt but previewed.
- Text under ~9–10 px blurs at 203 dpi. Jobs are bigger (an 80 mm receipt ~50–100 KB vs ~2 KB): instant on USB /
  network, slower on old serial printers — the Classic switch covers those.
- Printing from Firefox too (memory: check prints in Firefox).

## Still open

- Where the switch lives (shop-wide Settings, or per printer / per user like the receipt printer).
- Labels in the same step, or receipts first.
- Test on the user's real receipt and label printers before Classic goes.

Next step: proposal + mockup in a fresh session, then spec and plan.
