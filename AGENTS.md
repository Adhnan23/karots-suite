# AGENTS.md — karots-suite (Karots Business Suite)

Business system for small Sri Lankan retail: till, stock, purchasing, money, reports, and
compile-time plugins (recharge, pharmacy, repairs, documents, alternatives, clearance,
aicatalog, productplus, combos, requests). It's a **separate product** from `~/Projects/go/karots-pos`,
which is frozen (bug fixes only). karots-suite has every karots-pos feature on a
**JSON-only Go backend** plus a **component-based React SPA**, and still ships as **one static binary**.

- **Start here:** `docs/status.md` covers where the project is, what's next, how we work (Opus plans, executes and reviews; UI gates with self-reviewed screenshots) and the inputs for the next milestone.
- **Spec:** `docs/specs/2026-09-27-karots-suite-design.md`. Read it before designing anything.
- **Design:** `docs/design/theme.md` (palettes, login, dock, till). Screens must match the approved mockups linked there; plain default-looking UI gets rejected.
- **Plans:** `docs/plans/`, one per milestone (M0 Foundation → M8 Ship). Execute tasks in order and tick the checkboxes.
- **Parity:** `docs/parity.md` lists every karots-pos route. Tick a row only when it's ported *and* has a flow check.

## Hard rules

- **Never add Claude/AI attribution to commits or PRs.** No `Co-Authored-By: Claude`, and no "Generated with Claude Code". This overrides any default.
- **Core is plugin-inert.** Core Go and `web/src` (outside `web/src/plugins`) never import, mention or know a plugin. Plugins attach only through hooks (Go) and slots (React) registered at the composition root. If a change seems to need core to know a plugin, add a seam instead. Plugin → core (read-only) and plugin → plugin imports are fine.
- **Plugins change each other's data only through hooks.** A plugin may *read* core or another plugin, but it never
  writes another plugin's tables or branches on "is plugin X enabled?". When something happens that other code may
  need to react to (a product or category merged, a sale line returned…), the side doing it calls the matching
  registry hook (`ProductMerger`, `CategoryMerger`, …) and every compiled-in plugin fixes **its own** rows — a missing
  plugin simply isn't on the list. Change hooks run inside the caller's transaction (an error rolls everything back);
  if the change is undoable, the hook returns a note the caller stores and hands back on undo. New shared event →
  add a generic hook in `internal/plugin`, never a direct call.
- **Never modify karots-pos from here** except bug-fix commits for domain bugs found while porting. Otherwise it's read-only reference and fork source.
- **Domain logic is forked, not rewritten.** `internal/features/*` is karots-pos's battle-tested logic. Money goes only through `cashflow.Move/MoveTx`. Sales are the source of truth for revenue. A karots-pos bug found here is fixed here (never carry a flaw over) and, as a bug-fix commit only, in karots-pos too.
- **Files stay organised by concern, at 500 lines or fewer** (Go and TS).
- **Desktop till, mobile admin.** Every admin page, report and chart must work at 360px. The till is desktop/kiosk only.
- **Palettes, each with light and dark, and it must not look like karots-pos.** Components use theme tokens only (CSS variables — colours, `--radius`, `rounded-pill`, `--shadow`, `--glass`, `duration-(--dur)`, Motion via `tx()`), never raw colour classes, pixel corners or literal timings (`theme/guard.test.ts` fails on them), and never branch on which palette is active. See `docs/design/theme.md`: theme **Dock** has 22 built-ins (Studio, Night, Bazaar, Lagoon, Catppuccin + 17 famous / colourful ones, theme.md §1) and custom palettes, chosen shop-wide by the system user (`settings.skin`); **mode** (light/dark) is per device (`ks-mode` in `localStorage`). One CSS file per built-in in `web/src/theme/palettes/dock/`. Every screen works with every palette in both modes.

## Layout

