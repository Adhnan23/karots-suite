# M8c2 Backup copies Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every automatic backup also lands in one owner-chosen copy folder (USB / share) and, for paying shops, the
vendor's encrypted Karots cloud bucket — restorable from the Data page and from Setup's Bring back a shop.

**Architecture:** A copy pass in `internal/backup` runs after each scheduled backup and every 30 min, driven by a
`backup_copies` table (never backed up). The folder copy catches up and prunes; the cloud gets AES-GCM-sealed files
through a stdlib SigV4 client moved to `internal/s3`, about one a day. Data page, Shop Builder and Setup grow small
pieces on top.

**Tech Stack:** Go (Echo, sqlx, goose, stdlib crypto/aes + net/http), React + TanStack Query, tygo.

**Spec:** `docs/specs/2026-10-20-m8c2-backup-copies-design.md`

## Global Constraints

- Never stop a sale: copy errors are logged and stored on the row, never returned to a till request, never fail a backup.
- Stdlib only for S3 and crypto. Files ≤ 500 lines. Core plugin-inert (nothing here touches plugins).
- `backup_copies` is never dumped and never touched by a restore.
- Cloud: only the newest backup, when the last cloud copy is > 20 h old; never list on a timer; shops never delete.
- Cloud object names: `<CLOUD_BACKUP_FOLDER>/<yyyymmdd-hhmmss>.json.gz.enc` — no shop name.
- Copy folder keeps `BACKUP_KEEP` newest; only `*.json.gz` copied; written as `.tmp` then renamed.
- Only the system user may switch the cloud **on**; any admin may switch it **off**.
- `.env` keys: `BACKUP_COPY_DIR`; `CLOUD_BACKUP_URL` (`https://<account>.r2.cloudflarestorage.com/<bucket>`),
  `CLOUD_BACKUP_KEY_ID`, `CLOUD_BACKUP_SECRET`, `CLOUD_BACKUP_FOLDER`, `CLOUD_BACKUP_CRYPT` (64 hex), `CLOUD_BACKUP` (on|off).
- Every admin screen works at 360px and in Studio/Night × light/dark; theme tokens only.
- No Claude attribution in commits. One commit per task.

## Review Focus

