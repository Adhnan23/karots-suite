# Karots Suite Onboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** the setup team's Flutter app: one catalogue for every kind of shop, fed by pulling running shops,
used to set up new shops and add to / change running ones as their system user.

**Architecture:** a Flutter app (Android + Linux) with a local SQLite catalogue, talking to each shop's
existing `/api/v1` as the system user (hourly PIN worked out in the app). The Suite gains one call (every
product's packs at once); everything else uses routes that already exist (product / supplier CSV exports,
intake create + restock for old stock, product update, merge, Product Plus, Alternatives, AI Assistant).

**Tech Stack:** Flutter 3.47 / Dart, sqflite + sqflite_common_ffi, http, crypto, csv, flutter_secure_storage,
local_auth (Android), google_fonts (Figtree), mobile_scanner + google_mlkit_text_recognition + image_picker
(Android), `tesseract` CLI for OCR on Linux when installed. Suite side: Go, Echo.

**Spec:** `docs/specs/2026-11-08-suite-onboard-design.md`

## Global Constraints

- App repo: `~/Projects/flutter/karots_suite_onboard`, package `karots_suite_onboard`, name "Karots Suite Onboard". Platforms: Android and Linux only.
- Look = Karots Suite's **Studio** palette (`web/src/theme/palettes/dock/studio.css`): light bg `#EFF0F3`, card `#FFFFFF`, fg `#15161A`, muted fg `#5A5E69`, border `#E4E6EB`, brand `#F2A900` on `#15161A`, success `#12955A`, danger `#D23F3F`, warning `#B66A00`; dark bg `#111215`, card `#1B1C21`, fg `#EDEEF1`, muted fg `#A6A9B3`, border `#2E3038`, brand `#F5B820`, success `#3CCB84`, danger `#F27272`, warning `#F0A23B`. Radius 16 (small 11), pills fully round, font Figtree. Follows the device's light / dark.
- System user phone `0000000001`. PIN = Suite `internal/support/pin.go` exactly: seed = HMAC-SHA256(secret, `"karots-suite/support/seed/v1|" + UPPER(TRIM(installId))`); code = HMAC-SHA256(seed, big-endian uint64(unixUTC / 3600)), first 4 bytes big-endian uint32 % 1000000, zero-padded to 6.
- Every request carries `X-Ks-Device: karots-suite-onboard`; the session is the `pos_token` cookie from `/auth/login`.
- Old stock is never a delivery: only intake create / restock (no money, no supplier bill).
- Money is a decimal string everywhere (never a double) on the wire and in the database.
- Suite rules: core plugin-inert, `make types` after a DTO change, files ≤ 500 lines (Dart too), no AI attribution in commits.
- The app opens only after the user's own PIN (and fingerprint on Android); the support secret lives in secure storage.

## Review Focus

1. A shop's clock or a PIN at the hour's edge: login retries once with the previous hour's PIN (server accepts ±1 window) — test in Task 3.
2. Pull of a big shop (3,000+ products) with names in Sinhala / Tamil and commas or quotes in names — CSV parse test in Task 6.
3. A push while the shop has stock-take turned off (intake refused): clear message, nothing half-made — Task 11 test.
4. A push that fails half-way (product made, second batch refused): the product is kept, the failed batch is listed to retry, not repeated — Task 11 test.
5. Same product scanned under a different extra barcode, or a name differing only in case / spaces: matched, not sent to Review — Task 6 test.

---

### Task 1: Suite — every product's packs in one call

**Files:** Modify `internal/api/admin_packs.go`; Test `internal/api/admin_packs_test.go` (or the existing packs test file).

**Interfaces:** Produces `GET /api/v1/admin/products/packs` → `map[string][]AdminPack` keyed by product id (decimal string), active products with at least one pack.

- [ ] Test `TestAdminAllPacks`: two products, one with packs Card ×10 and Box ×100 → body has only that product's id with both packs, smallest first; a disabled product's packs absent.
- [ ] Run, see it fail (404).
- [ ] Implement `AdminAllPacks` using `products.PacksFor` over active product ids; register `g.GET("/products/packs", h.AdminAllPacks)` **before** `/products/:id/…` routes.
- [ ] `go test ./internal/api -run AllPacks` passes; `make check-core`.
- [ ] Commit `feat(api): every product's packs in one call (for the setup app)`.

### Task 2: App scaffold, theme, app lock

**Files:** `flutter create --org lk.karots --platforms android,linux karots_suite_onboard`; Create `lib/main.dart`, `lib/theme/tokens.dart`, `lib/theme/app_theme.dart`, `lib/lock/lock_screen.dart`, `lib/lock/app_lock.dart`, `lib/settings/secure.dart`; Test `test/theme_test.dart`, `test/app_lock_test.dart`.

