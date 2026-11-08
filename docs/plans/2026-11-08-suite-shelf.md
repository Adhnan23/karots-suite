# Karots Suite Shelf Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Shop staff sign in to their own shop's Suite from an Android app (Karots Shelf, built from Onboard's
code) and add and edit products, offline too, with a queue.

**Architecture:** Suite side: a per-user flag (`can_use_shelf`), a Shelf sign-in that never claims the
person's one device, and a guard that lets flagged users (plus admin / manager) reach only the API routes the
app calls when the request says it comes from Shelf. App side: a second Android flavour in the Onboard repo
(`--flavor shelf`, `lib/main_shelf.dart`) that reuses Onboard's shop screens for one shop, skips everything
catalogue-related, signs in as the person, and keeps a queue per person.

**Tech Stack:** Go (Echo, sqlx, goose, Postgres 17), React (Users page only), Flutter (Android).

**Spec:** `docs/specs/2026-11-08-suite-shelf-design.md`

## Global Constraints

- Names: full **Karots Suite Shelf**, home screen **Karots Shelf**; Android id `lk.karots.karots_suite_shelf`.
- Admin and manager always have access; cashier and sales person only with the Users-page tick.
- The tick is the whole trust: the app ignores every other permission (Can see cost etc.).
- A Shelf sign-in is **not a device** under "one person, one device": never throws a till off, never refused
  because of one.
- Free: no plugin, no price, no legal change. The user's team installs the APK.
- Phones never sync with each other or with Onboard; Shelf never writes Onboard's catalogue or review list.
- The PIN is never stored; a salted hash for the offline unlock only.
- Suite rules (`AGENTS.md`): core plugin-inert (`make check-core`), `make types` after `dto*.go`, files ≤ 500
  lines, next migration is `00032_`, work on `main`, no AI attribution in commits.
- App: `dart format -l 150`; `flutter analyze` clean; Onboard keeps working and keeps its id.

## Review Focus

1. **Access removed while a phone is signed in:** the next call is refused (403 `SHELF_NO_ACCESS`), the app
   says "Ask the owner to give you Karots Shelf again" and keeps that person's queue. Tests: Task 3, Task 8.
2. **A cashier with an open till (Pocket Cashier compiled in) signs in to Shelf:** sign-in succeeds, the till
   keeps working. Test: Task 2.
3. **Two people add the same barcode offline:** the second push is refused by the shop, stays in the queue
   with the shop's reason; no twin. Test: Task 8.
4. **Wrong PINs on the offline unlock:** after 5 wrong in a row the phone stops unlocking offline and asks for
   an online sign-in. Test: Task 7.
5. **A person who must change their PIN** (`PIN_CHANGE_REQUIRED`): Shelf says "Change your PIN on the shop's
   PC first" instead of failing quietly. Test: Task 6.

---

## Part A: the Suite (`/home/karots/Projects/go/karots-suite`)

### Task 1: The "Can use Karots Shelf" tick

**Files:**
- Create: `migrations/00032_user_shelf.sql`
- Modify: `internal/features/auth/auth.go`, `internal/features/auth/repository.go` (insert / update / select
  carry the column like `can_see_low_stock`), `internal/middleware/user_validator.go` + `internal/middleware/flags.go`
  (flag in the request context), `internal/api/dto_people.go` (user DTOs), `internal/api/dto.go` (`/me`),
  `internal/api/me.go`, the users handlers, `web/src/features/people/users/UsersPage.tsx`, `AGENTS.md`
  (migration list)
- Test: `internal/api/admin_users_test.go` (or the existing users test file)

**Interfaces:**
- Produces: column `users.can_use_shelf boolean NOT NULL DEFAULT false`; `middleware.CanUseShelf(c) bool`
  (raw flag); `middleware.MayUseShelf(role string, flag bool) bool` = role admin / manager or flag;
  JSON `canUseShelf` on the user DTOs and request; `/me` field `mayUseShelf` (role-aware).

