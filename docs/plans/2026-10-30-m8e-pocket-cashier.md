# M8e Pocket Cashier (release 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A phone cashier (plugin `pocket`, sold as the "Pocket Cashier app") that sells live on the shop's wifi and keeps selling from an encrypted copy when the PC is down. The server replays the offline work in order, through the normal core services, and flags anything odd instead of refusing it.

**Architecture:** Core gets six generic seams first: replayable sales with flags, till sessions at a given time, one person one device, extra plugin listeners, the till copy, and full-screen plugin surfaces. Then the plugin `pocket` adds its own CA and HTTPS listener, phone pairing, the copy and a live feed, the action replay, and a service worker. Its React half is the phone app at `/x/pocket/` (a surface) plus the admin Phones page. Every phone write is an **action** with a one-time ref, sent through one endpoint whether the phone is online or offline. One code path means a retried send can never double-book.

**Tech Stack:** Go (Echo, sqlx, goose, crypto/x509), PostgreSQL 17, React + TanStack Router/Query, IndexedDB + WebCrypto (AES-GCM), `bcryptjs` (offline PIN check), `@zxing/library` (camera barcodes; already installed), Playwright.

**Spec:** `docs/specs/2026-10-29-m8e-pocket-cashier-design.md`. **Look:** `docs/design/mockups/pocket-cashier.html` (approved phone mockups: sign in, sell, pay, PC down; Studio / Night / Bazaar, light and dark).

## Global Constraints

- Release 1 only: online and offline selling, Pay, Held, session open and close, withdraw and deposit, online printing at the counter, pairing and blocking, one person one device, certificates and the setup page, replay with flags, and the review screen. **No Counter tab and no plugin menus.** The bottom tabs are Sell / Held / Menu (no greyed "coming later" tab: the user rejects placeholders).
- Core stays plugin-inert: no `pocket` literal, import or comment outside `plugins/pocket` and `web/src/plugins/pocket` (`make check-core`).
- Plugin key `pocket`, name "Pocket Cashier app", price **5,000 LKR**, included in **Everything, which stays 95,000**.
- Only **cashier and manager** PIN hashes go into the copy, never admin or system. Admin and system sign in on a phone only online, and an admin then gets the normal admin panel.
- **A phone sale never pops the drawer.** Phone session opens and closes don't either.
- **Never refuse an offline action after the fact:** apply it and flag it. An action that still can't be applied is stored as `failed` with its error, acknowledged to the phone (never resent) and shown on the review screen.
- **No printing from the phone.** Online, the sale prints through the core `POST /api/v1/till/sales/:id/print` and follows ask-to-print, with the question as a dialog, never a toast. Offline: no paper.
- The phone's HTTPS port is **8443**, and the CA lives in the database, so backups and restores carry it.
- Money only through `cashflow.Move/MoveTx` (via the core services). Files ≤ 500 lines. Theme tokens only. Every screen works with every palette in light and dark. The phone screens are checked at 390 px.
- No Claude/AI attribution in commits. One commit per task step marked Commit, on `main`.
- New core migrations: `00016_`, `00017_`, `00018_`. Run `make types` after any `dto*.go` change.
- Docs and legal move with the product (Task 26). docscheck learns `"kind": "app"`.

## Review Focus

1. **The same send arrives twice at once** (a retry while the first is still applying, or two tabs). Expect each ref applied exactly once. Pinned in Task 13 (`TestReplayConcurrentSendsApplyOnce`).
2. **A phone with a wrong clock** (rung time in the future, or before the session it claims). Expect the sale to count at arrival with a `clock` flag, never in a closed day. Pinned in Task 13 (`TestReplayBadClockFlagsAndCountsNow`).
3. **The server was restored from an older backup**, so the phone sells a product that no longer exists. Expect the action stored `failed` with the reason, acknowledged and on the review screen, with later actions in the batch still applied. Pinned in Task 13 (`TestReplayMissingProductFailsButBatchGoesOn`).
4. **A cashier deactivated after the copy was taken** sells offline. Expect the sale applied under them and flagged `inactive_user`. Pinned in Task 13 (`TestReplayInactiveUserFlagged`).
5. **The PC's IP changes while a phone is in use.** Expect the next TLS handshake to get a fresh certificate for the new IP, signed by the same CA. Pinned in Task 10 (`TestLeafFollowsNewIP`).

---

## Phase A — core seams

### Task 1: Replayable sales and sale flags

**Files:**
- Create: `migrations/00016_sale_replay.sql`, `internal/features/sales/flags.go`, `internal/features/sales/replay_db_test.go`
- Modify: `internal/features/sales/service_types.go`, `internal/features/sales/service.go`, `internal/features/sales/service_lines.go`, `internal/features/sales/sales.go` (InsertSale)

**Interfaces:**
- Produces:
  - Migration: `sales.client_ref text UNIQUE NULL`, `sales.rung_at timestamptz NULL`; table `sale_flags(id bigserial pk, sale_id bigint fk sales on delete cascade, kind text not null, note text not null default '', created_at timestamptz default now())`, indexed on `sale_id`.
  - `CreateInput` gains `ClientRef string` (one-time id; a repeat returns the first sale), `RungAt *time.Time` (stored as `rung_at`), `At *time.Time` (`created_at`; nil = now) and `Review bool` (never refuse: an oversell is taken as found-at-till, over-limit credit is allowed, and an `ExpectPrice` that differs is kept; each writes a flag).
  - `ItemInput.ExpectPrice string`: the unit price the client showed ("" = none).
  - `Detail.Duplicate bool`: true when `ClientRef` matched an existing sale.
  - In `flags.go`: kind constants `FlagPrice = "price"`, `FlagOversold = "oversold"`, `FlagOverLimit = "over_limit"`, `FlagLookalike = "lookalike"`, `FlagLate = "late"`, `FlagClock = "clock"`, `FlagInactiveUser = "inactive_user"`; `type Flag struct{ ID, SaleID int64; Kind, Note string; CreatedAt time.Time }` (db + json tags); `func AddFlag(ctx context.Context, q sqlx.ExtContext, saleID int64, kind, note string) error`; `func (s *Service) Flags(ctx context.Context, saleID int64) ([]Flag, error)`.

- [ ] **Step 1: Write the failing DB tests** in `replay_db_test.go` (reuse the fixtures other `*_db_test.go` in the package use):
  - `TestCreateClientRefOnce`: two `Create` calls with `ClientRef: "r1"` → the same sale id, `second.Duplicate == true`, one row in `sales`, stock taken once.
  - `TestCreateAtAndRungAt`: `At` and `RungAt` two hours ago → `created_at` and `rung_at` equal that time.
  - `TestReviewKeepsExpectedPrice`: product at 120, `ExpectPrice: "100"`, `Review: true` → line `unit_price` 100, one `price` flag whose note contains `100` and `120`. Without `Review`, `ExpectPrice` is ignored.
  - `TestReviewOversellFlagged`: stock 1, qty 3, `Review: true`, `AllowOversell: false` → the sale is saved, on-hand is 0 and there is an `oversold` flag.
  - `TestReviewOverLimitFlagged`: customer limit 100, 300 on account, `Review: true` → status `credit` and an `over_limit` flag.
- [ ] **Step 2: Run them to see them fail.** `DATABASE_URL=… go test ./internal/features/sales -run 'TestCreateClientRef|TestCreateAt|TestReview' -v`. Expected: FAIL (unknown fields).
- [ ] **Step 3: Implement.**
  - At the top of `Create`, if `ClientRef != ""` and a sale has it, return `loadDetail` with `Duplicate = true`. A unique violation on insert (a concurrent twin) rolls back and returns the existing sale the same way.
  - `Review` forces `AllowOversell` per line and `AllowOverLimit`, and records which of them actually mattered so the flags are written after `InsertSale`.
  - `ExpectPrice` replaces `unitPrice` before packs and discounts when `Review` is set and it differs.
  - `InsertSale` writes `client_ref`, `rung_at` and `created_at` (`COALESCE($n, now())`).