**Interfaces:** Produces `KsTokens` (light / dark constants above), `ThemeData ksTheme(Brightness)`, `class Secure { Future<String?> read(String k); Future<void> write(String k, String v); }` (keys `app_pin_hash`, `support_secret`), `class AppLock { Future<bool> hasPin(); Future<void> setPin(String); Future<bool> check(String); }` (PIN stored as SHA-256 of salt+pin).

- [ ] Tests: `ksTheme(Brightness.light).colorScheme.primary == Color(0xFFF2A900)`, dark `0xFFF5B820`, card radius 16; `AppLock` set "4321" → check "4321" true, "1234" false.
- [ ] Implement; first run asks to choose a PIN and paste the support secret; later runs ask the PIN (Android also offers fingerprint via local_auth).
- [ ] `flutter test` passes; `flutter build linux` builds.
- [ ] `git init`, commit `feat: scaffold, Studio theme, app lock`.

### Task 3: Support PIN

**Files:** Create `lib/suite/support_pin.dart`; Test `test/support_pin_test.dart`.

**Interfaces:** Produces `String supportPin(String secret, String installId, DateTime t)`, `List<String> supportPinsNow(String secret, String installId, DateTime now)` (this hour, then the previous one).

- [ ] Get vectors from Go: a throwaway `go run` calling `support.CodeForSecret("test-secret", " fc1a79f7 ", time.Unix(1762560000,0))` and `time.Unix(1762563599,0)`, `time.Unix(1762563600,0)`; paste the three codes into the test.
- [ ] Test the three vectors and that `supportPinsNow` returns two different codes.
- [ ] Implement with `package:crypto`.
- [ ] Pass; commit `feat: the system user's hourly PIN`.

### Task 4: Local database and models

**Files:** Create `lib/data/db.dart`, `lib/data/models.dart`; Test `test/db_test.dart`.