```
cmd/server/           boot, migrations, flags (-migrate -seed -demo -reset -init -support-pin; -backup <dir> /
                      -restore <file> for Setup — the till must be stopped);
                      enabled_plugins.go is GENERATED (committed core-only)
cmd/bootstrap/        Shop Builder: no flags = browser app — the shop list (shops.json profiles: plugins, OS,
                      database mode, ports, secrets, install id, channel) + Quick build; -shop <key|all>, -list,
                      -import-pos; writes both plugin lists, builds, restores them core-only. A shop build =
                      dist/<key>/ karots-setup + install.conf + the binary ("Add installer" off = binary only)
cmd/setup/            karots-setup, the shop's installer: double-click → page (UAC / pkexec) → modular steps per
                      role (main PC | cashier PC) and mode (file | server); only applies install.conf; -yes headless.
                      Uninstall (backup first → kept in C:\Karots backups / ~/Karots backups; nothing removed if it
                      fails), Bring back a shop at a first install, mode change on Update (backup → new mode → restore)
internal/shopconf/    install.conf — the Builder ⇄ installer contract (install settings + the final .env)
shops.json            every shop's profile, committed (private repo — it holds each shop's secrets)
internal/filedb/      file mode: the shop's own PostgreSQL 17 in data/ next to the binary (no DATABASE_URL +
                      a `-tags filemode` build, which embeds bundle/postgres.txz — placed by the builder)
plugins/<key>/        a plugin's Go half: plugin.json, init() → plugin.Register, own migrations
                      (goose_db_version_<key>), JSON routes on reg.Admin()/Cashier() → /api/v1/x/…
internal/app/         BuildServer + services.go (service construction + core seam wiring)
internal/api/         JSON /api/v1 — dto.go (all request/response types) + one file per area
                      till*.go — /api/v1/till/* (config, catalog, customers, drawer, sales, held, printing)
                      counter*.go — the till's counter tabs (dto_counter.go); reprints take ?size=58|80
                      admin*.go — /api/v1/admin/* (admin + manager): catalog (dto_catalog.go), stock
                      (dto_stock.go), purchasing — suppliers, purchases, reorder, returns, money sources
                      (dto_purchasing.go) — and people — customers, users (admin only) (dto_people.go);
                      sheets.go = CSV/XLSX/ODS download + import helpers
internal/spa/         go:embed of the built SPA (dist/) + index.html fallback
internal/features/*   forked domain services
internal/features/palettes/ custom palettes (CSS kept as-is; settings.skin = custom-<id>; public palette.css)
internal/features/reorder/ smart reorder levels: calc.go (pure arithmetic) + run.go (nightly 03:00 / boot when stale);
                      Auto products get reorder_level written, every product reorder_calc (the reason lines)
internal/update/      self-update (M8c): signed manifest in the shop's bucket folder (channel), check / download /
                      swap / BootGuard rollback; restart = exit 75, the supervisor (systemd / karots-start) starts it
internal/plugin/      plugin registry + data/behaviour hooks (Go side); started by internal/app/plugins.go
internal/middleware/  JWT cookie auth + guards (lock, PIN, role, system, kiosk, supplier)
migrations/           00001_baseline.sql (squashed karots-pos 0001–0075), 00002 sale line groups, 00003 product barcodes, 00004 recovery extra charge, 00005 product packs, 00006 backup copies, 00007 till closed_by; 00008 reorder auto + low-stock permission, 00009 unset (0) levels → Auto, 00010 print as (classic | image), 00011 custom palettes, 00012 label pictures (label_print_as + label_styles); next is 00013_
web/                  Vite + React + TS app → builds into internal/spa/dist
  go.mod              EMPTY module fence — stops `go ./...` walking node_modules. Do not delete.
  src/api/            client.ts (api<T>, ApiError), session.ts (meQuery, redirectFor), types.gen.ts
  src/components/ui/  shadcn primitives (owned, themeable)
  src/components/admin/ admin kit: PageHead, DataList (table/cards), FormSheet, PickerShell, Pager,
                      fields, ConfirmButton, EmojiPicker — every admin screen is built from these
  src/theme/          tokens.css + palettes/dock/*.css (palettes/tokens.ts = the token list), appearance.ts,
                      mode.ts, motion.ts (tx() for Motion), guard.test.ts,
                      AdminShell (dock, no sidebar) + Dock/Shelf/PhoneTabs/CommandPalette, TillShell, nav
  src/features/appearance/ Appearance (system user): PalettePicker, PaletteEditor (+ EditorControls, PreviewStrip,
                      draft.ts live preview, presets.ts), paletteCss.ts (palette file read/write), ReceiptStyles
  src/features/auth/  LoginHero, PinField (no browser memory), PinPad, AutoLock
  src/features/*      screens by area; src/routes/ = TanStack Router file routes
  src/features/till/  sell screen: cart/ (pure logic + reducer, Vitest), drawer/, catalog/, cart-ui/, pay/, checkout/
                      state in TillProvider (per-user sessionStorage); dialogs via TillUi; data via api.ts queries
  src/features/print/ Image printing: picture.ts (slip → PNG at printer dots), templates/ (one per slip kind),
                      styles/<key>/ (the ten receipt styles' kits + whole-slip overrides; STYLES in styles/index.ts),
                      labels/ (labels as pictures: kit (paper, kinds, INK), codes (bars / QR at whole dots + quiet
                      zones), registry (kinds + fallbacks), picture (labelPictureQuery), print (printLabel: Image or
                      Classic, a failed picture prints Classic), LabelView; price/ = core's eight price styles)
  src/features/counter/ till tabs other than Sell (returns, credit, receipts, write-off, convert, labels, low stock,
                      warranty, suppliers, expenses): CounterPage (find | act), ProductFind/SaleFind, queries.ts
  src/features/admin/ admin screens by area (products, categories, units, groups, services, stock,
                      intake, conversions, labels); shared pickers: CategoryPicker, ProductPicker
  src/features/purchasing/ suppliers, purchases (entry / receive / PO + GRN docs), reorder, returns;
                      shared LineEditor + totals (margin guard) used by the counter too, MoneySourcePicker
  src/features/people/ customers, users, OpeningSheet (old balance), StatementSheet (print)
  src/routes/_authed/print/  printable pages (receipt, X/Z report, order, PO, GRN, statement, label sheet) — paper/ink tokens
  src/plugins/        slots.ts + GENERATED enabled-plugins.ts (committed core-only) + <key>/index.tsx
                      (each plugin's React half: export default register(slots))
  src/plugins/slots.ts  typed plugin slots (apps, adminPages → /admin/x/<key>/*, tillMenu, tillActions,
                        tillCartHints, tillLineHints, drawerSections (on: open|close), tillTenders,
                        tillTabs, tillReceiptKinds, slipTemplates (a plugin slip's Image-mode
                        template, by its ?data=1 kind — drawn with the style kit it gets, so it
                        follows every receipt style; label + sample for Appearance), labelTemplates (a plugin's
                        label kind "<plugin>.<kind>": sample + styles, Classic first; print it with printLabel),
                        returnLineFields, reportCards, settingsSections,
                        productFormSections, before/after-sale); TillApi.addGroup = cart line groups
                        (kept together, stored on sale_items.line_group*, printed as one block);
                        TillApi.addPayout = money given to the customer (a sale may total below zero)
scripts/              squash-migrations.sh, parity.sh, e2e-users.sql (fixed e2e logins, not -seed's demo ones)
```