1. Copy folder set to the backups folder itself (or inside it) → refused on save (else the pass copies into itself and
   prunes the shop's own backups). Test in Task 4 / Task 5.
2. USB pulled mid-copy → a stray `*.tmp` at the place; the next pass ignores it for listing/prune and overwrites it. Task 4.
3. Clock wrong (last cloud copy time in the future) → treated as stale, upload happens. Task 4.
4. A client-supplied file name on fetch/restore from a copy (`../`, other shop's folder) → refused unless it is a bare
   name the place actually lists. Task 5.
5. A file fetched back from a copy lands in the local folder with *now* as its mtime → it must keep the backup's own
   time (Chtimes), or the cloud pass re-uploads an old backup as "newest". Task 5.

---

### Task 1: `internal/s3` — shared SigV4 client

**Files:**
- Create: `internal/s3/s3.go`, `internal/s3/s3_test.go`
- Modify: `cmd/bootstrap/r2.go` (use `s3.SignV4`; delete its copy), `cmd/bootstrap/r2_test.go` (move `TestSignV4MatchesAWSExample` to `internal/s3`)

**Interfaces:**
- Produces:
  - `func SignV4(req *http.Request, host, region, keyID, secret string, body []byte, now time.Time)` (moved verbatim)
  - `type Client struct{ URL, KeyID, Secret string; HTTP *http.Client }` — URL = `https://host/bucket` (path-style), region `auto`
  - `func (c Client) Put(ctx context.Context, key string, body []byte) error`
  - `func (c Client) Get(ctx context.Context, key string) ([]byte, error)`
  - `type Object struct{ Key string; Size int64; Modified time.Time }`
  - `func (c Client) List(ctx context.Context, prefix string) ([]Object, error)` — ListObjectsV2, follows continuation tokens, parsed with `encoding/xml`
  - `var ErrNotFound` for a 404 on Get

- [ ] **Step 1:** Move the SigV4 test; add `TestClientPutGetList` against an `httptest.Server` that checks the
  `Authorization` header starts `AWS4-HMAC-SHA256 Credential=<keyID>/` and the path is `/bucket/<key>`, stores PUT
  bodies, answers GET (404 → `ErrNotFound`) and a two-page ListObjectsV2 XML (`IsTruncated`, `NextContinuationToken`).
  Assert List returns both pages' keys with sizes and times.
- [ ] **Step 2:** `go test ./internal/s3/` → FAIL (package missing).
- [ ] **Step 3:** Implement. Per-request timeout via ctx is the caller's; `HTTP` nil → a client with 60 s timeout.
  Non-2xx → error with status + first 2 KB of body. Bootstrap's `r2.do` keeps its watchdog/progress and calls `s3.SignV4`.
- [ ] **Step 4:** `go test ./internal/s3/ ./cmd/bootstrap/` → PASS.
- [ ] **Step 5:** Commit `refactor(s3): shared SigV4 client (put / get / list)`.

### Task 2: Backup sealing (AES-256-GCM)

**Files:**
- Create: `internal/backup/seal.go`, `internal/backup/seal_test.go`

**Interfaces:**
- Produces:
  - `func ParseKey(hex string) ([]byte, error)` — exactly 32 bytes, else error
  - `func Seal(key, plain []byte) ([]byte, error)` — `"KBC1"` + 12-byte random nonce + GCM ciphertext
  - `func Open(key, sealed []byte) ([]byte, error)` — `ErrNotSealed` without the magic, auth failure → error
  - `const SealedSuffix = ".json.gz.enc"`

- [ ] **Step 1:** Tests: round-trip; wrong key → error; one flipped byte → error; `Open` of a plain gzip → `ErrNotSealed`;
  two `Seal`s of the same input differ; `ParseKey` rejects 63/65 hex chars and non-hex.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement (in memory; `// ponytail: whole file in memory — stream if backups pass a few hundred MB`).
- [ ] **Step 4:** Run → PASS.
- [ ] **Step 5:** Commit `feat(backup): seal / open backups (AES-256-GCM) for Karots cloud`.

### Task 3: `backup_copies` table, config, seeding

**Files:**
- Create: `migrations/00006_backup_copies.sql`, `internal/backup/copies.go`, `internal/backup/copies_db_test.go`
- Modify: `internal/backup/backup.go` (`skipTable`), `internal/config/config.go`, `AGENTS.md` (migrations line: next is 00007_)

**Interfaces:**
- Produces:
  - Table `backup_copies(kind text PRIMARY KEY CHECK (kind IN ('folder','cloud')), enabled bool NOT NULL, path text NOT NULL DEFAULT '', preset text NOT NULL DEFAULT '', last_copied_at timestamptz, last_file text NOT NULL DEFAULT '', last_error text NOT NULL DEFAULT '', last_tried_at timestamptz)`
  - `config.Config.BackupCopyDir string`; `config.Config.Cloud CloudBackup` with `type CloudBackup struct{ URL, KeyID, Secret, Folder, Crypt string; On bool }` and `func (c CloudBackup) Set() bool` (URL, KeyID, Secret, Folder, Crypt all non-empty)
  - `type Copy struct{ Kind string; Enabled bool; Path, Preset string; LastCopiedAt, LastTriedAt *time.Time; LastFile, LastError string }` (db tags)
  - `func SyncCopies(ctx context.Context, db *sqlx.DB, copyDir string, cloud config.CloudBackup) error` — start-up reconcile
  - `func Copies(ctx context.Context, db *sqlx.DB) ([]Copy, error)` — folder first, then cloud
  - `func SaveFolder(ctx, db, path string, enabled bool) error`, `func ClearFolder(ctx, db) error`, `func SetEnabled(ctx, db, kind string, on bool) error`

SyncCopies rules: folder preset non-empty and ≠ row's `preset` → upsert row (path = preset, enabled, preset = value);
same preset → leave alone (owner's edits/clear stay). Cloud `Set()` → insert row if missing with `enabled = On`;
not `Set()` → delete the cloud row.

- [ ] **Step 1:** DB tests (gated on `DATABASE_URL`): preset seeds once; owner clears → next Sync with same preset does
  not re-create; a different preset replaces the path; cloud row created on/off per `On`, kept when `On` flips later,
  removed when creds vanish; `skipTable("backup_copies")` true and a Dump contains no `backup_copies`.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Migration, config parsing (`CLOUD_BACKUP` = "on" → On), store functions, `skipTable`.
- [ ] **Step 4:** `go test ./internal/backup/ ./internal/config/` → PASS.
- [ ] **Step 5:** Commit `feat(backup): backup_copies table + presets from .env`.

### Task 4: The copy pass

**Files:**
- Create: `internal/backup/copies_run.go`, `internal/backup/copies_run_test.go`
- Modify: `internal/backup/scheduler.go` (call a hook after each successful backup), `cmd/server/main.go` (Sync at boot, start the copier when `BACKUP_DIR` is set)

**Interfaces:**
- Consumes: Task 1 `s3.Client`, Task 2 `Seal`, Task 3 store + config.
- Produces:
  - `type Copier struct{ DB *sqlx.DB; Dir string; Keep int; Cloud config.CloudBackup; S3 CloudStore; Now func() time.Time }`
  - `type CloudStore interface{ Put(ctx, key string, body []byte) error; Get(ctx, key string) ([]byte, error); List(ctx, prefix string) ([]s3.Object, error) }` (s3.Client satisfies it; tests fake it)
  - `func (c *Copier) RunOnce(ctx context.Context)` — one pass over enabled rows; `func (c *Copier) Run(ctx context.Context, kick <-chan struct{})` — pass on kick and every 30 min
  - `func (c *Copier) CopyNow(ctx context.Context, kind string) error` — one row now, returns its error (Data page)
  - `func (c *Copier) TestFolder(path string) error` — create if parent exists, write+remove `karots-test.tmp`
  - `RunScheduler` gains `after func()` param, called after a successful snapshot (non-blocking send to `kick`)

Folder pass: path missing → status "not there right now: <path>" (do not create). Else list `*.json.gz` there, copy each
local file it lacks oldest-first (`.tmp` → rename, keep mtime via `os.Chtimes`), then `Rotate(path, Keep)`. Cloud pass:
if `LastCopiedAt == nil || now-LastCopiedAt > 20h || LastCopiedAt after now`: newest local file → `Seal` → `Put`
`Folder/<mtime yyyymmdd-hhmmss>.json.gz.enc`. Success → `last_copied_at`, `last_file`, clear `last_error`; every try → `last_tried_at`.

- [ ] **Step 1:** Tests: folder catch-up copies 3 missing of 4 in mtime order and prunes to Keep; missing path → error
  stored, no folder created, local files untouched; a stray `x.json.gz.tmp` at the place is ignored by list and Rotate;
  cloud uploads once, second pass within 20 h does nothing, pass after 21 h uploads the newest, `LastCopiedAt` 2 h in
  the future → uploads; uploaded body opens with the key; key name has no shop slug; a failing fake store stores
  `last_error` and `RunOnce` returns normally; disabled rows untouched.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement; boot: `SyncCopies` then `go copier.Run(bgCtx, kick)`; log one line per pass result.
- [ ] **Step 4:** `go test ./internal/backup/` → PASS.
- [ ] **Step 5:** Commit `feat(backup): copy pass — folder catch-up + daily Karots cloud`.

### Task 5: Data API for copies + dashboard warning

**Files:**
- Create: `internal/api/admin_copies.go`, `internal/api/admin_copies_test.go`
- Modify: `internal/api/admin_data.go` (register), `internal/api/dto_data.go`, `internal/api/dto_reports_ops.go` + `admin_dashboard.go` (attention), `internal/app/services.go` (pass the Copier), `web/src/api/types.gen.ts` via `make types`

**Interfaces:**
- Consumes: Task 3/4.
- Produces (all under `/api/v1/admin/data`, admin role):
  - `GET /copies` → `[]BackupCopyRow{ kind, enabled, path, lastCopiedAt?, lastTriedAt?, lastFile, lastError, waiting int }` (`waiting` = local files the folder lacks; cloud: 1 if due, else 0)
  - `PUT /copies/folder` `{path, enabled}` → validates absolute path, refuses the backups folder or inside it (`BACKUP_FOLDER_IS_COPY` → 400 "Pick a different folder — that is where the backups already are"), runs `TestFolder`, saves
  - `DELETE /copies/folder`
  - `PUT /copies/cloud` `{enabled}` → `enabled:true` from a non-system user = 403; 404 when no cloud row
  - `POST /copies/:kind/run` → `CopyNow`, returns the row
  - `GET /copies/:kind/backups` → `[]BackupFileRow` (folder: `ListBackups(path)`; cloud: `List(Folder+"/")`, names without prefix)
  - `POST /copies/:kind/backups/:name/fetch` → name must be in that list; folder: copy into `BackupFolder()`; cloud: `Get` + `Open` → write `cloud-<name minus .enc>`; `Chtimes` to the listed time; returns `{name}` of the local file. The SPA then calls the existing `POST /backups/:name/restore`.
  - `DashboardAttention.BackupCopies []string` — "USB folder" / "Karots cloud" for enabled rows with no copy in 24 h (or never and the row is > 24 h old — use `last_tried_at` presence)
- Audit: folder saved/cleared, cloud switched, restore-from-copy fetched (existing `audit` service).

- [ ] **Step 1:** Handler tests: copy folder = backups folder → 400; inside it → 400; admin enabling cloud → 403,
  system user → 200, admin disabling → 200; fetch `../x.json.gz` and an unlisted name → 400; fetch from a folder
  place keeps the source mtime; attention lists a 25-h-stale enabled folder and not a disabled one.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement; `make types`.
- [ ] **Step 4:** `go test ./internal/api/ -run Copies` and `go test ./internal/api/ -run Dashboard` → PASS.
- [ ] **Step 5:** Commit `feat(api): backup copies on the Data page API + dashboard warning`.

### Task 6: Data page card + dashboard item (UI gate)

**Files:**
- Create: `web/src/features/data/CopiesCard.tsx` (+ `CopyBackupsSheet.tsx` if over ~250 lines)
- Modify: `web/src/features/data/DataPage.tsx` (new `Card title="Backup copies"` after Automatic backups, `lg:col-span-2`), `web/src/features/dashboard/Dashboard.tsx` (attention item, tone `warn`, links to `/admin/data`), `web/e2e/data-copies.spec.ts`

**Interfaces:**
- Consumes: Task 5 endpoints/types.

Rows: **Copy folder** — path (or "Not set — add a USB stick or network folder"), status line ("Last copied 10 min ago",
"Not there right now — plug the USB in", "Failed: …", "N waiting"), switch, Copy now, Change folder (FormSheet: path +
Save which tests), Clear (ConfirmButton), Backups here…. **Karots cloud** — only if present: status, switch (admin:
switch disabled when off with "Ask Karots to switch it on"; system user: enabled), Copy now, Backups here…. Backups
here… = PickerShell list, newest first, Restore → fetch then the existing RestoreDialog flow.

- [ ] **Step 1:** Playwright `data-copies.spec.ts` (import `test` from `e2e/fixtures.ts`): admin sets the copy folder to
  a temp dir → Save → Copy now → status "Last copied" and the dir holds a `.json.gz`; setting it to the backups folder
  shows the refusal; Backups here… lists the file. Unit test (Vitest) for the status-line wording function.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement with the admin kit and theme tokens.
- [ ] **Step 4:** `make test-web`, `make build`, the spec → PASS. **Gate:** screenshots on :3100 at 360px and desktop,
  Studio + Night, light + dark, with a stale-copy warning on the dashboard; review, fix, restore Studio.
- [ ] **Step 5:** Commit `feat(data): Backup copies card + dashboard warning`.

### Task 7: Shop Builder — copy folder preset, Karots cloud, open a cloud copy

**Files:**
- Modify: `cmd/bootstrap/profiles.go` (+ test), `cmd/bootstrap/app_shops.go`, `cmd/bootstrap/app.html` / `app.js`
- Create: `cmd/bootstrap/cloudbackups.go` (+ test)
- Modify: `cmd/setup/installed.go` (`builderOwned` += the six `CLOUD_BACKUP*` keys; `pcOwned` += `BACKUP_COPY_DIR`)

**Interfaces:**
- Consumes: Task 1 `s3.Client`, Task 2 `Open`.
- Produces:
  - `ShopBackups.CopyDir string \`json:"copyDir,omitempty"\``, `ShopBackups.Cloud bool \`json:"cloud,omitempty"\``, `Shop.CloudKey string \`json:"cloudKey,omitempty"\`` (made with `randHex(32)` on first tick, never rotated, masked in `Masked()`)
  - `Conf()` writes `BACKUP_COPY_DIR` when set; when `Cloud`: `CLOUD_BACKUP_URL/KEY_ID/SECRET` from the Builder's `.env` (`CLOUD_BACKUP_URL`, `CLOUD_BACKUP_KEY_ID`, `CLOUD_BACKUP_SECRET`), `CLOUD_BACKUP_FOLDER=<shop key>`, `CLOUD_BACKUP_CRYPT=<CloudKey>`, `CLOUD_BACKUP=on`. `Check()` errors when `Cloud` is ticked and the Builder's `.env` lacks them.
  - `refusedEnv` += `BACKUP_COPY_DIR`, `CLOUD_BACKUP*`.
  - Builder page: shop form fields "Also copy backups to" and "Karots cloud backup" tick; shop row action **Cloud backups** → list (newest first) → Download saves an opened `.json.gz` into `dist/<key>/`.

- [ ] **Step 1:** Tests: `Conf()` env with/without cloud; first tick makes a 64-hex key, a second save keeps it; `Masked()`
  hides it; `Check()` refuses cloud without Builder creds; `refusedEnv` refuses `CLOUD_BACKUP_SECRET` in extra env;
  Setup `applyInstalled`: an Update whose install.conf lacks `CLOUD_BACKUP*` drops them from the PC's `.env`, keeps
  `BACKUP_COPY_DIR`; cloud download handler with a fake store writes an openable file.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `go test ./cmd/bootstrap/ ./cmd/setup/` → PASS; open the Builder page and check the form (screenshot).
- [ ] **Step 5:** Commit `feat(builder): copy folder preset + Karots cloud backup per shop`.

### Task 8: Setup — copy folder choice, Bring back from the copy folder and Karots cloud

**Files:**
- Modify: `cmd/setup/main.go` (`Choice.CopyDir` → `BACKUP_COPY_DIR`, abs-path check, refuse == backup dir), `cmd/setup/restore.go` (`findBackups`, cloud fetch), `cmd/setup/page.go`, `cmd/setup/page.html`, tests in `restore_test.go` / `steps_test.go`

**Interfaces:**
- Consumes: Task 1, Task 2; install.conf env.
- Produces:
  - `foundBackup.Place string` — `""` (Karots backups / beside Setup), `"copy"`, `"cloud"`
  - `findBackups` adds `BACKUP_COPY_DIR`'s files (if the folder exists) and, with `CLOUD_BACKUP_*` in `j.Conf.Env`, the cloud list (10 s timeout; failure → `j.note("Karots cloud: couldn't reach it — <err>")`, list without it)
  - `Choice.Restore = "cloud:<name>"` → before the restore step, `Get` + `Open` into `keepDir()/cloud-<name minus .enc>`, then the existing restore runs on that path
  - Page: "Also copy backups to" field (+ Choose…) beside "Backups go to"; Bring back list shows a place label ("USB / copy folder", "Karots cloud")

- [ ] **Step 1:** Tests: copy folder equal to backup dir → error; `findBackups` with a temp copy dir lists its files as
  `copy`; with a fake cloud (httptest) lists `cloud` rows; unreachable cloud → no rows + a note; `cloud:` restore writes
  a plain file into keepDir and sets `j.Restore` to it; `cloud:../x` refused.
- [ ] **Step 2:** Run → FAIL.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** `go test ./cmd/setup/` → PASS; `karots-setup -dry` beside an install.conf with a copy dir: look at the page.
- [ ] **Step 5:** Commit `feat(setup): copy folder choice + bring back from copy folder / Karots cloud`.

### Task 9: Bucket setup doc, full checks, status

**Files:**
- Create: `docs/cloud-backup-bucket.md`
- Modify: `docs/status.md`, `AGENTS.md` (Dev workflow: Builder `.env` `CLOUD_BACKUP_*`; one line on copies), `docs/ideas/backup-copies.md` (point to the spec: narrowed)

- [ ] **Step 1:** Write the bucket doc: create bucket `karots-suite-backups`; R2 API token "Object Read & Write" scoped to
  that bucket only (→ `CLOUD_BACKUP_KEY_ID/SECRET`); lifecycle rule delete after 30 days; bucket lock rule 30 days on
  the whole bucket (`wrangler r2 bucket lock add karots-suite-backups --name keep-30 --retention-days 30`); free-plan
  numbers (10 GB, 1 M Class A, 10 M Class B); a leaked key → new token, Builder `.env`, Release all; stopping a shop →
  untick, Release, shop runs Update.
- [ ] **Step 2:** `make verify`, `go test -p 1 ./...` core-only and with `make plugins P=all` (then `make core-only`),
  `make test-web`, `make verify-portable`, full Playwright (nothing else on 3100/5173) → all green.
- [ ] **Step 3:** Update `docs/status.md` (M8c2 built; user's VM acceptance next: copy folder on a USB, pull it, plug it
  back → catch-up; cloud on a test shop → Bring back a shop on a fresh VM from the cloud).
- [ ] **Step 4:** Commit `docs: M8c2 backup copies — bucket setup, status`.