**Interfaces:** Produces tables and Dart classes (all money as `String`):
- `catalog_products(uid, name, name_local, barcode, category, unit, cost, selling, wholesale, tax_rate, warranty_months, track_serial, kinds TEXT /*comma*/, sources TEXT /*shop uids*/, updated_at)`; `catalog_barcodes(product_uid, code)`; `catalog_packs(product_uid, name, size, selling, wholesale, barcode)`; `catalog_fields(product_uid, label, value)`.
- `catalog_suppliers(uid, name, contact, phone, address, credit_days, kinds, sources)`.
- `shops(uid, name, kind, address /*http://host:port*/, install_id, plugins TEXT, last_pull, snapshot TEXT /*json: categories, units, fields, alternatives, ai settings, stock_take*/)`.
- `shop_products(shop_uid, remote_id, catalog_uid, name, barcode, category, json)` (the shop's own list from the last pull).
- `review(uid, shop_uid, kind /*product|supplier*/, json, status /*new|accepted|dropped*/)`.
- `pending(uid, shop_uid, op /*create|update|merge|alt*/, json, error, created_at)`.
- `CatalogProduct`, `CatalogSupplier`, `Shop`, `ReviewItem`, `PendingOp` with `toMap / fromMap`.
- `class Db { static Future<Db> open([String? path]); …CRUD per table… }`.

- [ ] Tests: open in-memory (ffi), insert / read each model round trip; `catalog_products` lookup by any barcode.
- [ ] Implement; pass; commit `feat: local catalogue database`.

### Task 5: Suite client

**Files:** Create `lib/suite/client.dart`, `lib/suite/dto.dart`; Test `test/client_test.dart` (a local `HttpServer` fake).

**Interfaces:** `class SuiteClient(String baseUrl)` with: `Future<LoginInfo> loginInfo()`; `Future<void> signIn(String secret)` (tries `supportPinsNow`, keeps cookie); `Future<Me> me()` (plugins, shopName, stockTakeEnabled); `Future<String> productsCsv({bool extras})` (`/admin/products/export?format=csv&extras=productplus` when Product Plus is on); `Future<String> suppliersCsv()`; `Future<Map<int, List<Pack>>> allPacks()`; `categories()`, `units()`, `suppliers()`; `fields()` (`/x/productplus/fields`); `alternatives()` + `alternative(id)`; `aiSettings()` (`/x/aicatalog/settings`); `identify(IdentifyAsk)`; `intakeCreate(ProductReq, qty, expiry)`; `intakeRestock(productId, qty, expiry, cost, selling, wholesale, supplierId)`; `product(id)`; `updateProduct(id, ProductReq)`; `createSupplier(SupplierReq)`; `quickCategory(path)`; `mergePreview(from, into)`, `merge(from, into)`; `createAlternative(AltReq)`. Errors → `SuiteError(code, message)` from `{"error":{"code","message"}}`.

- [ ] Tests (fake server): sign-in sends phone `0000000001` and the PIN, keeps the cookie on the next call, sends `X-Ks-Device`; first PIN refused → previous hour's PIN tried; an error body becomes `SuiteError` with its message.
- [ ] Implement each call by reading the matching Go handler for its exact body (`internal/api/admin_intake.go`, `admin_products.go`, `admin_suppliers.go`, `admin_categories.go`, `plugins/alternatives/api*.go`, `plugins/aicatalog/api.go`).
- [ ] Pass; commit `feat: Suite client as the system user`.

### Task 6: Pull and match

**Files:** Create `lib/sync/pull.dart`, `lib/sync/match.dart`, `lib/sync/suite_csv.dart`; Test `test/pull_test.dart`, `test/match_test.dart`.

**Interfaces:** `String normName(String)` (lower, collapse spaces, trim); `CatalogProduct? matchProduct(Db, {List<String> barcodes, String name})`; `Future<PullResult> pull(Db, SuiteClient, Shop)` → counts `{known, toReview, suppliersKnown, suppliersToReview}`; it stores the shop snapshot (plugins, categories, units, fields with scopes, alternatives, AI settings, stock-take flag) and `shop_products`.

- [ ] Tests: CSV with a Sinhala name, a comma and a quote in a name, extra barcodes `a;b`; a product matched by its second barcode is known; `"Panadol  500MG "` matches `"panadol 500mg"`; a new one becomes a review row with its packs and custom field values; suppliers likewise; a second pull adds no duplicate review rows.
- [ ] Implement; pass; commit `feat: pull a shop into Review`.

### Task 7: Shops and connect screens

**Files:** Create `lib/screens/shops_screen.dart`, `lib/screens/shop_screen.dart`, `lib/screens/connect_sheet.dart`; Test `test/widgets/shops_test.dart`.

- [ ] Widget test: add a shop (name, kind, address) → listed; the connect sheet shows "unencrypted on Wi-Fi" for a non-Tailscale address (`100.x` or `*.ts.net` counts as Tailscale).
- [ ] Implement: list, add / edit, Connect (login-info fills install id, signs in, shows plugins and stock-take), Pull with progress and counts.
- [ ] Pass; commit `feat: shops and connect`.

### Task 8: Review

**Files:** Create `lib/screens/review_screen.dart`; Modify `lib/data/db.dart` (accept / drop); Test `test/review_test.dart`.

**Interfaces:** `Future<void> acceptReview(Db, String uid, {Map<String, dynamic>? edited})` → catalogue row tagged with the shop's kind and uid; `dropReview`.

- [ ] Tests: accept adds to the catalogue with kind + source; drop never comes back on the next pull (dropped rows are remembered by name / barcode).
- [ ] Implement screen (grouped by shop, accept all / one, edit before accepting); pass; commit `feat: Review`.

### Task 9: Catalogue screen

**Files:** Create `lib/screens/catalog_screen.dart`, `lib/screens/catalog_product_sheet.dart`; Test `test/widgets/catalog_test.dart`.

- [ ] Widget test: search by name and barcode, filter by kind, open and edit a product (name, category, prices, packs, barcodes, fields).
- [ ] Implement; pass; commit `feat: the catalogue`.

### Task 10: OCR and scan into any field

**Files:** Copy and adapt from `~/Projects/flutter/karots_pos_onboard/lib/screens/ocr_capture.dart` and `scan_screen.dart` into `lib/input/ocr_sheet.dart`, `lib/input/scan.dart`, `lib/input/ks_field.dart`; Test `test/ocr_filter_test.dart`.

**Interfaces:** `enum FieldKind { text, number, date, barcode }`; `String filterFor(FieldKind, String picked)` (number: digits and one `.`; date: reads `12/2027`, `2027-12`, `12-31-2027`, `31.12.27` → `YYYY-MM-DD`, month-only → last day); `KsField(label, controller, kind)` = text field with an OCR button (Android camera, Linux file + `tesseract`, hidden when neither) and, for barcode, a scan button (Android).

- [ ] Tests for `filterFor` with those inputs.
- [ ] Implement; pass; commit `feat: OCR and scan into any field`.

### Task 11: Product editor and push

**Files:** Create `lib/screens/product_editor.dart` (fields), `lib/screens/stock_editor.dart` (none / one / several batches), `lib/screens/packs_editor.dart`, `lib/sync/push.dart`, `lib/screens/shop_products_screen.dart`; Test `test/push_test.dart`.

**Interfaces:** `class Draft { CatalogProduct p; String? supplierUid /*catalogue*/; int? supplierId /*shop*/; bool newSupplier; StockMode mode; List<Batch> batches /*qty, expiry, cost, selling, wholesale*/; Map<String,String> fields; }`; `Future<PushResult> pushCreate(Db, SuiteClient, Shop, Draft)`: category (`quickCategory` path) → supplier (create if new) → `intakeCreate` with batch 1 (or no qty) → `intakeRestock` per further batch → saves / updates the catalogue copy; `pushUpdate(…, remoteId, Draft)` → `updateProduct`. Offline → a `pending` row; `flushPending(Db, SuiteClient, Shop)`.

- [ ] Tests (fake server): create with 3 batches → one create + two restocks with their expiry and prices, catalogue gets the product; stock-take off → refused before any call, message "Turn on Stock-take on this shop first"; second restock fails → result lists it, a retry sends only that batch; offline → pending, flush sends it once.
- [ ] Implement editor (shop's custom fields from the snapshot, supplier picker: shop's / catalogue's / new / none, prices, packs, extra barcodes, stock mode) and the shop's product list with Edit; pass; commit `feat: create and change products in a shop`.

### Task 12: AI Identify

**Files:** Create `lib/ai/identify.dart`, `lib/ai/prompts.dart`, `lib/screens/identify_sheet.dart`; Test `test/identify_test.dart`.

**Interfaces:** connected → `client.identify` (paste mode answers with a prompt to copy; the pasted reply goes back to the same call); offline → `String identifyPrompt(String query, ShopSnapshot s)` mirroring `plugins/aicatalog/prompts.go` + `identify_fields.go` (naming style, custom instructions, ticked fields with categories) and `IdentifyResult parseIdentify(String reply)`. A pick fills name, category and the custom fields in the editor.

- [ ] Tests: offline prompt has the ticked field line `generic_name (Generic name, text), only for categories: Medicine`, the naming style and custom instructions; a fenced reply parses; values map to fields.
- [ ] Implement; pass; commit `feat: AI Identify with the shop's settings`.

### Task 13: Duplicates

**Files:** Create `lib/dupes/find.dart`, `lib/screens/dupes_screen.dart`; Test `test/dupes_test.dart`.

**Interfaces:** `List<DupPair> findDupes(List<CatalogProduct>)` (shared barcode, same `normName`, or names equal after dropping spaces / punctuation and unit words like `mg`, `ml`, `tabs`); `mergeCatalog(Db, keepUid, dropUid)` (barcodes, packs, fields, kinds, sources move; empty fields filled from the dropped one); shop: `mergePreview` then `merge` on the connected shop's `shop_products` pairs.

- [ ] Tests: `Panadol 500mg Tabs` / `panadol 500 mg tabs` paired; different sizes not paired; merge keeps all barcodes.
- [ ] Implement; pass; commit `feat: find and merge duplicates`.

### Task 14: Alternatives

**Files:** Create `lib/screens/alternatives_screen.dart`, `lib/sync/alternatives.dart`; Test `test/alternatives_test.dart`.

- [ ] Tests (fake server): a pulled group is listed with its members' names; creating a group in a shop sends the body `plugins/alternatives` expects with the shop's product ids.
- [ ] Implement (pulled groups per shop; Create in connected shop from chosen products); pass; commit `feat: Alternatives`.

### Task 15: CSV export

**Files:** Create `lib/csv/export.dart`; Test `test/export_test.dart`.

**Interfaces:** `String productsCsv(List<Draft>, ShopSnapshot)` with the Suite's import columns (`name,name_local,barcode,extra_barcodes,category,unit,cost_price,selling_price,wholesale_price,tax_rate,reorder_level,opening_qty,supplier,track_serial,warranty_months` + the shop's custom field labels); `String suppliersCsv(List<CatalogSupplier>)` with the Suite's supplier columns (read them from `internal/api/admin_import.go`).

- [ ] Tests: header exact; a custom field column filled; a quote in a name escaped. Then a real check: import the file in the dev Suite (Sheets ▾ → Import, custom fields on).
- [ ] Implement with save (Linux) / share (Android); pass; commit `feat: CSV export for a shop`.

### Task 16: End-to-end on the dev Suite, docs

- [ ] Run the dev Suite with every plugin; in the Linux app: connect as system user, pull, accept, create a product with packs, 2 batches, a custom field; Identify; merge a pair; create an Alternatives group; export CSV. Screenshot the main screens light and dark and look at them.
- [ ] App `README.md` (what it is, first run, connecting, building the APK); Suite `docs/status.md` note.
- [ ] Commit both repos.
