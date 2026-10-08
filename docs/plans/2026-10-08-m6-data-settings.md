# M6 Data and settings

> Lean plan, same shape as M2–M5. Opus plans, executes and reviews. Each task is a vertical slice:
> karots-pos **behaviour** (handler + templ + `static/js` + memory note) → DTOs (`internal/api/dto_data.go`,
> `make types`) → handlers with a Go test written first → SPA screen → self-reviewed screenshots
> (4 combos, 1366 + 360) → one commit. **Don't copy karots-pos UI 1:1** — build it from the admin kit
> in the approved look.

**Goal:** backup / restore (manual, selective, automatic), Settings, product / customer / supplier
import + export, settings sheet import / export, the setup checklist, the Stock Capture catalog sync —
and everything M6.5 needs (run steps, a real karots-pos backup restored and checked, the side-by-side
checklist, `docs/differences.md`). Spec §11 M6 — **done when a karots-pos backup restores into
karots-suite.**

**Parity rows:** `register_admin.go` `/backup`, `POST /restore`, `/data*` (7), `/settings*` (5),
`/setup`, products / customers / suppliers `export` · `import` (GET + POST) · `import/template` (12).
`/setup` is ticked through the dashboard checklist + the dock Setup shelf (as M4 did for `/money`).

**Out of M6:**
- **Self-update — all of it moves to M8 (user, 2026-10-08).** No suite shop exists before M8 and the
  client is useless without the release side. Note it on the three `/update` parity rows and in
  status.md's M8 inputs.
- Appearance is done (M0, System → Appearance); M6 only checks it still works after a restore.
- Printer-only rows stay unticked (need a real POS80) — the printer test button is built, its row
  ticks in M6.5 with the hardware.

**karots-pos sources to read per task:** `internal/backup/*`, `internal/web/{admin_backup,admin_data,
data_files,data_settings,admin_settings,setup,import_products,import_customers,import_suppliers,csv}.go`,
`templates/pages/admin/{data,settings,setup}.templ`, `templates/fragments/admin/import*.templ`,
`static/js/` for the settings / data / import screens. **Memory first** (karots-pos memory dir):
`backup-restore-invariants.md`, `data-page-backup-export.md`, `stock-capture-lan-sync.md`,
`cash-drawer-kick.md`, `credit-as-payment-type.md` (`default_sale_type` landmine),
`shared-drawer-continue-last-toggle.md`.

## Shared design

- **API:** `/api/v1/admin/data/*` — **admin only** (`RequireRole(admin)`; manager → 403).
  `/api/v1/admin/settings*` — admin + manager, as karots-pos. Imports / exports sit on their entity's
  group (`/admin/products/export` …) with that group's roles.
- **Downloads** are plain GETs the SPA opens with `<a href download>` (cookie auth, no blob juggling);
  uploads are multipart (`file` field, as `readImportSheet`; restore uses `backup`).
- **Upload size:** restore accepts up to **256 MB** (a few years of a busy shop gzipped is tens of MB);
  imports up to 8 MB. Per-route body limit, not a global raise.
- **Audit** every backup download, restore, settings save, logo change, import and settings import
  (`audit.ActionBackup` / `ActionRestore` / `ActionSettings`, as karots-pos).
- **Dock:** Setup gains **Settings** (admin + manager) and **Data** (admin). The dashboard gains the
  setup checklist card (admins, hidden once every step is done).
- **Kit reuse:** `writeSheet` / `readImportSheet` / `cellGetter` / `importErrMsg` (`sheets.go`),
  `ExportMenu` (reports kit), `FormSheet`, `ConfirmButton`, `PageHead`, `DataList`. One new shared
  piece: `components/admin/ImportSheet.tsx` (template download, file pick, result list) used by all
  three entities.

## Upgrades over karots-pos (flaws found while planning, not copied)

1. **Restore breaks on any schema difference.** `insertRows` inserts the archive's column list as is,
   so a backup with a column the target lacks (a newer karots-pos, or a suite `00002_` dropping /
   renaming one) fails the whole restore. Restore now inserts **archive ∩ target columns** and reports
   what it dropped; target columns the archive lacks take their defaults (an older backup). Plugin
   tables the target doesn't have (a plugin shop's backup into a core-only build) are skipped and
   reported, not silently ignored.