## Stack

- **Backend:** Go, Echo v4, sqlx, goose, Postgres 17. Postgres-only SQL is deliberate (pg_trgm, `::numeric`, `RETURNING`, `ON CONFLICT`).
- **Frontend:**
  - bun, Vite, React, TypeScript strict
  - TanStack Router, Query, Table, Form + Zod (add Table and Form when first needed)
  - Tailwind v4, shadcn/ui, sonner, lucide
  - ECharts for charts, Motion for animation
- **Types:** tygo generates `web/src/api/types.gen.ts` from `internal/api/dto.go`. Never hand-edit the generated file.
- **TanStack skills:** before TanStack work, run `intent list` in `web/` and `intent load` the matching skill.

## API conventions

- All JSON lives under `/api/v1`. Success is plain JSON. Errors are `{"error":{"code","message"}}`, from `apperr`.
- JSON fields are camelCase. Money is sent as decimal strings, never floats. Dates are ISO 8601.
- Protected groups use `h.protected` (jwt → unlocked → PIN chosen). `/me` and `/auth/*` are jwt-only by design.
- **Session codes the SPA routes on:** `UNAUTHORIZED` → `/login`, `SESSION_LOCKED` → `/lock`, `PIN_CHANGE_REQUIRED` → `/account/pin`.
- **Session cookie:** `pos_token`, httpOnly, SameSite=Strict. No token is ever visible to JS.
- **Device id:** every call carries `X-Ks-Device` (`web/src/lib/device.ts`); signed-in screens and the lock page beat
  `POST /presence` for the admin's Who's on now (`internal/features/presence`, in memory).