- [ ] **Step 4: Run them to see them pass.** Run the same command, then `go test ./internal/features/sales/...`. Expected: PASS, with the existing sale tests unchanged.
- [ ] **Step 5: Commit.** `feat(sales): replayable sales — one-time ref, rung time, review flags`

### Task 2: Sale flags in the admin

**Files:**
- Modify: `internal/api/dto_sale.go` (sale detail DTO), `internal/api/admin_sales.go`, `web/src/features/sales/SaleSheet.tsx`, `web/src/features/sales/SalesPage.tsx` + `web/src/routes/_authed/admin/sales.tsx` (search param `sale`)
- Test: `internal/api/admin_sales_test.go`, `web/src/features/sales/SaleSheet.test.tsx` (create)

**Interfaces:**
- Consumes: `sales.Service.Flags`, `sales.Flag` (Task 1).
- Produces: the admin sale detail DTO gains `flags: SaleFlag[]` (`{kind, note, createdAt}`), `rungAt?: string` and `tillSource: string`. `tillSource` is the money-source value of the open till session that took the sale, or `""` when that session is closed. The admin sales page opens a sale's sheet from `?sale=<id>`. The sheet's return form preselects `tillSource` as the refund source.

- [ ] **Step 1: Write the failing tests.**
  - Go `TestAdminSaleDetailFlags`: a sale with an `oversold` flag and an open till → detail JSON has `flags[0].kind == "oversold"` and a non-empty `tillSource`. After closing the till → `tillSource == ""`.
  - Vitest `SaleSheet shows flags`: render with two flags → the text "Needs review" plus both notes. The return form's source select starts at `tillSource`.