2. **No undo for a restore.** karots-pos TRUNCATEs everything on an upload with no copy of what was
   there. The suite writes a **safety snapshot** (`WriteSnapshot` into `BACKUP_DIR`, name suffixed
   `-before-restore`) before it touches anything and refuses to restore if that snapshot fails. It
   shows up in the auto-backup list, so undo = restore it.
3. **A restore leaves stale sessions.** The users table is replaced, so the restorer's JWT may now
   point at another person (ids are reused). After a restore the server clears the session cookie and
   the SPA goes to `/login`.
4. **Old write-side damage is invisible.** Data written by karots-pos before the fixes found in M3–M5
   is restored as is. A read-only **data check** lists it after every restore and on the Data page
   (task 1); repairs are the user's call in M6.5, never automatic.
5. **Stock Capture login.** The suite's login only sets an httpOnly cookie, but the app reads
   `access_token` from `/api/auth/login`. The suite mounts the two **legacy paths** the app uses
   (`POST /api/auth/login` → `{data:{access_token}}`, `GET /api/sync/catalog` → `{data:[…]}`) so the
   user's phone app works unchanged; same rate limiter, same PIN rules, nothing else on that prefix.

## Review Focus

1. **A real karots-pos backup restores and adds up.** The user's backup (and one from the dev
   `pos_db` on 5432, with plugin tables) restores into the suite; every core table's row count
   matches the archive; the next sale gets the next S- / CR- / DP- number, not a collision; a till
   opens; stock, dues and locker balances equal what karots-pos showed. Tests: task 0 (Go) +
   task 6 (the real file).
2. **A failed restore changes nothing.** A truncated gzip, wrong JSON, a future `FormatVersion`, or a
   row breaking an FK → 422 with a readable reason, the DB exactly as before, the safety snapshot on
   disk. Test: task 0.
3. **File names from the client are never paths.** `../`, absolute paths, `x.json.gz/../..`, a name
   not in the list → 400 on download and restore-from-file. Test: task 0 (`safeBackupName`).
4. **Imports with messy sheets.** Header synonyms in any case, `1,250.00`, a blank line, an XLSX
   numeric cell (`1250` read as `1250`, not `1.25E3`), a duplicate barcode, re-importing the same file
   (updates, never duplicates), a bad row in the middle → the good rows land, each bad row comes back
   as `line N: reason`. Test: task 4.
5. **Settings import / who may touch data.** A settings sheet never overwrites `install_id`, the
   printers or the default locker; a manager gets 403 on every `/admin/data/*`; a cashier gets 403 on
   Settings. Tests: tasks 2, 3.

---

## Tasks

- [x] **0. Backup engine upgrades + data API** (`internal/backup/`, `internal/api/admin_data.go`, `dto_data.go`).
  - **Go first** (`internal/backup/restore_db_test.go`, rolled back / own schema as the existing
    tests do): an archive with (a) an extra column on `units`, (b) a table the DB lacks
    (`recharge_bills`), (c) `products` missing a column that has a default → restore succeeds,
    `RestoreReport{Tables, Rows, SkippedTables, DroppedColumns}` names (a) and (b), (c) takes the
    default. A broken gzip / bad JSON / `Version: 2` / an FK-breaking row → error and the row counts
    of every table unchanged. Receipt sequences advance past the restored max.
  - `Restore(ctx, db, r) (RestoreReport, error)`: intersect `t.Columns` with `tableColumns(target)`
    per table (Upgrade 1); keep the one-tx / FK-drop / trigger-disable shape.
  - `safeBackupName(dir, name) (path, error)` + `ListBackups(dir) ([]BackupFile, error)` in
    `internal/backup/files.go` (port `data_files.go`; test the traversal cases of Review Focus 3).
  - Endpoints (admin only):
    - `GET /admin/data` → `{backupDir, keep, intervalHours, groups: [{key,name}], backups: [{name, size, modifiedAt}]}`
    - `GET /admin/data/backup?groups=a&groups=b&setupOnly=1` (no params = full; port `DataBackup`;
      `backup.FileName`)
    - `POST /admin/data/restore` (multipart `backup`) and `POST /admin/data/backups/:name/restore`:
      safety snapshot first (Upgrade 2, refuse on failure), restore, audit, clear the cookie
      (Upgrade 3), return `RestoreResult{report, check}` (`check` from task 1; empty until then).
    - `GET /admin/data/backups/:name` (download).
  - Go API tests: manager → 403; restore writes the `-before-restore` file; traversal → 400;
    response clears `pos_token`.
  - No UI yet (task 2). Commit.
  - Done: `Restore` = `ReadArchive` (checks the file, touches nothing) + `RestoreArchive`; the
    handler reads first, so a bad file never writes a safety copy (found at the UI gate: failed
    test restores piled up copies that count toward `BACKUP_KEEP`). `Config.BackupFolder()` =
    `BACKUP_DIR` or `./backups` (the safety copy needs a home on an install without automatic
    backups; `backups/` is git-ignored). Row helpers moved to `restore_rows.go` (≤ 500 lines).