- [ ] **Step 1: Failing test** `TestUserShelfFlag`: an admin creates a cashier with `canUseShelf: true`, the
  users list returns it true; updating with false returns false; `/me` for an admin says `mayUseShelf: true`,
  for a cashier without the tick `false`, with it `true`.
- [ ] **Step 2:** `go test ./internal/api -run TestUserShelfFlag` → FAIL (unknown field).
- [ ] **Step 3:** Migration (down drops the column), model, repository, validator, DTOs, `/me`, `make types`.
  Users page: a Toggle "Can use Karots Shelf" with hint "Add and change products from the Karots Shelf phone
  app, during setup. Untick when setup is done." Shown only for cashier (incl. sales person); admin / manager
  rows don't show it (they always can).
- [ ] **Step 4:** Test passes; `cd web && bunx tsc --noEmit -p .`; screenshot the user form at 360 px and
  desktop, check it yourself.
- [ ] **Step 5:** Commit `feat(users): Can use Karots Shelf (the shop staff's product app)`.

### Task 2: Shelf sign-in is not a device

**Files:**
- Modify: `internal/api/dto.go` (`LoginRequest.App string json:"app,omitempty"`), `internal/api/auth.go`
  (`Login`), `internal/api/device.go` (`movedTo`)
- Test: `internal/api/device_test.go`

**Interfaces:**
- Consumes: `middleware.MayUseShelf` (Task 1).
- Produces: `POST /auth/login` with `"app":"shelf"`: wrong PIN → 401 as today; signed in but may not use
  Shelf → 403 code `SHELF_NO_ACCESS`, message "Ask the owner to give you Karots Shelf (Users page)", no
  cookie set; otherwise the cookie, and `takeDevice` is **not** called. Header constant
  `shelfHeader = "X-Ks-App"`, value `"shelf"`: `movedTo` returns "" for such a request when the person may
  use Shelf.