- **Porting an area:** add DTOs to `dto.go`, then `make types`, then write handlers in `internal/api/<area>.go`, then **delete that feature's legacy `api.go`/`RegisterAPI`**, and tick the rows in `docs/parity.md`.
- **Thermal printing stays in Go** (`internal/escpos`). A4 and PDF documents are React print routes.

## Dev workflow

- **Ports:** Postgres `suite_db` on **5433**, Go server on **3100**, Vite on **5173** (proxies `/api`). karots-pos keeps 5432 and 3000, so both can run side by side.
- `make db-up` → `make migrate` → `make dev` (air + Vite).
- `make test` (Go, DB tests gated on `DATABASE_URL`) · `make test-web` (Vitest) · `make e2e-seed && cd web && bunx playwright test`.
- `make types` after any `dto.go` change. `make build` → `make verify-portable` (static, `CGO_ENABLED=0 GOAMD64=v1`).
- **Plugins:** `make plugins P=all` (or `P=clearance,…`) compiles plugins into the working tree for dev;
  `make core-only` puts the lists back — **never commit a plugin selection**. `make check-core` (in
  `make verify`) fails if core names a plugin or a list isn't core-only. Playwright builds with every
  plugin. A shop build: `make bootstrap` (Shop Builder in the browser).
- **Shop builds:** `make bootstrap` → Shops → **Release…** (build + publish to the shop's folder in the `karots-suite`
  R2 bucket; `make shop S=al-raj`) or **Test build** (local only, marked "Test build" on the Data page; `make shop-test
  S=al-raj`). A shop's **first install must be a Release**, or its Data page says "No update published". Release refuses
  uncommitted changes (except `shops.json` and the plugin lists). `.env` needs `POS_SUPPORT_SECRET`, `R2_*`,
  `UPDATE_BASE_URL`; `SUITE_RELEASE_KEY` is made on the first build — **keep a copy** (shops only take updates signed by
  it). Shops update from Setup → Data → Updates → Update (never by themselves). **Backup copies** (M8c2): every
  automatic backup is also copied to the shop's copy folder (USB / network; Builder preset `BACKUP_COPY_DIR`, then the
  Data page owns it — `backup_copies` table, never backed up) and, for paying shops, sealed to Karots cloud (Builder
  `.env` `CLOUD_BACKUP_URL/KEY_ID/SECRET`, tick per shop; `docs/cloud-backup-bucket.md`). A Windows
  server-mode main PC: tick **+ PostgreSQL** (or `-postgres`) to put the PostgreSQL installer (cached once in the user
  cache, SHA-256 pinned in `internal/shopconf`) beside the files — Setup uses it before winget / downloading. Carry `dist/<key>/` (three files) to the shop; double-click `karots-setup` there (Linux headless: `sudo
  ./karots-setup -yes`). A changed profile reaches a shop by building again and running Setup → Update — a
  **file ⇄ server change** is noticed there and carries the data across once "Change it" is ticked (`-change-mode`).
  **Move a shop to another PC:** Setup → Uninstall on the old one (its final backup), then the three files + that backup
  on the new PC → Setup offers it under "Bring back a shop" (`-restore <file>`). Headless: `-uninstall
  [-remove-postgres] [-keep-dir …]`; `-no-backup` is for technicians only. Look at the
  installer page without installing: `karots-setup -dry` beside an install.conf. `build.ps1` = make on Windows.
- **File mode** (one-PC shops): Shop Builder → Database "One PC", or `go run ./cmd/bootstrap -plugins none
  -mode file`. Run the binary with no `DATABASE_URL` (and no `.env` setting it): data, auto-backups and the
  generated secret go to `data/` beside it (`-data` / `FILE_DB_PORT`, default 5434). `go test ./internal/filedb`
  downloads PostgreSQL once into the user cache (skipped with `-short` or offline).
- **The dev DB is disposable.** Drop and re-migrate freely.
- **Work style:** one task at a time, test-first, one commit per step, working on `main` (local).