- [x] **1. Data check — damage from old write-side bugs** (`internal/features/datacheck/`).
  - First read the fix commits and their notes (`status.md` M3/M4/M5, `docs/karots-pos-fixes-2026-10-04.md`)
    and list which ones **stored** wrong data (read-only bugs need nothing). Expected at least:
    returns on a part-paid credit sale over-credited the customer (`d55ac5f`); a deleted expense left
    its cash out (`d66ac18`); received money counted the change / non-cash got change
    (`3c0cd9d`, `c5553e7`); an archived locker took or gave money (`15f5554`). Plus the generic
    invariants: customer / supplier stored balance ≠ its ledger; locker stored balance ≠ Σ its moves;
    stock qty ≠ Σ its lots; a cashflow row pointing at a missing source.
  - `datacheck.Run(ctx, db) []Finding{Key, Title, Explain, Count, Sample []FindingRow{Ref, Detail}}` —
    one SQL per check, **read-only**, each with a Go test that inserts the bad shape and expects one
    finding (and a clean seed expects none). Plugin checks: a `plugin.DataChecker` seam only if a
    check needs it — not now.
  - `GET /admin/data/check` (admin) + included in `RestoreResult`. Commit.
  - Done: six checks (`datacheck.go`): over-credited returns (`d55ac5f`), refunds over what was
    paid (`b256484`), "returned" sales with no return (`3f77ef3`), deleted expenses' money
    (`d66ac18` — legs netted per ref, because the suite's own Void deletes the expense and puts the
    money back under the same ref), change beyond the cash (`c5553e7`), money in archived lockers
    (`15f5554`). Dropped: customer balance vs ledger (repayments only logged since 0028, so old
    shops would all flag), stock vs lots (oversell makes them differ legitimately), admin returns
    with no cash move (cashier refunds carry no ref to tell them apart).