- [ ] **Step 2: Run them to see them fail.** `go test ./internal/api -run TestAdminSaleDetailFlags` and `cd web && bunx vitest run src/features/sales/SaleSheet.test.tsx`
- [ ] **Step 3: Implement.** Add the DTO fields, then `make types`. The sheet shows an amber "Needs review" block (the `--warning` token family, as the till's stale-session warning uses it), one line per flag with its time, plus "Rung at HH:MM" when `rungAt` differs from the sale time by more than a minute.
- [ ] **Step 4: Run them to see them pass.**
- [ ] **Step 5: Commit.** `feat(sales): admin sale sheet shows review flags, rung time, return from the till that took it`

### Task 3: Till sessions at a given time

**Files:**
- Create: `migrations/00017_till_review_note.sql` (`cash_register.review_note text NULL`), `internal/features/cashregister/at_db_test.go`
- Modify: `internal/features/cashregister/cashregister.go`, `internal/features/cashregister/repository.go`, `internal/api/admin_tills.go` + its DTO (show `reviewNote`), the admin tills list UI row (`web/src/features/money/…` where the tills list renders; a badge)

**Interfaces:**
- Produces:
  - `OpenInput` gains `At *time.Time` (`opened_at`; nil = now), `Remote bool` (not at the counter: no drawer pulse) and `ReviewNote string` (stored, shown on the admin tills list).
  - `CloseInput` gains `At *time.Time` (`closed_at`; nil = now) and `Remote bool`.
  - `Session.ReviewNote *string`.
  - Rules: an open `At` after now + 5 min, or not after the user's last `closed_at`, is a validation error. A close `At` before `opened_at`, or after now + 5 min, is a validation error.

- [ ] **Step 1: Write the failing DB tests.**
  - `TestOpenAtPast`: open with `At` = 1 h ago → `opened_at` is that time, and a cash sale created with `sales.CreateInput.At` 30 min ago counts in `Summary.CashSales`.
  - `TestOpenAtBeforeLastClose`: refused.
  - `TestCloseAt`: open, sale, close with `At` = 5 min ago → `closed_at` is that time and `expected_cash` includes the sale.
  - `TestRemoteNoKick`: with `WithDrawerKick` counting calls, `Remote: true` open and close → 0 kicks.
  - `TestReviewNoteStored`.
- [ ] **Step 2: Run them to see them fail.** `go test ./internal/features/cashregister -run 'TestOpenAt|TestCloseAt|TestRemote|TestReviewNote' -v`
- [ ] **Step 3: Implement.** `repo.Open` and `repo.Close` take the times (`COALESCE`). The checks run inside the tx, against `LastClosed` and `FindOpenForUpdate`. `Remote` skips `s.kick`.
- [ ] **Step 4: Run them to see them pass,** plus `go test ./internal/features/cashregister/... ./internal/api -run Till`.
- [ ] **Step 5: Commit.** `feat(tills): open and close at a given time, remote (no drawer), review note`

### Task 4: One person, one device (server)

**Files:**
- Create: `migrations/00018_active_device.sql` (`users.active_device text`, `users.active_device_label text`, `users.active_device_at timestamptz`), `internal/plugin/hooks_device.go`, `internal/api/device.go`, `internal/api/device_test.go`
- Modify: `internal/features/auth/service.go` + `repository.go`, `internal/middleware/auth.go` (UserFlags), `internal/middleware/user_validator.go`, `internal/middleware/flags.go` (if UserFlags lives there), `internal/api/api.go` (`h.protected`), `internal/api/auth.go` (Login, Unlock), `internal/api/presence.go` (reply), `internal/api/me.go`, `internal/api/till_sales.go` (hold route), `internal/api/dto.go`

**Interfaces:**
- Produces:
  - Plugin seams: `func (r *Registry) OneDevicePerPerson()`; `func OneDevice() bool`; `type TakeoverGuard func(ctx context.Context, userID int64, activeDevice string) (block bool, reason string)`; `func (r *Registry) AddTakeoverGuard(g TakeoverGuard)`; `func TakeoverGuards() []TakeoverGuard`. Off unless a plugin asks, so shops without the app behave exactly as today.
  - `auth.Service`: `ClaimDevice(ctx, userID int64, device, label string) error`, `ReleaseDevice(ctx, userID int64) error`.
  - `middleware.UserFlags` gains `ActiveDevice, ActiveLabel string` (read by `DBUserValidator`).
  - api: middleware `h.onDevice` (appended to `h.protected`). When `plugin.OneDevice()` is on, the user isn't system, `ActiveDevice != ""` and `device(c) != ActiveDevice`, it answers 403 `SESSION_LOCKED` "<Name> is now on <label>".
  - Login and Unlock (when on) first run the takeover guards (a block answers 409 `DEVICE_HELD` with the guard's reason), then claim `device(c)`. The label is `LoginRequest.deviceLabel` when given, else presence's UA label.
  - `Me.movedTo string` ("" unless locked out by another device). `PresenceReply.moved bool`.
  - `POST /api/v1/till/held` runs without `h.onDevice`, so a moved PC can still park its cart.

- [ ] **Step 1: Write the failing API tests** in `device_test.go`, with a test-only toggle `plugin.SetOneDeviceForTest(bool)` (`plugin/only_test.go` shows the pattern):
  - `TestOneDeviceOffByDefault`: the same user signs in on devices A and B, and both can call `/till/config`.
  - `TestTakeover`: on. A signs in, then B signs in → A's `/till/config` gets 403 `SESSION_LOCKED` whose message contains B's label. A's `/me` has `movedTo`. A's presence beat replies `moved: true`. A's `POST /till/held` still works.
  - `TestUnlockTakesBack`: A unlocks with the PIN → B is now locked.
  - `TestSystemExempt`: the system user is never locked.
  - `TestTakeoverGuardBlocks`: a guard returning `(true, "on Phone 2")` → B's login gets 409 `DEVICE_HELD` with that reason, and A keeps working.
- [ ] **Step 2: Run them to see them fail.** `go test ./internal/api -run 'TestOneDevice|TestTakeover|TestUnlockTakesBack|TestSystemExempt' -v`
- [ ] **Step 3: Implement** as in the Interfaces above. Only plugins call `ReleaseDevice` in release 1; there is no core button for it.
- [ ] **Step 4: Run them to see them pass,** plus `go test ./internal/...`.
- [ ] **Step 5: Commit.** `feat(auth): one person, one device — server-side active device, takeover guard seam`

### Task 5: One person, one device (SPA)

**Files:**
- Modify: `web/src/routes/lock.tsx` (or its `features/auth` component), the presence beat sender (`web/src/lib/presence*.ts` or wherever `POST /presence` is sent), `web/src/features/till/TillProvider.tsx`, `web/src/features/auth/LoginHero.tsx` (or the login form: show `DEVICE_HELD`)
- Test: `web/src/features/till/moved.test.ts` (create)

**Interfaces:**
- Consumes: `Me.movedTo`, `PresenceReply.moved`, the `DEVICE_HELD` code (Task 4).
- Produces: `holdOnMove(cart): Promise<void>` in `web/src/features/till/cart/held.ts` (pure apart from the API call). When the cart has lines it holds it with the label "Moved to another device", then the app goes to `/lock`.

- [ ] **Step 1: Write the failing vitest.** `moved.test.ts`: with the API mocked, `holdOnMove` on a 2-line cart → posts `/till/held` once with the label and the cart JSON. On an empty cart it posts nothing.
- [ ] **Step 2: Run it to see it fail.** `cd web && bunx vitest run src/features/till/moved.test.ts`
- [ ] **Step 3: Implement.** The presence sender reads `moved` → TillProvider's `holdOnMove` → clear the cart → navigate to `/lock`. The lock page shows `me.movedTo` ("Nimal is now on Phone 2. Sign in here to take it back.") above the PIN pad. The login form shows a `DEVICE_HELD` message in place, not as a toast.
- [ ] **Step 4: Run it to see it pass,** plus `make test-web`.
- [ ] **Step 5: Commit.** `feat(till): a moved PC holds its cart and says where the person went`

### Task 6: Plugin listeners

**Files:**
- Create: `internal/plugin/listen.go`, `internal/app/listeners.go`, `internal/app/listeners_test.go`
- Modify: `cmd/server/main.go` (start after `e.Start`, shut down with it)

**Interfaces:**
- Produces: `type Listener struct{ Key string; Addr string; TLS *tls.Config }`; `func (r *Registry) AddListener(l Listener)`; `func Listeners() []Listener`; `func ServeListeners(e *echo.Echo, ls []plugin.Listener) (shutdown func(context.Context) error)` in `internal/app`. Each listener is an `http.Server{Addr, Handler: e, TLSConfig}` running `ServeTLS("", "")`, with a bind error logged, never fatal: the till must still start.

- [ ] **Step 1: Write the failing test.** `TestServeListenersTLS`: a self-signed `tls.Config` on `127.0.0.1:0` (pick a free port first), with an echo that has `/health` → an HTTPS GET with that root trusted returns 200. After `shutdown` the port is closed.
- [ ] **Step 2: Run it to see it fail.** `go test ./internal/app -run TestServeListenersTLS -v`
- [ ] **Step 3: Implement,** and call it from `cmd/server/main.go`.
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(plugin): plugins may serve the app on an extra HTTPS listener`

### Task 7: The till copy

**Files:**
- Create: `internal/api/dto_copy.go`, `internal/api/till_copy.go`, `internal/api/till_copy_test.go`
- Modify: `internal/plugin/core.go`, `internal/app/plugins.go`, `internal/app/services.go` (expose heldsales if it's built in api only)

**Interfaces:**
- Produces (a generic "everything a till needs to sell offline" copy):
  - `type TillCopy struct` with JSON fields:
    - `takenAt`, `version`
    - `shop` (`ShopHeader`), `symbol`, `defaultSaleType`, `askToPrint`, `allowUntrackedCash`, `defaultLockerId`, `tenderMethods []TenderMethod`
    - `products []TillProduct` (active, with packs and stock, as `/till/products` builds them), `groups []TillGroup`
    - `customers []CopyCustomer{id,name,phone,balance,creditLimit}`
    - `people []CopyPerson{id,name,phone,role,pinHash,canManageCredit,canSeeCost}` (**cashier and manager only, active only**)
    - `places []CopyPlace{id,name,kind}` (lockers a till may open from or close into, and card and bank accounts for card tenders)
    - `sessions []CopySession{id,userId,openedAt,openingCash,expected}` (open ones)
    - `held []CopyHeld{id,userId,label,saleType,customerId,cart,itemCount,total,createdAt}`
  - `func BuildTillCopy(ctx context.Context, db *sqlx.DB) (*TillCopy, error)` and `func TillCopyVersion(ctx context.Context, db *sqlx.DB) (string, error)`. The version is a cheap fingerprint: max ids or updated times of products, stock_movements, customers, cash_register, held_sales and users, joined.
  - `plugin.Core` gains `TillCopy func(ctx context.Context) (any, error)`, `TillCopyVersion func(ctx context.Context) (string, error)`, `Auth *auth.Service` and `Held *heldsales.Service`.

- [ ] **Step 1: Write the failing test.** `TestTillCopy` with a cashier, a manager, an admin, the system user, a product with a pack and stock 5, a customer with balance 50 and limit 500, an open session and one held sale → `people` holds exactly the cashier and the manager, each with a non-empty `pinHash`. The product has `packs` and `stockQty "5.000"` (match the existing format), and the session's `expected` equals its opening cash. `TestTillCopyVersionChanges`: the version changes after a sale and after a customer edit.
- [ ] **Step 2: Run them to see them fail.** `go test ./internal/api -run TestTillCopy -v`
- [ ] **Step 3: Implement.** Reuse the till catalog's product query and `withPacks`, so the copy prices exactly as the till does. Then `make types` and wire `Core` in `startPlugins`.
- [ ] **Step 4: Run them to see them pass.**
- [ ] **Step 5: Commit.** `feat(till): the till copy — a till's whole selling world in one read, for offline clients`

### Task 8: Plugin surfaces and the offline flag

**Files:**
- Create: `web/src/routes/x.$key.$.tsx`, `web/src/routes/x.surface.test.tsx`
- Modify: `web/src/plugins/slots.ts`

**Interfaces:**
- Produces:
  - `export type Surface = { key: string; Surface: ComponentType<{ path: string }> }` and `slots.surfaces: [] as Surface[]`: a full-screen plugin page at `/x/<key>/*`, outside the signed-in shells, that handles its own sign-in (no `_authed` guard, no dock).
  - `TillTender` gains `offline?: boolean`: works from a copy (absent = needs the server).

- [ ] **Step 1: Write the failing test.** Render the router at `/x/demo/a/b` with `slots.surfaces = [{ key: 'demo', Surface: ({path}) => <p>{path}</p> }]` → it shows `a/b`, and no `/me` request is made. An unknown key → not found.
- [ ] **Step 2: Run it to see it fail.** `cd web && bunx vitest run src/routes/x.surface.test.tsx`
- [ ] **Step 3: Implement.** The route uses `notFound()` like `admin/x.$key.$.tsx`. Then update the slot list in `AGENTS.md` and `docs/guide/architecture.md` (one word each).
- [ ] **Step 4: Run it to see it pass,** plus `make test-web` and `make check-core`.
- [ ] **Step 5: Commit.** `feat(plugins): full-screen plugin surfaces at /x/<key>, tenders may say they work offline`

## Phase B — the plugin, Go half

### Task 9: Plugin skeleton and tables

**Files:**
- Create: `plugins/pocket/plugin.json`, `plugins/pocket/pocket.go`, `plugins/pocket/store.go`, `plugins/pocket/migrations/embed.go`, `plugins/pocket/migrations/0001_pocket.sql`, `plugins/pocket/store_db_test.go`, `web/src/plugins/pocket/index.tsx`

**Interfaces:**
- Produces:
  - `plugin.json`: `{"key":"pocket","name":"Pocket Cashier app","import":"karots-suite/plugins/pocket","version":"1.0.0","kind":"app","ports":[8443],"description":"…phone cashier, live and offline…"}`.
  - Tables:
    - `pocket_ca(id int pk check (id=1), cert_pem text, key_pem text, created_at)`
    - `pocket_phones(id bigserial, name text, device text unique, token_hash bytea unique, status text check in ('pending','active','blocked'), pair_code text, user_id bigint null, last_seen timestamptz, copy_at timestamptz, created_at, approved_at, approved_by bigint)`
    - `pocket_sends(id bigserial, phone_id fk, received_at, actions int, failed int, flagged int)`
    - `pocket_actions(ref text pk, phone_id fk, send_id fk, kind text, user_id bigint, at timestamptz, body jsonb, status text check in ('applied','skipped','failed'), sale_id bigint null, note text, applied_at)`
  - `Store` with `NewStore(db *sqlx.DB) *Store`. `Setup` registers the DataGroup (`pocket_phones`, `pocket_sends`, `pocket_actions`, `pocket_ca`) and calls `reg.OneDevicePerPerson()`. `const Port = 8443`.

- [ ] **Step 1: Write the failing test.** `TestMigrationsApply`: migrate core and pocket on the test DB, then insert and read one phone row.
- [ ] **Step 2: Run it to see it fail.** `make plugins P=pocket && DATABASE_URL=… go test ./plugins/pocket -run TestMigrationsApply -v`
- [ ] **Step 3: Implement.** The React half `export default function pocket(s)` registers nothing yet. Leave `make core-only` to put the lists back before the commit.
- [ ] **Step 4: Run it to see it pass,** plus `make core-only && make check-core`.
- [ ] **Step 5: Commit.** `feat(pocket): plugin skeleton and tables`

### Task 10: Certificates and the phone listener

**Files:**
- Create: `plugins/pocket/certs.go`, `plugins/pocket/certs_test.go`, `plugins/pocket/listener.go`

**Interfaces:**
- Consumes: `plugin.Listener` (Task 6).
- Produces:
  - `func (s *Store) CA(ctx) (*x509.Certificate, *ecdsa.PrivateKey, error)`: load row 1, or make and save it (from the spike's `loadOrMakeCA`, with the subject "Karots Shop CA — <shop name>").
  - `func makeLeaf(ca *x509.Certificate, key *ecdsa.PrivateKey, ips []net.IP) (tls.Certificate, error)`; `func lanIPs() []net.IP` (non-loopback IPv4 plus 127.0.0.1, without docker / virbr bridges); `func mobileconfig(der []byte, shop string) string`.
  - `type leafCache` with `GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error)`, which re-issues when `lanIPs()` (checked at most every 30 s) differs from the IPs in the cached leaf.
  - Setup adds `plugin.Listener{Key: "pocket", Addr: ":8443", TLS: &tls.Config{GetCertificate: cache.GetCertificate}}`.
  - VPS or Tailscale shops: when `.env` sets `POCKET_CERT_FILE` and `POCKET_KEY_FILE` (a public certificate), the listener serves that pair instead (`tls.LoadX509KeyPair`, re-read when the files change, for renewals). The setup page then skips the CA steps.
  - Copy `loadOrMakeCA`, `makeLeaf`, `mobileconfig`, `serial` and `lanIPs` from `~/Projects/go/karots-pwaspike/main.go`, swapping the files for the table. **The spike folder can be deleted once this task is committed.**

- [ ] **Step 1: Write the failing tests.**
  - `TestCAStoredOnce`: two `CA` calls → the same serial, and one row.
  - `TestLeafVerifies`: a leaf for 192.168.1.20 verifies against the CA pool with the IP SAN.
  - `TestLeafFollowsNewIP`: a cache with a stubbed `lanIPs` returning A, then B (after the check interval) → the second handshake's cert contains B, from the same CA.
  - `TestMobileconfig`: it contains `com.apple.security.root`, the base64 DER and the shop name.
  - `TestPublicCertWins`: with the two env files set to a test pair → the handshake serves that cert, not a CA leaf.
- [ ] **Step 2: Run them to see them fail.** `go test ./plugins/pocket -run 'TestCA|TestLeaf|TestMobileconfig' -v`
- [ ] **Step 3: Implement** (with `lanIPs` as a package var so the test can stub it).
- [ ] **Step 4: Run them to see them pass.**
- [ ] **Step 5: Commit.** `feat(pocket): the shop's own CA and a phone certificate that follows the PC's IP`

### Task 11: Pairing and the phones API

**Files:**
- Create: `plugins/pocket/pair.go`, `plugins/pocket/api_phones.go`, `plugins/pocket/pair_db_test.go`

**Interfaces:**
- Produces:
  - Public:
    - `POST /api/v1/x/pocket/pair {name, device, secret}` → `{code}` (6 digits; pending row with `token_hash = sha256(secret)`; a repeat for the same device replaces its pending row).
    - `GET /api/v1/x/pocket/pair/status` with header `X-Ks-Phone: <secret>` → `{status, name}`.
    - `GET /api/v1/x/pocket/ca.crt | ca.pem | ca.mobileconfig` (the right content types).
    - `GET /api/v1/x/pocket/ping` (CORS `*`; 204).
  - Middleware `phoneAuth`: `X-Ks-Phone` → an active phone, or 401 `PHONE_UNKNOWN` / 403 `PHONE_BLOCKED`. It sets `last_seen` (at most once per 10 s) and stores the phone in the context: `phoneFrom(c) *Phone`.
  - Admin (`reg.Admin()`, admin only via `RequireRole("admin")`):
    - `GET /pocket/phones` → `[{id,name,status,userName,copyAt,lastSeen,pairCode}]`
    - `POST /pocket/phones/approve {code}` and `POST /pocket/phones/:id/approve`
    - `POST /pocket/phones/:id/block`, `POST /pocket/phones/:id/release` (`Core.Auth.ReleaseDevice` for the phone's `user_id`, which is then cleared)
    - `PUT /pocket/phones/:id {name}`
    - `GET /pocket/addresses` → `{setup: ["http://<ip>:<port>/x/pocket/setup", …], app: ["https://<ip>:8443/x/pocket/", …]}`
  - Every admin action is audited (`Core.Audit`).

- [ ] **Step 1: Write the failing tests.**
  - `TestPairApproveUse`: pair → pending, so the status call says `pending` and `phoneAuth` rejects it. Approve by code → `active`, and `phoneAuth` accepts.
  - `TestBlockStopsAtOnce`: block → the next `phoneAuth` call gets 403.
  - `TestReleaseClearsDevice`: a user with `active_device` = the phone's device → release clears it.
  - `TestCashierCannotApprove`: a manager or cashier gets 403.
- [ ] **Step 2: Run them to see them fail.** `go test ./plugins/pocket -run 'TestPair|TestBlock|TestRelease|TestCashierCannot' -v`
- [ ] **Step 3: Implement.** Compare secrets in constant time by hash lookup, and never store the raw secret.
- [ ] **Step 4: Run them to see them pass.**
- [ ] **Step 5: Commit.** `feat(pocket): pair phones with admin approval, block and release`

### Task 12: Copy and live feed

**Files:**
- Create: `plugins/pocket/copy.go`, `plugins/pocket/feed.go`, `plugins/pocket/copy_db_test.go`

**Interfaces:**
- Consumes: `Core.TillCopy`, `Core.TillCopyVersion` (Task 7), `phoneAuth` (Task 11).
- Produces:
  - `GET /api/v1/x/pocket/copy` (`phoneAuth`) → the copy, gzipped, with `ETag: <version>`; `If-None-Match` → 304. It sets `copy_at`.
  - `GET /api/v1/x/pocket/feed?t=<secret>` (EventSource can't send headers, and the query is safe on the TLS listener): an SSE stream with `event: version` whenever the version changes (one shared 2 s poller for all subscribers, started on the first subscriber), plus a `: ping` every 25 s.

- [ ] **Step 1: Write the failing tests.**
  - `TestCopyNeedsActivePhone`: 401 without a phone. 200 with an active phone, and the body has `people` and no admin.
  - `TestCopyETag`: a second call with `If-None-Match` → 304, and after a sale → 200.
  - `TestFeedSendsVersion`: open the feed (httptest), make a sale → a `version` event arrives within 5 s.
- [ ] **Step 2: Run them to see them fail.** `go test ./plugins/pocket -run 'TestCopy|TestFeed' -v`
- [ ] **Step 3: Implement.** Mark the shared poller `// ponytail: polls a fingerprint every 2 s; a core change bus if many phones or big shops need it`.
- [ ] **Step 4: Run them to see them pass.**
- [ ] **Step 5: Commit.** `feat(pocket): phone copy with ETag and a live change feed`

### Task 13: Replay

**Files:**
- Create: `plugins/pocket/replay.go` (endpoint, ordering, refs, statuses), `plugins/pocket/replay_sale.go` (the sale and session rules, look-alikes), `plugins/pocket/replay_db_test.go`, `plugins/pocket/replay_rules_db_test.go`

**Interfaces:**
- Consumes: `sales.CreateInput{ClientRef, RungAt, At, Review}`, `ItemInput.ExpectPrice`, `sales.AddFlag` and the flag kinds (Task 1); `cashregister.OpenInput{At, Remote, ReviewNote}`, `CloseInput{At, Remote}` (Task 3); `Core.Held`, `Core.CashRegister`, `Core.Sales`, `Core.Auth.ClaimDevice`.
- Produces:
  - `POST /api/v1/x/pocket/actions` (`phoneAuth`), request `{signedIn: userId|null, actions: [{ref, kind, userId, at, body}]}`, oldest first. Kinds:
    - `open` (`{openingCash, breakdown, sourceLockerId}`)
    - `sale` (the core `SaleRequest` shape plus `expectPrice` per item)
    - `hold` (the `HoldRequest` shape) and `unhold` (`{heldId}`)
    - `withdraw` / `deposit` (the `MovementInput` shape)
    - `close` (`{closingCash, breakdown, destLockerId}`)
  - Response: `{results: [{ref, status, saleId?, receiptNo?, heldId?, note?}]}`.
  - Each batch takes `pg_advisory_xact_lock` on the phone id while it records refs, so concurrent sends serialise. A ref already in `pocket_actions` → its stored result, unchanged.
  - Rules (`replay_sale.go`):
    - **open:** the user already has an open session → `skipped`, "already open on <time>". Otherwise `Open{At: at, Remote: true}`.
    - **sale:** `at` after now + 5 min, or before the user's last `closed_at` → treat `at` as now and add a `clock` flag. Then, if the user's open session has `opened_at <= at` → `At = at`. Otherwise, with no open session, open one now (`Remote`, opening 0, `ReviewNote: "opened for late sales from <phone>"`), set `At = nil` and flag `late`. Always `ClientRef = ref`, `RungAt = at`, `Review = true` and `DeferDrawerKick = true`. Inactive user → still applied, flagged `inactive_user`.
    - **Look-alike:** after the sale, any other sale in ±10 min with the same total and the same multiset of (product, qty) → flag `lookalike` on the replayed one, with a note naming the other receipt.
    - **close:** `Close{At: at, Remote: true}`, where no open session → `skipped`.
    - **withdraw / deposit:** the core movement with `DeferDrawerKick`. No open session → `failed`.
    - **hold / unhold:** the core held sales.
    - Any error → `failed` with the message, and the batch goes on.
  - `signedIn` → `ClaimDevice(signedIn, phone.device, phone.name)` and `pocket_phones.user_id`. Admin and system user ids in actions → `failed` ("admins sell from a cashier account on phones").
  - `pocket_sends` gets one row per batch with the counts.

- [ ] **Step 1: Write the failing tests.**
  - `replay_db_test.go`:
    - `TestReplayRefOnce`: the same batch twice → one sale and an identical second response.
    - `TestReplayConcurrentSendsApplyOnce`: two goroutines sending the same 3-sale batch → 3 sales.
    - `TestReplayOrder`: open → sale → sale → close, all offline times → session `opened_at`/`closed_at` = the phone times, `expected_cash` = float + both cash sales, and no flags.
    - `TestReplayMissingProductFailsButBatchGoesOn`: sale (deleted product) then a good sale → `failed` + `applied`, and the send row has `failed=1`.
  - `replay_rules_db_test.go`:
    - `TestReplayLateSaleAfterClose`: the session was closed on the PC → the sale lands in a new session opened now with a review note, `created_at` ≈ now, `rung_at` = the phone time and a `late` flag. The closed session's `expected_cash` is unchanged.
    - `TestReplayBadClockFlagsAndCountsNow`: `at` = tomorrow → `created_at` ≈ now, and a `clock` flag.
    - `TestReplayPriceChangedKept`, `TestReplayOversoldFlagged`, `TestReplayOverLimitFlagged`: the flags from Task 1, through the endpoint.
    - `TestReplayLookalike`: a PC sale of 2×Tea at 10:00:30, then a phone sale of 2×Tea rung at 10:02 → the phone one is flagged `lookalike`, naming the PC receipt.
    - `TestReplayInactiveUserFlagged`.
    - `TestReplayAdminRefused`.
    - `TestReplaySignedInClaimsDevice`.
    - `TestReplayNoDrawerKick` (with a counting kick).
- [ ] **Step 2: Run them to see them fail.** `go test ./plugins/pocket -run TestReplay -v`
- [ ] **Step 3: Implement.** Keep `replay.go` and `replay_sale.go` each under 500 lines; split the look-alike out into `lookalike.go` if needed.
- [ ] **Step 4: Run them to see them pass,** plus `go test ./plugins/pocket/...`.
- [ ] **Step 5: Commit.** `feat(pocket): replay phone actions in order — once each, flagged never refused`

### Task 14: Review API and the takeover guard

**Files:**
- Create: `plugins/pocket/review.go`, `plugins/pocket/review_db_test.go`, `plugins/pocket/guard.go`

**Interfaces:**
- Consumes: `plugin.AddTakeoverGuard` (Task 4), tables (Task 9).
- Produces:
  - Admin `GET /pocket/sends?limit=` → `[{id, phone, receivedAt, actions, failed, flagged}]`.
  - `GET /pocket/sends/:id` → `{actions: [{ref, kind, userName, at, status, note, saleId, receiptNo, flags: [{kind, note}]}]}`, where flags come from `sale_flags`.
  - Takeover guard: block when the user's `active_device` is a pocket phone whose `last_seen` is more than 60 s ago, with the reason "<Name> is signed in on <phone>, which isn't reachable. An admin can release it in Phones."

- [ ] **Step 1: Write the failing tests.**
  - `TestSendsList`: two batches → listed newest first with their counts.
  - `TestSendDetailFlags`: a look-alike sale → `flags[0].kind == "lookalike"`.
  - `TestGuardBlocksUnreachablePhone`: `last_seen` 2 min ago → PC login gets 409 `DEVICE_HELD`. 10 s ago → allowed (takes it back).
- [ ] **Step 2: Run them to see them fail.** `go test ./plugins/pocket -run 'TestSends|TestSendDetail|TestGuard' -v`
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run them to see them pass.**
- [ ] **Step 5: Commit.** `feat(pocket): what came in from each phone, and keep a person off the PC while their phone is out of reach`

### Task 15: Service worker and manifest

**Files:**
- Create: `plugins/pocket/sw.go`, `plugins/pocket/sw.js.tmpl` (embedded), `plugins/pocket/sw_test.go`

**Interfaces:**
- Produces:
  - Public `GET /api/v1/x/pocket/sw.js`, with `Service-Worker-Allowed: /` and `Cache-Control: no-cache`. It is the template filled with the version (`Core.Cfg.Version`) and the list of every file in `spa.Dist()` (walk the embedded FS).
  - SW behaviour:
    - install precaches every listed file plus `/x/pocket/`
    - a navigation is network-first, falling back to the cached `/index.html`
    - `/assets/*` is cache-first
    - `/api/*` is never cached
    - activate deletes old version caches
  - Public `GET /api/v1/x/pocket/manifest.webmanifest`: name "Karots Pocket Cashier", `start_url: "/x/pocket/"`, `scope: "/"`, `display: "standalone"`, and the icons from `web/public` (`/icon-192.png`, `/icon-512.png`, `/icon-maskable-512.png`).

- [ ] **Step 1: Write the failing test.** `TestSWListsAssets`: the body contains `"/index.html"`, at least one `"/assets/` entry and the version string, and the response has the `Service-Worker-Allowed` header. `TestManifest`: `start_url` is `/x/pocket/`.
- [ ] **Step 2: Run it to see it fail.** `go test ./plugins/pocket -run 'TestSW|TestManifest' -v` (after `make web` so `dist` has assets)
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocket): service worker and manifest for the installed phone app`

## Phase C — the plugin, React half (`web/src/plugins/pocket/`)

Every screen follows `docs/design/mockups/pocket-cashier.html` and uses theme tokens and the admin kit or ui primitives only. The phone app reads the palette from `login-info` (cached in the copy) and the mode from `ks-mode`, like every screen.

### Task 16: Phone store (IndexedDB, encrypted)

**Files:**
- Create: `store/db.ts`, `store/crypto.ts`, `store/copy.ts`, `store/outbox.ts`, `store/store.test.ts`
- Modify: `web/package.json` (dev dependency `fake-indexeddb`)

**Interfaces:**
- Produces:
  - `openDb(): Promise<IDBDatabase>`, with database `ks-pocket` v1 and stores `keys`, `copy`, `outbox`, `phone`.
  - `getKey(): Promise<CryptoKey>`: AES-GCM 256, generated non-extractable once and kept in `keys`.
  - `seal(v: unknown): Promise<{iv: Uint8Array; data: ArrayBuffer}>`, `open<T>(s): Promise<T>`.
  - `saveCopy(c: TillCopy)`, `loadCopy(): Promise<TillCopy | null>`.
  - `type Action = { ref: string; kind: 'open'|'sale'|'hold'|'unhold'|'withdraw'|'deposit'|'close'; userId: number; at: string; body: unknown }`.
  - `addAction(a: Omit<Action,'ref'|'at'>): Promise<Action>` (ref from `crypto.randomUUID()`, `at` = now ISO).
  - `pending(): Promise<Action[]>` (oldest first), `ack(refs: string[])`.
  - `savePhone({secret, name, device})`, `loadPhone()`, where the secret is sealed too.

- [ ] **Step 1: Write the failing vitest** (`import 'fake-indexeddb/auto'`):
  - The copy round-trips.
  - The raw `copy` store value is not readable JSON, meaning it's sealed.
  - `addAction` × 3 → `pending` in order. `ack` of the first two → one left.
  - The key is non-extractable (`exportKey` rejects).
- [ ] **Step 2: Run it to see it fail.** `cd web && bunx vitest run src/plugins/pocket/store`
- [ ] **Step 3: Implement** with plain IndexedDB (no library).
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocket): the phone's encrypted copy and outbox`

### Task 17: Sync engine

**Files:**
- Create: `sync.ts`, `sync.test.ts`, `api.ts` (fetch helpers with `X-Ks-Phone` and `X-Ks-Device`)

**Interfaces:**
- Consumes: store (Task 16), the endpoints from Tasks 11–13.
- Produces:
  - `type SyncState = { live: boolean; copyAt: string | null; waiting: number; lastError?: string }`.
  - `type ActionResult = { ref: string; status: 'applied' | 'skipped' | 'failed'; saleId?: number; receiptNo?: string; heldId?: number; note?: string }` (the Task 13 response, one per action).
  - `startSync(onState: (s: SyncState) => void, signedIn: () => number | null): () => void`. On start: refresh the copy, then flush. Every 60 s it refreshes (`If-None-Match`), it opens an EventSource on `/feed` (a `version` event refreshes the copy), and a flush runs whenever `waiting > 0` and live. `live` = a call succeeded in the last 15 s. A network failure goes offline with no exception.
  - `sendNow(a: Action): Promise<ActionResult | null>` adds the action to the outbox and, when live, flushes and returns its result (null when offline).
  - Flush sends at most 50 actions per POST and acks every ref in the response (applied, skipped and failed alike).

- [ ] **Step 1: Write the failing vitest** (fetch and EventSource mocked):
  - Offline → `live false`, and the actions stay pending.
  - Back online → one POST with the actions oldest first, then acked, so `waiting 0`.
  - A `failed` result is acked too.
  - A 304 keeps the copy.
  - A `version` event triggers a copy fetch.
- [ ] **Step 2: Run it to see it fail.** `cd web && bunx vitest run src/plugins/pocket/sync.test.ts`
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocket): sync — copy refresh, live feed, outbox flush`

### Task 18: Surface, PWA hook-up, pairing and sign-in

**Files:**
- Create: `App.tsx` (surface root: path `setup` → Setup (Task 23), else the phone app), `pwa.ts`, `screens/Pair.tsx`, `screens/SignIn.tsx`, `pin.ts`, `pin.test.ts`, `session.ts` (who is signed in, kept in memory plus `sessionStorage`)
- Modify: `index.tsx` (push `slots.surfaces`), `web/package.json` (`bcryptjs` dependency; move `@zxing/library` to dependencies)

**Interfaces:**
- Consumes: `slots.surfaces` (Task 8), store, sync, the core `PinPad` (`features/auth`), core `/auth/login` (with `deviceLabel`), `/auth/lock`.
- Produces:
  - `registerPwa()`: on `location.protocol === 'https:'` only, it swaps `<link rel=manifest>` to the plugin manifest, adds the `apple-mobile-web-app-capable` and status-bar metas, and registers `/api/v1/x/pocket/sw.js` with `{ scope: '/' }`.
  - `checkPin(person: CopyPerson, pin: string): Promise<boolean>` (`bcryptjs.compare`).
  - `signIn(person, pin)`: online → core login; an admin → `location.assign('/admin')`; `DEVICE_HELD` → its reason shown in place. Offline → `checkPin`, and admins never appear offline.
  - The pairing screen: no phone saved → enter a phone name → `pair` → show the 6-digit code large, with a QR of it → poll the status every 3 s → active → save → sign-in.
  - The sign-in screen: name tiles from `copy.people`, plus "Admin" online only, then the PIN pad.

- [ ] **Step 1: Write the failing vitest.** `pin.test.ts`: a bcrypt hash of `1234` made with Go's cost (a fixture string from `bcrypt.GenerateFromPassword` at `DefaultCost`) → `checkPin` true for 1234 and false for 4321.
- [ ] **Step 2: Run it to see it fail.**
- [ ] **Step 3: Implement** the screens per the mockup ("sign in" frame).
- [ ] **Step 4: Run it to see it pass,** plus `make test-web`.
- [ ] **Step 5: Commit.** `feat(pocket): the phone app — install, pair, sign in online and offline`

### Task 19: Sell screen

**Files:**
- Create: `screens/Phone.tsx` (the shell: top bar + bottom tabs Sell / Held / Menu + the amber offline state), `screens/TopBar.tsx`, `screens/Sell.tsx`, `screens/CartBar.tsx`, `screens/Scanner.tsx`, `catalog.ts` (search and barcode lookup over the copy), `catalog.test.ts`

**Interfaces:**
- Consumes: the copy, the core cart reducer and logic in `web/src/features/till/cart/` (`store.ts`, `entry.ts`, `packs.ts`, `totals.ts`; use them as the desktop till does, never re-implement pricing).
- Produces:
  - `search(copy, text): TillProduct[]` (name, local name and barcode prefix; at most 50).
  - `byBarcode(copy, code): { product: TillProduct; packId?: number } | null` (a pack barcode → that pack).
  - Top bar: Live / "PC not reachable" pill, drawer pill, avatar menu (switch person = core `/auth/lock` online, then sign-in).
  - Offline state: amber, "Selling from the copy of HH:MM", "Waiting N", stock shown as "about N", and the copy age always visible, with a warning past 30 min.
  - `Scanner`: `@zxing/library` `BrowserMultiFormatReader` on the back camera, which closes on the first code. A Bluetooth scanner works as typing in the search box with Enter.

- [ ] **Step 1: Write the failing vitest.** `catalog.test.ts`: name search is case-insensitive and finds the local name. A pack barcode returns `packId`. An unknown code → null.
- [ ] **Step 2: Run it to see it fail.**
- [ ] **Step 3: Implement** per the mockup ("sell" frame and "PC down" frame).
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocket): phone Sell — search, chips, camera scan, cart bar, offline state`

### Task 20: Pay

**Files:**
- Create: `screens/Pay.tsx`, `screens/PayTenders.tsx`, `sale.ts` (cart → sale action body), `sale.test.ts`

**Interfaces:**
- Consumes: the cart, `sendNow` (Task 17), core `POST /till/sales/:id/print`, `copy.places` (the card account), `copy.tenderMethods` with `slots.tillTenders` (`offline` flag, Task 8).
- Produces:
  - `saleBody(cart, copy): SaleRequest & { items: { expectPrice: string }[] }`. `expectPrice` is the unit price the phone used: wholesale or retail, or the lot price.
  - Pay page:
    - "‹ Back to items" keeps the cart, and the lines have ±
    - tenders Cash / Card / Cheque / Credit, plus plugin tenders live only unless `offline`
    - cash quick amounts and change to give
    - Complete → `sendNow(sale)`
  - Online result → "Receipt prints at the counter" and the print call. With ask-to-print on, a dialog asks first. Offline → "Saved, will send when the PC is back". New sale.

- [ ] **Step 1: Write the failing vitest.**
  - `saleBody` for a retail cart with a pack line → items keep the product ids and qty, `expectPrice` = the copy's selling price, and `deferDrawerKick: true`.
  - A wholesale cart → the wholesale price.
  - Credit without a customer → the Pay page's own validation message (the core rule, mirrored).
- [ ] **Step 2: Run it to see it fail.**
- [ ] **Step 3: Implement** per the mockup ("pay" frame).
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocket): phone Pay — tenders, change, send once, print at the counter`

### Task 21: Held and Menu (the drawer)

**Files:**
- Create: `screens/Held.tsx`, `screens/Menu.tsx`, `screens/DrawerSheet.tsx`, `drawer.ts`, `drawer.test.ts`

**Interfaces:**
- Consumes: `copy.held`, `copy.sessions`, `copy.places`, the outbox and sync.
- Produces:
  - Held: list (the copy's held for this person plus local holds not yet sent), hold (action `hold`) and resume (load the cart, then action `unhold`).
  - Menu: Open session (float + source locker from `places`), Withdraw, Deposit, Close (counted cash + destination locker, with `allowUntrackedCash` respected), and Switch person.
  - `expectedCash(copy, userId, outbox): string`: the copy's `expected` plus unsent cash sales minus change, plus deposits minus withdrawals. Shown as "about" offline.

- [ ] **Step 1: Write the failing vitest.** `drawer.test.ts`: copy expected 1000, plus an unsent cash sale of 250 paid with 500 (change 250), a withdraw of 100 and a deposit of 50 → `1200.00`.
- [ ] **Step 2: Run it to see it fail.**
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocket): phone Held and Menu — open, withdraw, deposit, close, online or offline`

### Task 22: Admin Phones page

**Files:**
- Create: `admin/PhonesPage.tsx`, `admin/PhoneList.tsx`, `admin/Approve.tsx` (type the code, or scan its QR with the camera via `Scanner`), `admin/SetupCard.tsx` (QR of the setup address, from `/pocket/addresses`), `admin/Sends.tsx`, `admin/SendDetail.tsx`
- Modify: `index.tsx` (`apps` tile "Phones", `adminPages` key `pocket`)

**Interfaces:**
- Consumes: the Task 11 and Task 14 endpoints, admin kit (`PageHead`, `DataList`, `FormSheet`, `ConfirmButton`).
- Produces: `/admin/x/pocket` (phones; the pending ones on top with Approve), `/admin/x/pocket/sends` and `/admin/x/pocket/sends/<id>`. Each flagged sale links to `/admin/sales?sale=<id>` (Task 2), which is where a look-alike is removed with the admin return.

- [ ] **Step 1: Write the failing vitest.** `PhoneList`: a row with `lastSeen` 3 min ago and a user shows "Nimal · not reachable" and a Release button. A blocked row shows "Blocked" and Remove, with no Unblock (pairing again makes a new row).
- [ ] **Step 2: Run it to see it fail.**
- [ ] **Step 3: Implement.** It works at 360 px.
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocket): admin Phones — approve, block, release, setup QR, what came in`

### Task 23: Setup page (per-phone steps and the trust test)

**Files:**
- Create: `Setup.tsx`, `setupSteps.ts`, `setupSteps.test.ts`

**Interfaces:**
- Consumes: public `/pocket/ca.*`, `/pocket/addresses` is admin-only, so the setup page builds the app address from `location.hostname` and port 8443, and `https://<host>:8443/api/v1/x/pocket/ping`.
- Produces:
  - `phoneKind(ua: string): 'android' | 'iphone' | 'other'`, and `steps(kind)` returning the exact spec steps:
    - Android: download `.crt` → Settings → Security → Encryption & credentials → Install a certificate → **CA certificate** (never through Wi-Fi settings)
    - iPhone: Safari only → download the `.mobileconfig` → Settings → Profile Downloaded → Install → General → About → Certificate Trust Settings → turn on "Karots Shop CA — <shop>"
    - other: `.pem`
  - Trust test: `fetch(ping)` every 3 s. The "Open the app" link appears **only after** a ping succeeds, so the phone never reaches a certificate warning.

- [ ] **Step 1: Write the failing vitest.** iPhone and Android UA strings → the right kind, and the iPhone steps mention "Certificate Trust Settings". A failed ping keeps the link hidden, and a successful one shows it (fetch mocked).
- [ ] **Step 2: Run it to see it fail.**
- [ ] **Step 3: Implement.** The page works on plain HTTP at `http://<ip>:<port>/x/pocket/setup`.
- [ ] **Step 4: Run it to see it pass.**
- [ ] **Step 5: Commit.** `feat(pocket): phone setup page — exact steps per phone, trust test before the app link`

## Phase D — install

### Task 24: Plugin ports through Shop Builder and Setup

**Files:**
- Modify: `cmd/bootstrap/main.go` (`manifest.Ports []int`, `manifest.Kind string`), the code that writes `install.conf`, `internal/shopconf/shopconf.go` (`Conf.ExtraPorts []int \`json:"extraPorts,omitempty"\``, checked 1–65535), `cmd/setup/linux_system.go` + `linux_uninstall.go`, `cmd/setup/windows_system.go` + `windows_uninstall.go`
- Test: `internal/shopconf/shopconf_test.go`, `cmd/setup/linux_test.go`, `cmd/setup/windows_test.go`, the bootstrap test that builds a conf

**Interfaces:**
- Produces:
  - The Builder writes `extraPorts` = the union of the chosen plugins' `ports`.
  - Linux opens and closes each port the same way as the till port (ufw or firewalld).
  - Windows: one rule `Karots Suite` with `-LocalPort` as a list (`3100,8443`).
  - No plugin name anywhere in setup or shopconf.

- [ ] **Step 1: Write the failing tests.**
  - `TestExtraPortsChecked`: port 0 → error.
  - Linux with ufw active and `ExtraPorts [8443]` → the recorded commands include `ufw allow 8443/tcp`, and the uninstall removes it.
  - The Windows script contains `-LocalPort 3100,8443`.
  - The bootstrap conf for a shop with `pocket` has `extraPorts [8443]`.
- [ ] **Step 2: Run them to see them fail.** `go test ./internal/shopconf ./cmd/setup ./cmd/bootstrap -run 'ExtraPorts|Firewall|Network' -v`
- [ ] **Step 3: Implement.**
- [ ] **Step 4: Run them to see them pass.**
- [ ] **Step 5: Commit.** `feat(setup): open the ports a shop's plugins listen on`

## Phase E — proof, docs, close

### Task 25: End-to-end and UI gate

**Files:**
- Create: `web/e2e/pocket.spec.ts` (import `test` from `e2e/fixtures.ts`), shots in `web/shots/pocket/`
- Modify: `web/playwright.config.ts` only if the 8443 listener needs `ignoreHTTPSErrors` for a project

**Interfaces:**
- Consumes: everything above. The Playwright build has every plugin compiled in, so 8443 is up.

- [ ] **Step 1: Write the spec.**
  - `pair → approve → sell online → print called`: a phone context at 390×844 with `ignoreHTTPSErrors` on `https://localhost:8443/x/pocket/`, paired via the UI. The admin approves via the API, the cashier signs in, adds 2 items and pays cash → the sale shows in the admin sales list.
  - `offline sale replays and flags`: `context.setOffline(true)`, reload from the cached app, sign in offline, sell → "Waiting 1". `setOffline(false)` → "Waiting 0", and the admin Phones → Sends shows the action applied.
  - `takeover`: the same cashier on a PC context with a cart → after the phone signs in, the PC goes to `/lock` with "now on", and Held lists the cart.
- [ ] **Step 2: Run it.** Kill anything on 3100/5173 first, then `make e2e-seed && cd web && bunx playwright test e2e/pocket.spec.ts`. Expected: PASS, and the full suite stays green (`bunx playwright test`).
- [ ] **Step 3: UI gate.** Screenshot sign-in, Sell, Pay, PC down, Held, Menu, the admin Phones page (360 px) and the setup page in Studio, Night and Bazaar × light and dark. Compare them with `docs/design/mockups/pocket-cashier.html` yourself and fix anything off: card edges at 360 px, figures overflowing, raw colours.
- [ ] **Step 4: Commit.** `test(pocket): end-to-end — pair, sell live, sell offline and replay, takeover`

### Task 26: Docs and legal (an app, not a plugin row)

**Files:**
- Modify:
  - `internal/docscheck/docs_test.go`: plugin.json `kind: "app"` means a link in README's **Apps** section instead of the plugin table, a row under **Apps** in `docs/guide/plugins/README.md`, and its tick box in the First Schedule's Apps block. The price row is still required. The badge count and the "all N plugins" word count only non-app plugins. The Everything row must also contain `<name>`.
  - `README.md` (an Apps section with a phone screenshot)
  - `docs/guide/plugins/pocket.md` (create: setup per phone, pairing, selling live and offline, review, release, a lost phone)
  - `docs/guide/plugins/README.md`
  - `web/docs-shots/shots.spec.ts` (key `'pocket'`, a 390 px shot) → `make docs-shots` → `docs/images/plugins/pocket.jpg`
  - `docs/legal/agreement/order-form.html` (☐ Pocket Cashier app, in its own Apps block)
  - `docs/legal/agreement/shop-rules.html`: §15 row "Pocket Cashier app" 5,000. The Everything row becomes "the core with all ten plugins and the Pocket Cashier app", still 95,000. Bump the version on the cover.
  - `LICENSE` (the Second Schedule, only if it lists what's sold), `THIRD_PARTY_NOTICES.md` (`go run ./scripts/notices > THIRD_PARTY_NOTICES.md`; bcryptjs and @zxing/library are now dependencies)
  - `AGENTS.md` (plugin list, `slots.surfaces`, the `OneDevicePerPerson` / `TakeoverGuard` / `AddListener` seams, migrations 00016–00018, next is `00019_`)
  - `docs/guide/architecture.md` (the hooks and slots lists)
  - `docs/status.md` (M8e release 1 done, release 2 next)
- Test: `internal/docscheck`

- [ ] **Step 1: Teach docscheck first** and run `go test ./internal/docscheck`. Expected: FAIL, listing exactly the missing pocket items.
- [ ] **Step 2: Write the docs and legal changes,** then `make docs-shots` and `make agreement`.
- [ ] **Step 3: Run `make test` and `make verify`.** Expected: PASS.
- [ ] **Step 4: Commit.** `docs(pocket): Pocket Cashier app — guide, README, screenshots, order form, price, notices`

### Task 27: Whole-milestone review and clean-up

- [ ] **Step 1:** A fresh reviewer (most capable model) reviews the whole milestone diff against the spec, the Global Constraints and the Review Focus. Fix findings test-first, one commit each.
- [ ] **Step 2:** The user tests on the OnePlus 13 and the iPhone 13: setup page → pair → sell live → stop the server → sell → start it → sales are replayed and the flags are on the review screen.
- [ ] **Step 3:** Delete `docs/specs/2026-10-29-m8e-pocket-cashier-design.md` and this plan (git keeps them), and update `docs/status.md` and the memory. If Task 10 didn't already remove `~/Projects/go/karots-pwaspike`, delete it, and remind the user to remove the spike CA profile from both phones.
- [ ] **Step 4: Commit.** `docs: M8e Pocket Cashier release 1 done`