## Hidden behaviours (must keep working)

- **System user** `0000000001`: its PIN rotates hourly (install ID + HMAC); `POS_SYSTEM_PIN` overrides it, with `2273` as the dev fallback. It's invisible to the shop and excluded from Activity and audit. It reaches Appearance by tapping its own name 8 times in the avatar menu — no nav item, ever.
- **Login-page Konami** (↑↑↓↓←→←→BA) reveals the install ID and server time. The install ID is also logged to the console.
- **Clock easter egg:** 7 taps on the login / lock clock (within 3s) open a games menu (`GameMenu.tsx`; tap or 1 2 3; closing a game returns to it) — coin catch (`CoinGame.tsx`), falling blocks (`BlockGame.tsx`) or Shop Rush (`ShopGame.tsx`: ring up a customer, then drag the customer's money into the cash drawer and give change from the shop's own notes (`loginInfo.notes/coins`) (no change total shown — work it out) or drag the customer's card into the card machine and key the amount, then drag the printed receipt (`loginInfo.receipt`, the real `Paper` look) to the customer), in `features/auth/`, best score per device; Esc closes it. It only paints over the page. Core, not a plugin (user: a pure easter egg); a game on the lock screen shows as "Playing …" on Who's on now.
- **Kiosk Konami** → `POST /api/v1/kiosk/exit`, for admin and system only. The server writes the relaunch loop's sentinel and kills the `--kiosk` browser.
- **Auto-lock** after `lockTimeoutMinutes` of idle time. Unlock works for the same user or any admin.
- **Kiosk lockdown** (`web/src/lib/lockdown.ts`, karots-pos `core.js`): F1–F12 never reach the browser; F1 on a counter tab goes back to Sell; on the till / account / lock pages browser Ctrl combos, Alt+←/→ and right-click (outside text boxes) are off. **The system user is exempt** (support needs DevTools) — suite-only.
- **Till semantics:** F1 / "⟲ Menu" clears the **search only**. Only New Sale clears the cart.
- **No browser memory on PIN entry** (shared counter): `autocomplete="off"`, non-standard field names, and PIN inputs are `type="text"` with `-webkit-text-security: disc` — never `type="password"`, or a password manager will offer to save it.

## Gotchas

- `vite build` empties `internal/spa/dist` (including `.gitkeep`). `make web` restores it, so always build through make.
- `migrations/00001_baseline.sql` re-randomises `settings.install_id`. Never hard-code one in a migration, or every shop shares a support PIN.
- `go run` children can survive `pkill` and leave a stale server. Restart cleanly.
- The login limiter (burst 10, +1 per 2s) is keyed on the socket IP. The e2e specs import `test` from `e2e/fixtures.ts`, which waits out a 429. Import from there, not `@playwright/test`, or a full run fails on sign-in.
- Playwright's `webServer` (`reuseExistingServer: true`) happily reuses whatever is already listening on 3100/5173 — including a dev server you started by hand — and serves stale code with no warning. Kill anything on those ports before `bunx playwright test` if you've been poking at the app manually.
- Go DB tests and Playwright share `suite_db`: never run `go test` while a Playwright run is going — it fails unrelated flows (a restore, the drawer, duplicate names).
- A CSS `background` shorthand silently drops the **whole declaration** if a colour lands on a non-final layer — which happens the moment a palette's `--bg-art` becomes a 2+ layer gradient (Night does; Studio's `none` doesn't). Set `background-color` / `background-image` / `background-attachment` as longhand instead of folding a token into the shorthand.
- **File mode never uses a plain `C` locale:** pg_trgm then treats every Sinhala / Tamil letter as punctuation and search finds nothing. Never let embedded-postgres see a data folder it can't use either — it **wipes and re-inits** one whose `PG_VERSION` doesn't match (`filedb.checkDataDir` refuses first).
- **Windows file mode carries the VC++ runtime** (`internal/filedb/vcrt/`: msvcp140, vcruntime140, vcruntime140_1 — zonky's PostgreSQL needs them and a fresh Windows lacks them → initdb `0xc0000135`). They are written to `data\vcrt\` and put first on PATH. Keep them; update them only from a Microsoft vc_redist x64 copy and record the new SHA-256 in `embed_windows.go`.