- [x] **2. Data page** (`features/data/`, `routes/_authed/admin/data.tsx`, settings sheet endpoints).
  - Sections (karots-pos `data.templ`, in the suite look): **Backup** (Full · Choose what to include —
    one checkbox per plugin group, empty in core — · Catalogue & setup only); **Restore** (pick a file
    → `ConfirmButton` that needs typing `RESTORE`, a progress state, then the report + data check, then
    "Sign in again"); **Automatic backups** (folder, keep, list with Download / Restore — same confirm;
    the `-before-restore` ones labelled "Before restore"); **Import & export** (links to the three
    entity pages' import / export + the settings sheet); **Data check** (findings, collapsed when clean).
  - Settings sheet: `GET /admin/data/settings/export?format=` and `POST /admin/data/settings/import`
    (port `data_settings.go`; skip-list `install_id`, `receipt_printer`, `label_printer`,
    `default_locker_id`, `id`, `updated_at` server-side). Go test: an imported `install_id` is ignored;
    an unknown column is reported, not applied.
  - Dock Setup → Data (admin). Screens 1366 + 360, 4 combos. Commit.
  - Done: `features/data/` (`DataPage`, `RestoreDialog`, `DataCheck`); `apiUpload` in
    `api/client.ts` (stock-take reuses it). The list shows the newest 8 backups + "Show all".
- [x] **3. Settings** (`internal/api/admin_settings.go`, `features/settings/`, `routes/_authed/admin/settings.tsx`).
  - `GET /admin/settings` → `{settings, printers (printing.Queues), lockers (active)}`;
    `PUT /admin/settings` (`settings.UpdateInput`, network-printer override as karots-pos);
    `POST /admin/settings/logo` (multipart, `receiptimg.ToDataURI(…, 400)`, 8 MB) + `POST /admin/settings/logo/clear`;
    `POST /admin/settings/printer/test` (`escpos.TestDocument` + `KickDrawer`, error as a 422 message,
    not a 200 toast). Audit each.
  - Screen: sections from `settings.templ` (Shop & receipt, Logo with a live receipt preview, Tax,
    Till & cash — default locker / untracked cash / continue-last-count / drawer kick —, Printers &
    labels, Security — lock timeout, PIN rules —, Stock — stock-take toggle, low-stock alerts). One
    `FormSheet`-style form per section, each saved alone (a phone can't hold the whole form).
    After save, invalidate `meQuery` (`stockTakeEnabled`, `taxRegistered`, `lockTimeoutMinutes` live there).
  - `default_sale_type` stays `retail` unless the user picks wholesale (karots-pos landmine note).
  - Go tests: cashier → 403; a bad receipt width → 422; logo too big / not an image → 422; saving
    stock-take off hides Stock-take in `/me`. Dock Setup → Settings. Screens. Commit.
  - Done: `ShopSettings` DTO (camelCase) mapped onto `settings.UpdateInput`; each card sends the
    saved settings with only its own fields; one printer box (detected queues as a datalist, or a
    typed `tcp://`); the receipt preview shows the saved settings (+ logo on `Paper`);
    `slots.settingsSections` for M7; the legacy settings `RegisterAPI` is gone.
- [x] **4. Import + export: products, customers, suppliers.**
  - `internal/api/admin_import.go` (≤ 500 lines, split per entity if needed): for each entity
    `GET …/export?format=csv|xlsx|ods` (every active row, the template's columns),
    `GET …/import/template?format=`, `POST …/import` → `ImportResult{created, updated, skipped, errors: [{line, message}]}`.
    Columns + synonyms from karots-pos (`importColumns`, `customerImportSynonyms`,
    `supplierImportSynonyms`). Products go through the existing `products.ImportOne`; customers /
    suppliers port karots-pos's per-row logic — the **opening balance goes through the existing
    opening-balance service** (audited, as M3's OpeningSheet), never a raw balance write.
  - Go tests (Review Focus 4): synonyms, `1,250.00`, XLSX numeric cell, blank line, duplicate
    barcode, re-import updates, bad middle row → partial success with `line N`.
  - SPA: `components/admin/ImportSheet.tsx` + an Import / Export pair on the Products, Customers and
    Suppliers list heads; after import, invalidate the list query. Screens. Commit.
  - Done: `admin_import.go` + `sheet_cells.go` (`amountCell` / `wholeCell`). **Found and fixed in
    both repos:** a customer import duplicated "077-123 4567" and every phoneless customer
    (karots-pos `776fd8e`); a typed "1,250.00" imported as 0, and re-importing a name + price sheet
    wiped cost / category / supplier (customers' limit and address, suppliers' contact and terms) —
    now `Given` columns only (karots-pos `5305a0f`). Also: Excel number cells read raw, a CSV's blank
    lines keep later rows on their line. UI: `components/admin/ImportSheet.tsx` (`SpreadsheetMenu`).
- [x] **5. Setup checklist + Stock Capture sync.**
  - `GET /admin/setup` (admin) → steps from karots-pos `setupStatus` (shop named ≠ "My Shop", a staff
    login other than the system user, categories, products, a default locker, a receipt printer, a
    first sale), each with its suite route. Dashboard card "Get the shop ready" (progress + next step),
    admins only, hidden when all done. Go test: a fresh `-init` DB → 0 done; the system user doesn't
    count as staff.
  - Upgrade 5: `internal/api/legacy_sync.go` mounts `POST /api/auth/login` (JSON phone + pin → the
    same `Auth.Login`, limiter, PIN-change rules) returning `{data:{access_token}}` and
    `GET /api/sync/catalog` (Bearer, any role, `products.SyncCatalog`) returning `{data:[…]}` with the
    karots-pos field names. Delete `internal/features/products/api.go` (legacy) once nothing uses it.
    Go test: login → token → catalog rows; no token → 401; field names match the app's
    `parseSyncCatalog` (`id,name,name_local,barcode,category,unit,selling_price,stock_qty,is_service`).
  - Commit.
  - Done: `admin_setup.go` (system user = `users.is_system`, not the phone env), `SetupCard` on the
    dashboard; `legacy_sync.go`; `products/api.go` deleted.
- [x] **6. M6.5 preparation** (docs + one script; no product code).
  - **Real restore:** take a backup from karots-pos's dev `pos_db` (plugins on) **and** the user's
    real backup if one is in reach (ask once, don't block on it); restore each into a fresh suite DB;
    check row counts per table vs the archive, next receipt numbers, a sale + till open/close, the data
    check. Note results (and anything fixed) in status.md. Any restore bug → fix + test first.
  - `docs/acceptance.md`: run steps for both side by side (suite: `make build`, own DB on 5433,
    port 3100, `-init`, restore; karots-pos **core-only**: its bootstrap with no plugins, 5432 / 3000);
    where to read the same day's Z report, P&L, stock value and dues in each, with the **expected
    deliberate differences** called out (the fixes listed in status.md change some figures on old
    data); a checklist by area (till, counter, stock, purchasing, money, reports, data / settings);
    the hardware list (POS80 + drawer kick, label printer, old-screen kiosk PC, phone) mapped to the
    11 printer-only parity rows.
  - `docs/differences.md`: every deliberate change gathered from each plan's "Upgrades over
    karots-pos" and "Done:" notes and the user's decisions (Cash register = Till sessions, Returns by
    return date, audit = Activity "Changes", Low stock = Reorder, menu cards hard-delete, held-sale
    resume refused with a live cart, …), one line each with **keep / change / add back** for the user.
  - Commit.
  - Done: `TestRestoreKarotsPosBackup` (opt-in: `KAROTS_POS_BACKUP` / `KAROTS_POS_DSN`,
    `KAROTS_POS_KEEP`). Restored: dev `pos_db`, `printshop.gz`, `m-u-m-digital-20260922-142337`,
    `pos-backup-20260917-133552` — all tables match, receipt numbers follow on, only plugin tables
    left out. Both run-step builds were tried (suite `-migrate` / `-init`, karots-pos core-only
    bootstrap).
- [x] **7. Flows, parity, docs.**
  - Playwright `admin-data.spec.ts` at 1366 + 360: backup downloads (full + setup-only); restore of
    that file (typed confirm → report → sign in again → data still there); auto-backup list shows the
    `-before-restore` file; settings save round-trip (shop name on the receipt preview, stock-take
    toggle hides the page); logo upload + clear; product / customer / supplier import of a fixture
    sheet with one bad row; export downloads; dashboard checklist; manager can't reach Data.
    **Restore wipes the DB** — the spec runs last or re-seeds (`make e2e-seed`) after itself.
  - Tick parity rows (note M8 on the `/update` rows); `status.md` handoff to M6.5 (karots-pos commits
    to push, if any); spec §11 unchanged.
  - Ship check: types, vet, tests, build, `verify-portable`, plugin grep, ≤ 500 lines.
  - Milestone review + one `fix:` commit.
  - Done: `admin-data.spec.ts` (5 flows); 92 Playwright flows, 104 web unit tests, `go test ./...`
    green; `verify-portable` OK; 26 parity rows ticked (324 in all).
