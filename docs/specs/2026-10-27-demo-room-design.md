# M8d Demo Room — design (2026-10-27)

## Why

The Demo Room is the sales tool: the owner opens it in front of a shop owner (on the owner's laptop or on the
customer's PC) and the customer sees *their own future shop*, busy and a year old, not an empty install.

karots-pos had one (`cmd/demo`, `internal/demogen`, `internal/demodb`). It is **what to avoid, not a fork source**.
In the user's words its "normal functionality didn't work well inside the demo room": plugins sometimes didn't load
for a profile, receipts went wrong, opening a new till broke. The lesson this design is built on:

> **The demo is the real suite.** Real binary code paths, real plugins registered the normal way, real database,
> real print path. Only the *data*, the *shop picker*, the *chrome* (watermark, paper strip, cheat menu) and the
> *lock* are demo-specific — and they sit **outside** the features.

## Goals

1. **Five built-in shops** (user-approved), each with its own plugin set, palette, receipt style, label styles and
   catalogue:

   | Shop | Plugins | Look (my call, adjustable) |
   |---|---|---|
   | Grocery / mini-mart | none (core only) — packs, smart low stock, credit | Studio |
   | Pharmacy | pharmacy, alternatives, productplus, requests | Lagoon |
   | Phone & repair | repairs, recharge, combos, documents | Synthwave '84 |
   | Hardware / building | clearance, productplus, documents, requests | Bazaar |
   | Everything shop | every plugin | Aurora |

   aicatalog only appears in the Everything shop (it needs an AI key; see "Left to the builder").
2. **Make your own** profile from the picker: pick plugins, palette, receipt style; it gets a generic catalogue
   plus the picked plugins' data. It is **thrown away on restart** like everything else (user).
3. **A year-old shop, ending today, every launch.** History looks like a system running 12+ months: weekly and
   seasonal rhythm in reports, smart reorder levels with real sales behind them (Auto levels computed, Low stock
   tab populated), suppliers, purchases (paid + on credit), purchase returns, customers on credit with payments,
   sale returns, write-offs, expenses, closed tills with counts and Z reports, and each plugin's own kinds of
   activity (prescriptions + dose labels + batches/expiry; repair jobs at every stage + warranty; recharge float,
   commission, payouts; quotes/invoices; clearance lines; combos; requests; alternatives). Opening it weeks later
   still shows today's sales — **never stale**.
4. **Nothing persists** (user): whatever is done in a demo is gone when the server closes. Next launch is fresh.
5. **Alive on its own** (user): background "staff" sell on their own tills through the normal sale path, a few
   a minute, busier at lunch / evening; Who's on now shows them; dashboard moves; stock runs down into low stock;
   now and then a credit sale, a return, and plugin events (a repair checked in, a recharge, a prescription).
   A **Pause** switch freezes it while explaining.
6. **Printers inside** (user): a receipt printer and a label printer emulator. Every real print (sale, reprint,
   X/Z, label, plugin slips) appears as a **paper strip sliding out on the same screen**. All shops print as
   **Image** (receipts and labels), so the customer sees the best output.
7. **Cheat menu** (user approved the list, "add more if needed"): see below.
8. **Watermark** (user): always visible "DEMO" mark with the owner's name and contact number, so a copy left on a
   customer's PC points back to Karots.
9. **Demo lock** (user): the Demo Room has its own install ID; on opening it asks for a code — the same hourly
   rotating PIN as the system user, read from the owner's PIN app using that install ID.
10. **Double-click to run**, like a file-mode shop: one binary (+ nothing else), Windows and Linux, no Docker, no
    DATABASE_URL, works offline.

## Non-goals

- Not a training / practice mode for real shops (practice mode was removed; stays removed).
- Not a hosted web demo. **One computer only** (user): the launcher listens on `127.0.0.1` only — no phones, no
  second PC as a till.
- No saving, exporting or restoring demo data. No updates channel (a new demo = a new binary).
- No real printers from the demo (the emulators replace them). Real-printer testing stays a separate step.

## Anti-goals (failure even if it "works")

- **Any demo-only branch inside features, core or plugins** (`if demo { … }`). That is exactly how karots-pos's
  demo drifted. Core stays demo-inert the same way it is plugin-inert.
- A plugin that loads differently in the demo than in a shop build.
- Dates that look stale, empty reports, a first screen that looks like a fresh install.
- A demo that can be used as a free working POS on a customer's PC (the lock + reset + watermark prevent it).
- A customer seeing an error, a broken print, or a "this feature isn't set up" screen in any built-in shop.

## Shape of the how

### One demo binary, the real server inside

- New command `cmd/demo` (fresh code; karots-pos's is reference-only). Built with **every plugin compiled in**
  and `-tags filemode` (embedded PostgreSQL, as file mode).
- **The demo must not affect the suite (user, 2026-10-27).** Shop builds behave exactly as today. Any seam the
  demo needs is **inert unless `cmd/demo` calls it** — no env var, flag or setting a shop could trip — and adds no
  work to a shop's request path. Everything else lives in `cmd/demo` / `internal/demo*`, which shop builds don't
  import.
- **Runtime plugin selection** (my call, the one seam into the plugin registry): a function such as
  `plugin.Only(keys)` that only `cmd/demo` calls before `SetupAll`; `cmd/server` never calls it, so a shop runs
  every compiled plugin as now. Plugin migrations follow the same list. The SPA side: the demo build serves the
  active list (e.g. in login-info, only when set) and the SPA's plugin registration skips plugins not on it;
  absent list = today's behaviour. A plugin is never told it's in a demo; it is simply on or off the list — the
  same outcome as a shop build that didn't compile it. `make check-core` must still pass.
- **Each shop runs as its own child process** of the demo launcher (my call): the launcher owns the embedded
  PostgreSQL, the lock, the picker and the port the browser talks to; picking a shop starts the real server
  (`internal/app.BuildServer`, same as `cmd/server`) as a child with that shop's database and active plugin list,
  and proxies to it. Switching = stop child, drop its database, start a fresh one. Reason: plugin registration and
  product hooks are process globals; a fresh process guarantees a clean load every time (the karots-pos failure).
- **Throwaway data**: the embedded PostgreSQL data folder lives in a temp directory, wiped on start and on exit.
- Demo chrome (watermark, strip, cheats) is injected by the **launcher proxy** into the pages it forwards, so the
  server and SPA bundle stay identical to a shop's. `BuildServer`'s existing `pre ...echo.MiddlewareFunc` is the
  fallback (its comment still names the old HTMX / Templ demo — fix that comment).

### The shop's history (generator)

- A generator (new package, e.g. `internal/demoshop`) builds each shop **through the real services** (sales,
  cashflow.Move, purchases, register sessions, plugin services/endpoints) — never raw INSERTs into feature tables,
  so the data is exactly what the app would have written. Backdating may adjust timestamps after the fact (as the
  current `cmd/server/demo.go` `touch` does) or via an injected clock — builder's choice.
- Each shop is a **profile description** (catalogue, suppliers, customers, staff, plugin activity recipes, volume
  curve, palette, receipt style, label styles, settings). **Plugin data is made by the demo calling each plugin's
  own JSON endpoints** (`/api/v1/x/<key>/…`), the same calls its screens make — plugins carry no demo code at all,
  and the demo package (outside core) is allowed to know plugins, as plugin → core reads are.
- History always **ends today**, generated at launch, deterministic per (shop, date) seed so the same day looks the
  same. Today already has sales up to "now" and one or two tills open.
- **Speed:** a shop must be ready within ~30 s of picking (my call), with a progress screen. If generating a year
  through services is too slow, the builder may generate a template at build time and shift dates at launch — but
  only if every date-bearing column is covered by a test that fails when a new one is added.
- Names, products, prices (LKR), suppliers and customers look Sri Lankan and believable, including some Sinhala /
  Tamil product names.

### Alive

- A background simulator in the launcher (or child) signs in demo staff and makes sales / events **through the
  real HTTP API** (or the same services), so it exercises the same paths the customer sees. It beats presence so
  Who's on now shows them on their tills. Rate follows a day curve; Pause in the chrome stops it.
- It never touches the till the owner is using.

### Printers

- The launcher opens two local TCP listeners (receipt, label). Each shop's settings point the receipt / label
  printer at them as network printers (`tcp://127.0.0.1:<port>`, already supported by `internal/printing`), with
  print-as **Image** for both. So the real print path runs unchanged.
- The listener decodes what arrives (ESC/POS raster for receipts, TSPL `BITMAP` for labels — both already produced
  by the suite) into a picture and pushes it to the browser; the paper strip slides out on the current screen
  (receipt from the edge, label as a small sticker), can be dismissed, and keeps the last few prints.
- Cash-drawer kick shows as a small "drawer opened" cue.

### Chrome (watermark, Switch shop, Pause, cheats)

- Drawn by the demo, over the real app, on every page including login, lock and print pages. **A4 / PDF print
  pages (quotes, invoices, statements, PO, GRN, label sheet) keep a diagonal "DEMO" watermark when printed or saved**
  (print CSS), so the demo can't produce clean documents (my call). Thermal slip pictures: a small "DEMO" line is
  acceptable (builder's call).
- **Watermark:** "DEMO — Karots Business Suite · <owner name> · <phone>" always on, not dismissable. **One demo for
  everyone, the owner's details baked in as a constant** (user); exact name and number asked for during the build
  (placeholder until then).
- **Switch shop** button → back to the picker (confirm: "what you did here will be lost").
- **Pause** toggle for the alive simulator.
- Theme tokens only (theme guard applies); works at 360 px for admin pages.

### Cheat menu

Hidden key (e.g. Ctrl+Shift+D or 5 taps on the watermark — builder's call; not discoverable by a customer). Every
move uses the real services / API.

- **Open my till** — open a register for the signed-in user with a sensible float, land on Sell.
- **Sign in as…** owner / manager / cashier / system, no PIN typing.
- **Make it busy** — burst of ~20 sales now across the staff tills.
- **Jump to closing time** — fills the open tills with a day's worth of sales so close + Z report have a real
  day to show. The clock does **not** move (no clock seam in core — it would touch every `time.Now()`); for the
  same reason history is **backdated after the fact**, as `cmd/server/demo.go`'s `touch` does (my call).
- **Low stock now** — push a few products under level so reorder / Low stock have suggestions.
- **Credit customer due** — a customer over their limit / with an old balance, for the credit story.
- **Supplier delivery** — a purchase arrives to receive (GRN).
- **Plugin moves** (only for active plugins, done by the demo through each plugin's own endpoints): repair ready
  for pickup, prescription waiting, recharge float low, quote to convert, clearance batch, combo on offer, a
  customer request in.
- **Print samples** — a receipt, a label and a plugin slip to the paper strip.
- **Cycle palette** / **cycle receipt style** — show the looks without the system-user detour.
- **Reset this shop** — fresh copy of the current shop without going back to the picker.

### Flow

1. Double-click → browser opens on the launcher page.
2. **Demo lock:** enter the hourly code (install ID shown on that page, as on the login Konami). Wrong code → stay.
   The code unlocks for the life of the process (my call); closing the demo locks it again.
3. **Shop picker:** five cards (name, plugins, a palette preview) + **Make your own**.
4. Pick → progress screen → the shop's real **login page**, with the demo logins shown in the chrome
   (owner / manager / cashier) so no PIN is fumbled.
5. Demo as normal; Switch shop any time.

### Lock

- The demo binary has its own fixed install ID (generated once, baked at build time) so the owner can add it to the
  PIN app; the code is the same HMAC as the system user's rotating PIN, with the suite's support label. Like shop builds, it bakes only the seed derived for that install ID
  (`cmd/bootstrap/build.go`), never `POS_SUPPORT_SECRET`.
- Inside a shop, the system user works as usual (its PIN rotates on the same install ID).

### Build

- `make demo` (and a Shop Builder button if cheap) → `dist/demo/karots-demo` for Linux and `karots-demo.exe` for
  Windows; static, portable, file-mode bundle embedded; VC++ runtime as file mode does.

## Constraints

- AGENTS.md rules apply: core plugin-inert (and demo-inert), plugins change each other only via hooks, files ≤ 500
  lines by concern, theme tokens only, money only through `cashflow.Move/MoveTx`, sales are the revenue truth.
- `make check-core` and the theme guard must pass; a shop build's `enabled_plugins` lists stay core-only in commits.
- Must work offline on a fresh Windows PC and on Linux.
- The demo must never write outside its temp folder (no shop data on a customer's PC survives a close).

## Decided by the user

Five shops as listed; custom profiles thrown away on restart; nothing persists; year-plus believable history kept
current; per-shop palette + receipt + label style; Image printing everywhere; paper strip on the same screen;
watermark with owner contact; rotating-code lock on its own install ID; alive by itself; cheat menu (list above,
more allowed); runs on the owner's laptop or a customer's PC; one computer only (no phones / second PC); one
demo with the owner's details baked in; **the demo must not affect the suite** — seams only, inert in shop builds.

## My calls (flag any)

Shop looks per shop; child process per shop; `plugin.Only` called only by `cmd/demo`; plugin data via each plugin's
own endpoints (no demo code in plugins); ~30 s ready target; code unlocks for the process lifetime; extra cheats (credit due, supplier delivery, cycle
looks, reset this shop); aicatalog only in Everything.

## Left to the builder

Exact catalogue contents and volumes; generator internals (clock injection vs backdating); simulator rates;
emulator decoding details; cheat-menu key; how the paper strip looks (must match the theme); whether aicatalog shows
canned results offline or simply sits idle; exact Who's on now staff names.