- [ ] **Step 1: Failing tests** with `plugin.SetOneDeviceForTest(true, guardThatHolds)` (a guard that
  refuses takeover, like Pocket Cashier's open till):
  `TestShelfLoginKeepsTheTill`: cashier (ticked) signs in on device A (till); signs in with `app: shelf`
  on device B → 200; a `/till/config` call from A still 200; a Shelf call from B (header + device B) → 200.
  `TestShelfLoginNoAccess`: unticked cashier with `app: shelf` → 403 `SHELF_NO_ACCESS`, no `pos_token` cookie.
  `TestShelfHeaderNeedsTheTick`: an unticked cashier's request with the header from another device still gets
  `SESSION_LOCKED` (the header alone grants nothing).
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement as in Interfaces.
- [ ] **Step 4:** `go test ./internal/api -run 'TestShelf|TestDevice'` → PASS (old device tests too).
- [ ] **Step 5:** Commit `feat(auth): a Karots Shelf sign-in never takes the person's device`.

### Task 3: The Shelf route guard

**Files:**
- Create: `internal/api/shelf.go` (the route list + guard), `internal/plugin/shelf.go` (the plugin seam)
- Modify: `internal/api/admin.go` and `internal/api/api.go` (the `/x` admin group) to use the guard in place of
  `RequireRole(admin, manager)`; `plugins/productplus/productplus.go`, `plugins/aicatalog/plugin.go` (register
  their Shelf routes)
- Test: `internal/api/shelf_test.go`, a case in `internal/plugin` tests for the seam

**Interfaces:**
- Consumes: Task 1's flag, Task 2's header.
- Produces: `func (h *handlers) adminOrShelf(next echo.HandlerFunc) echo.HandlerFunc`: admin / manager pass as
  today; anyone else passes only if `MayUseShelf`, the request has `X-Ks-App: shelf`, and `method+" "+c.Path()`
  is in `shelfRoutes` or `plugin.ShelfRoutes()`; a flagged-route miss → 403 `apperr.Forbidden("")`; a person
  without the tick sending the header → 403 `SHELF_NO_ACCESS`.
  Plugin seam: `func (r *Registry) AllowShelf(method, path string)` (path exactly as the plugin registers it,
  e.g. `"/productplus/fields"`; the seam stores `"/api/v1/x" + path`), `func ShelfRoutes() []string`.
  Core list (`shelfRoutes`, `"METHOD /api/v1/admin/…"` with Echo `:id` paths): GET `/units`, POST `/units`,
  GET `/categories`, POST `/categories/quick`, GET `/supplier-options`, POST `/suppliers`, GET
  `/suppliers/export`, GET `/products/export`, GET `/products/packs`, GET + PUT `/products/:id`, the
  barcodes and packs routes the app calls under `/products/:id/…`, POST `/inventory/intake/create`, POST
  `/inventory/intake/restock`, POST `/stock/adjust`, GET `/stock/lots/:id`, POST
  `/stock/batches/:id/expiry`, POST `/stock/batches/:id/price`. The builder confirms each exact path and
  method against `registerAdmin*` and the app's `lib/suite/client.dart` / `lib/sync/*.dart` (Onboard), and
  leaves out merge, settings and Alternatives (Shelf doesn't use them).
  productplus: `AllowShelf("GET", "/productplus/fields")`; aicatalog: `AllowShelf("GET", "/aicatalog/settings")`,
  `AllowShelf("POST", "/aicatalog/identify")`.

- [ ] **Step 1: Failing tests** `TestShelfGuard`: ticked cashier + header: GET `/admin/units` 200, POST
  `/admin/products/:id` path from the list works (PUT a seeded product → 200); GET `/admin/sales` 403; GET
  `/admin/settings` 403; same cashier **without** the header: GET `/admin/units` 403; untick the cashier (DB
  update) then GET `/admin/units` with the header → 403 `SHELF_NO_ACCESS`; admin with no header: everything
  as before. `TestShelfPluginRoutes`: with a test plugin calling `AllowShelf("GET","/t/ping")`, the guard lets
  a ticked cashier reach `/api/v1/x/t/ping` and not `/api/v1/x/t/other`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement; register the plugin routes.
- [ ] **Step 4:** `make test` (whole Go suite, one package at a time) → PASS; `make check-core` → OK.
- [ ] **Step 5:** Commit `feat(api): Karots Shelf reaches only its own routes`.

### Task 4: Suite docs

**Files:**
- Create: `docs/guide/shelf.md` (what Shelf is, who gets it, the tick, works offline, the setup team installs
  it, untick when done; Onboard is not mentioned to shops)
- Modify: `docs/guide/README.md` (or the people/users guide page) with a line + link; `docs/status.md`

- [ ] **Step 1:** Write the pages in the guide's voice.
- [ ] **Step 2:** If `web/docs-shots/shots.spec.ts` has the user form shot, retake it (`make docs-shots`
  needs a running Demo Room; skip with a note in status if not available).
- [ ] **Step 3:** Commit `docs(guide): Karots Shelf`.

## Part B: the app (`~/Projects/flutter/karots_suite_onboard`)

### Task 5: The Shelf flavour

**Files:**
- Modify: `android/app/build.gradle.kts` (flavour dimension `app`, flavours `onboard` (id
  `lk.karots.karots_suite_onboard`, label "Karots Onboard") and `shelf` (id `lk.karots.karots_suite_shelf`,
  label "Karots Shelf") via `manifestPlaceholders["appLabel"]`), `android/app/src/main/AndroidManifest.xml`
  (`android:label="${appLabel}"`), `README.md` (build commands)
- Create: `lib/flavor.dart` (`bool get isShelf => appFlavor == 'shelf';` from `package:flutter/services.dart`),
  `lib/main_shelf.dart` (Shelf's `main`; Task 7 fills its home), `android/app/src/shelf/res/mipmap-*` (an
  icon: Onboard's with a different background colour is fine)

- [ ] **Step 1:** Add the flavours; `main_shelf.dart` shows a placeholder "Karots Shelf" page for now.
- [ ] **Step 2:** `flutter build apk --flavor onboard -t lib/main.dart` and
  `flutter build apk --flavor shelf -t lib/main_shelf.dart` both succeed; `aapt dump badging` (or
  `apkanalyzer`) shows the two ids and labels.
- [ ] **Step 3:** `flutter test` still passes.
- [ ] **Step 4:** Commit `build: a second app, Karots Shelf (flavour shelf)`.

### Task 6: Signing in as a shop person

**Files:**
- Modify: `lib/suite/client.dart`
- Create: `lib/shelf/account.dart`
- Test: `test/shelf_signin_test.dart` (with `test/fake_shop.dart`)

**Interfaces:**
- Produces: `SuiteClient(base, {app})` with `final String app` (`''` Onboard, `'shelf'` Shelf); when
  `app == 'shelf'` every request carries `X-Ks-App: shelf` and `X-Ks-Device: karots-shelf-<device uid>`.
  `Future<ShelfUser> SuiteClient.signInPerson(String phone, String pin)` posting
  `{phone, pin, app: 'shelf', deviceLabel: 'Karots Shelf'}`; `class ShelfUser { String name, role, phone; }`
  from the login response. Friendly `SuiteError` messages: 401 → "Wrong phone number or PIN"; 403
  `SHELF_NO_ACCESS` → "Ask the owner to give you Karots Shelf (Users page)"; `PIN_CHANGE_REQUIRED` →
  "Change your PIN on the shop's PC first".
  `lib/shelf/account.dart`: `class ShelfAccount { String address, phone, name, role, pinSalt, pinHash;
  int wrongTries; }` kept as JSON in the `KeyStore` under `shelf_account_<phone>` plus `shelf_people` (list of
  phones) and `shelf_current`; `String hashPin(String pin, String salt)` (PBKDF2-SHA256 from the
  `cryptography` package, 50,000 rounds, base64); `bool checkPin(ShelfAccount a, String pin)`.

- [ ] **Step 1: Failing tests:** the fake shop answers `/auth/login`; `signInPerson` sends `app: shelf` and the
  header; each error code above gives its message; `hashPin` is salted (two salts → two hashes) and
  `checkPin` accepts the right PIN only.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `flutter test test/shelf_signin_test.dart` → PASS.
- [ ] **Step 5:** Commit `feat(shelf): sign in as the shop's own person`.

### Task 7: Shelf screens: connect, lock, home

**Files:**
- Create: `lib/shelf/shelf_app.dart` (root state: connect → lock → home), `lib/shelf/connect_screen.dart`
  (address, then phone + PIN; "Someone else" to add another person on this phone), `lib/shelf/lock_screen.dart`
  (PIN pad + phone unlock as Onboard's; shows who), `lib/shelf/session.dart`
- Modify: `lib/main_shelf.dart`, `lib/state.dart` (`AppState.connectPerson(Shop s, String phone, String pin)`
  beside `connect`), `lib/screens/shop_actions.dart` (an action may be `onboardOnly`; Alternatives is),
  `lib/screens/shop_screen.dart` (no shop-profile edit / delete / secret UI when `isShelf`)
- Test: `test/widgets/shelf_lock_test.dart`

**Interfaces:**
- Consumes: Task 6.
- Produces: one `Shop` row for the shop (made at the first connect, named from `/me`'s shop name). Home is
  Onboard's `ShopScreen(shop)` for it. `ShelfSession { ShelfAccount who; String? pin; }`: the PIN is held in
  memory only while unlocked; on any `SuiteError` 401 from the shop, `AppState` signs in again with it once
  and retries. Idle lock after 10 minutes (no touches). Offline unlock: `checkPin`; 5 wrong in a row →
  `wrongTries` = 5 → the lock screen says "Too many wrong PINs: sign in again with the shop in reach" and only
  an online sign-in clears it.

- [ ] **Step 1: Failing widget tests:** with a saved account: the right PIN opens home with no network; a
  wrong PIN shows "Wrong PIN"; 5 wrong → the "sign in again" message and the right PIN no longer opens it
  offline; "Someone else" leads to the phone + PIN form.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** Tests pass; `flutter analyze` clean; run the Shelf build on the Linux desktop or an emulator
  against the dev Suite and screenshot connect, lock and home; check them yourself.
- [ ] **Step 5:** Commit `feat(shelf): connect, lock and the shop's page`.

### Task 8: One shop, a queue per person

**Files:**
- Modify: `lib/sync/pull.dart` (when `isShelf`: no catalogue matching, no review, no Alternatives;
  everything else the same), `lib/sync/push.dart` (`keepCopy` keeps the shop's own list only when `isShelf`;
  `queue` stamps the person; `flushPending` sends only the current person's), `lib/data/db.dart` (schema
  version 5: `pending.who TEXT` default `''`), `lib/data/models.dart` (`PendingOp.who`),
  `lib/screens/pending_screen.dart` or wherever "Waiting to push" lives (others' items shown greyed "waiting
  for <name>"), the stock-take prompt (when `isShelf` and the person isn't admin / manager: say "Ask the
  owner to turn on Stock-take (Settings)" instead of offering to turn it on)
- Test: `test/shelf_queue_test.dart`, `test/pull_test.dart` (a Shelf case)

**Interfaces:**
- Consumes: Tasks 6–7 (`isShelf`, the current person's phone).
- Produces: `Future<void> queue(Db, Shop, String op, Draft, {String who = ''})`; `flushPending(db, c, s,
  {String who = ''})` sends only rows with that `who` (Onboard passes `''`, unchanged).

- [ ] **Step 1: Failing tests:** a Shelf pull against the fake shop leaves `catalog_products` and `review`
  empty and fills the shop's list; A's queued product isn't sent while B is signed in, and is when A is; a
  queued product whose create the fake shop refuses (409 barcode already used) stays in the queue with that
  message; a 403 `SHELF_NO_ACCESS` during a flush keeps the queue and surfaces "Ask the owner…".
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `flutter test` (all) → PASS; Onboard's own pull / push tests unchanged.
- [ ] **Step 5:** Commit `feat(shelf): one shop, nothing in the catalogue, a queue per person`.

### Task 9: End to end and hand-over

**Files:**
- Modify: `test/live_test.dart` (a Shelf round trip: sign in as a ticked cashier, pull, create a product with
  one batch, change it, see it in the shop; an unticked one is refused), `README.md`,
  `/home/karots/Projects/go/karots-suite/docs/status.md`

- [ ] **Step 1:** In the Suite: `make plugins P=all`, `make dev`; seed a cashier with the tick.
- [ ] **Step 2:** `LIVE_SUITE=http://localhost:3000 flutter test test/live_test.dart` → PASS (Onboard's cases
  too).
- [ ] **Step 3:** Build both APKs; copy them to `~/Downloads/karots-shelf.apk` and
  `~/Downloads/karots-onboard.apk`.
- [ ] **Step 4:** Suite: `make core-only`, `make check-core`, `make test`, `make test-web`; the Playwright
  specs touching users / login (`bunx playwright test e2e/admin-people.spec.ts e2e/auth.spec.ts`, kill
  anything on 3000 / 5173 first).
- [ ] **Step 5:** A fresh reviewer reads the whole change (both repos) against the spec; fix findings
  test-first.
- [ ] **Step 6:** Delete this plan and the spec (and the finished Onboard plan + spec), update
  `docs/status.md`; commit `docs: Karots Shelf done`.
