# M8c Self-update + Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A shop binary checks its own folder in the `karots-suite` R2 bucket, shows what's new on the Data page and
updates on **Update**; the Shop Builder makes **Test builds** (local only) and **Releases** (build + publish).

**Architecture:** New `internal/update` (forked from karots-pos `internal/update`: signed manifest, gzip download,
swap, BootGuard) — restart is "exit 75, the supervisor starts the new binary". The server stamps
`X-Karots-Version` on every response; the SPA reloads when it changes (till: only with an empty cart).
`cmd/bootstrap` gains `release.go` (key, version, publish) and `r2.go` (forked S3 PUT/DELETE), two buttons per shop row.

**Tech Stack:** Go stdlib (`crypto/ed25519`, `compress/gzip`, `net/http`), Echo, React + TanStack Query, Vitest.
No new dependencies.

**Spec:** `docs/specs/2026-10-19-m8c-self-update-design.md`

## Global Constraints

- Bucket `karots-suite`; shops read `UPDATE_BASE_URL=https://pub-e1fb05003fb841199f70633558a66fd2.r2.dev` directly. No Worker, no running-version tracking.
- Builder `.env`: `SUITE_RELEASE_KEY` (base64 ed25519 seed), `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET=karots-suite`, `UPDATE_BASE_URL`.
- Signed message: `"karots-suite update\n" + version + "\n" + sha256hex` — never karots-pos's prefix.
- Version `YYYY.MM.DD.HHMM` (`time.Now().Format("2006.01.02.1504")`); `""`/`dev` older than any release.
- Baked with `-X`: `main.version`, `main.buildKind` (`test` | `release`), `main.updateBaseURL`, `main.updateChannel`, `main.updatePubKey`. No channel → no updater.
- Restart exit code **75** (`update.RestartCode`). Checks: at start, every **6 h**, and on Data page open / Check now.
- The shop **never installs by itself**. Keep the last **3** binaries per channel; changelog keeps **10** entries.
- Upload order: `<file>.gz` first, `manifest.json` last with `Cache-Control: no-store`.
- The channel never appears in a page, log line or error text (wrap HTTP errors so the URL is dropped).
- Release refuses a dirty tree, ignoring `shops.json`, `cmd/server/enabled_plugins.go`, `web/src/plugins/enabled-plugins.ts`. Test build skips the check.
- Core stays plugin-inert; files ≤ 500 lines; no Claude/AI attribution in commits; `make types` after `dto.go` changes.

## Review Focus

1. **Slow / dropped line mid-download** — stall watchdog (2 min) gives up, nothing swapped, card says "Update failed, nothing changed: …". Test in Task 2.
2. **Manifest signed by another key or tampered file** — Check reports state `bad-signature` (a warning), Apply refuses a SHA mismatch and leaves the exe untouched. Test in Tasks 1 and 2.
3. **R2 not set up / offline during Release** — build kept, `published` unchanged, row red "Not published — R2 not set up" / the error, **Publish** retries the same files with the same version. Test in Task 6.
4. **Till mid-sale while the server restarts** — `X-Karots-Version` change with a non-empty cart does not reload; reload happens once the cart is empty. Test in Task 8.
5. **New binary crashes on boot twice** — BootGuard puts `.old` back and the Data page shows "The update to X couldn't start; Karots went back to Y" until dismissed. Test in Task 2.

---

### Task 1: `internal/update` — manifest, versions, check

**Files:**
- Create: `internal/update/manifest.go`, `internal/update/check.go`
- Test: `internal/update/manifest_test.go`, `internal/update/check_test.go`

Fork from karots-pos `internal/update/update.go` (read it first); change the signed prefix and add states.

**Interfaces:**
- Produces:
  - `type Manifest struct { Version, File, SHA256, Sig, Notes string; Changelog []Change; Commit string; History []string }` (JSON names as karots-pos)
  - `type Change struct{ Version, Notes string }`
  - `func SignedMessage(version, sha string) []byte`
  - `func (m *Manifest) Verify(pub ed25519.PublicKey) error`
  - `func (m *Manifest) NotesSince(current string) string`
  - `func Newer(a, b string) bool`
  - `const RestartCode = 75`
  - `type State string` = `"unchecked" | "up-to-date" | "available" | "not-published" | "offline" | "bad-signature" | "installing" | "failed"`
  - `type Status struct { Current, Kind, Latest, Notes string; State State; CheckedAt time.Time; Message, Progress, RolledBackFrom string }`
  - `type Updater struct { Current, Kind, BaseURL, Channel string; PubKey ed25519.PublicKey; Exe string; Backup func(ctx context.Context, version string) error; Restart func(); Client *http.Client; … }`
  - `func (u *Updater) Check(ctx context.Context) error`, `func (u *Updater) Status() Status`, `func (u *Updater) Run(ctx context.Context, every time.Duration)`

- [ ] **Step 1: Write failing tests**
  - `TestVerify`: sign with a fresh key → nil; flip a byte of SHA256 → error; a signature over `"karots-pos update\n…"` → error.
  - `TestNewer`: `Newer("2026.10.19.1530","2026.10.19.1529")`, `Newer("2026.10.19.1530","dev")`, `Newer("2026.10.19.1530","")` true; `Newer("dev","2026.01.01.0000")`, equal versions false.
  - `TestNotesSince`: changelog `[c3,c2,c1]`, current = c1 → c3 and c2 joined under their versions; nothing newer → `m.Notes`.
  - `TestCheckStates` with `httptest.Server`: 200 newer signed → `available` with Latest/Notes; 200 same version → `up-to-date`; 404 → `not-published`, Message `"No update published for this shop yet"`; wrong key → `bad-signature`; server closed → `offline`. Assert `Status().Message` never contains the channel string.
- [ ] **Step 2: Run** `go test ./internal/update/` → FAIL (package missing).
- [ ] **Step 3: Implement.** Client = karots-pos `ipv4First()` transport, no overall timeout, manifest request bounded at 1 min, body capped at 1 MiB. `offline` message: `"Couldn't reach the update server — last checked <time>"`; errors from `http.Client.Do` are reduced to their kind (no URL).
- [ ] **Step 4: Run** `go test ./internal/update/` → PASS.
- [ ] **Step 5: Commit** `feat(update): signed manifest + check with not-published / offline / bad-signature states`

### Task 2: `internal/update` — apply, swap, BootGuard

**Files:**
- Create: `internal/update/apply.go`, `internal/update/bootguard.go`
- Test: `internal/update/apply_test.go`, `internal/update/bootguard_test.go`

**Interfaces:**
- Consumes: Task 1 `Updater`, `Manifest`.
- Produces:
  - `func (u *Updater) Apply(ctx context.Context) error` — starts in the background, returns at once; errors only for "no update available — check first" / "an update is already being installed".
  - `func Swap(exe, newPath string) error` (keeps `exe+".old"`)
  - `func BootGuard(exe string) (rolledBack bool, err error)`, `func MarkHealthy(exe string)`
  - `func (u *Updater) Dismiss()` — clears `RolledBackFrom` (removes `exe+".rolledback"`).

- [ ] **Step 1: Write failing tests**
  - `TestApplyHappy`: fake server serves a gzipped file whose SHA matches → `Backup` called with the new version, then exe content replaced, `.old` holds the previous bytes, `Restart` called once; Progress passed through `"Downloading… N%"`, `"Backing up…"`, `"Restarting…"`.
  - `TestApplyBadHash`: wrong SHA → exe untouched, no `.old`, State `failed`, Message starts `"Update failed, nothing changed:"`, Restart not called.
  - `TestApplyStall`: server writes half then hangs; set package var `stallLimit = 200*time.Millisecond` in the test → `failed`, exe untouched.
  - `TestApplyBackupFails`: Backup returns error → nothing swapped, `failed`.
  - `TestBootGuard`: marker absent → false; after `Swap` (writes marker `0`) three `BootGuard` calls: false, false, then true with exe = old bytes and `exe+".rolledback"` holding the failed version; `MarkHealthy` removes the marker. Status after a rollback has `RolledBackFrom` set; `Dismiss` clears it.
- [ ] **Step 2: Run** `go test ./internal/update/` → FAIL.
- [ ] **Step 3: Implement** as karots-pos `apply` / `download` / `Swap` / `BootGuard` (read `relaunch_*.go` but do **not** port Relaunch — restart is the caller's `Restart`). Order: download to `exe+".new"` → verify SHA → `Backup` → `Swap` → `Restart`. The marker holds the new version on its first line so BootGuard can write `.rolledback`.
- [ ] **Step 4: Run** `go test ./internal/update/` → PASS.
- [ ] **Step 5: Commit** `feat(update): download, verify, swap and BootGuard rollback`

### Task 3: Server wiring + `X-Karots-Version`

**Files:**
- Create: `cmd/server/update.go`
- Modify: `cmd/server/main.go`, `internal/config/config.go` (`Version string`), `internal/app/app.go`
- Test: `internal/app/version_test.go`

**Interfaces:**
- Consumes: Tasks 1–2.
- Produces: `var version, buildKind, updateBaseURL, updateChannel, updatePubKey string` in `cmd/server`; `cfg.Version` (`"dev"` when empty); `func startUpdater(ctx context.Context, db *sqlx.DB, cfg *config.Config, restart chan<- struct{}) *update.Updater` (nil without a channel); `api.Deps.Update *update.Updater`.

- [ ] **Step 1: Write failing test** `TestVersionHeader`: `BuildServer` with `cfg.Version = "2026.10.19.1530"` → `GET /api/v1/health` (or any route) carries `X-Karots-Version: 2026.10.19.1530`; also on a 401 response.
- [ ] **Step 2: Run** `go test ./internal/app/ -run TestVersionHeader` → FAIL.
- [ ] **Step 3: Implement.** Middleware in `BuildServer` setting the header before the handler (and `Access-Control-Expose-Headers` not needed — same origin). In `main`: `update.BootGuard(exe)` first thing (on rollback: log and `os.Exit(update.RestartCode)` so the supervisor starts the old binary); `startUpdater` when `updateChannel != ""` — `Backup` = `backup.WriteSnapshot(ctx, db, dir, time.Now(), "before-update-"+v)` with `dir = cfg.BackupDir`, or `<exe dir>/backups` when empty (file mode: `cfg.FileDataDir/backups`); `Restart` sends on `restart`; `go u.Run(ctx, 6*time.Hour)`; `time.AfterFunc(2*time.Minute, func(){ update.MarkHealthy(exe) })`. The shutdown wait becomes `select { case <-quit: case <-restart: code = update.RestartCode }`, graceful shutdown as today (file mode stops PostgreSQL via the deferred Stop — so exit through a function return, then `os.Exit(code)` from a wrapper), then exit with `code`. Log the version at start.
- [ ] **Step 4: Run** `go test ./internal/app/ ./cmd/server/` → PASS; `go build ./...`.
- [ ] **Step 5: Commit** `feat(server): updater wiring, BootGuard at start, restart exit code, X-Karots-Version`

### Task 4: Update API

**Files:**
- Modify: `internal/api/dto_data.go`, `internal/api/api.go` (Deps + handlers field), `internal/api/admin_data.go` (routes)
- Create: `internal/api/admin_update.go`
- Test: `internal/api/admin_update_test.go`

**Interfaces:**
- Produces (camelCase JSON):
  - `type UpdateStatusDTO struct { Version, Kind string; Enabled bool; State, Latest, Notes, Message, Progress, RolledBackFrom string; CheckedAt *time.Time }`
  - `GET /api/v1/admin/data/update` → status (`Enabled:false`, `Version`, `Kind` when no updater)
  - `POST /api/v1/admin/data/update/check` → status after `Check`
  - `POST /api/v1/admin/data/update/apply` → 202 + status; `apperr.Validation` with the Apply error text
  - `POST /api/v1/admin/data/update/dismiss` → status
- Admin role only (the `/data` group already is). The system user passes as admin — confirm with an existing test helper.

- [ ] **Step 1: Write failing tests**: no updater → `enabled:false`, version `dev`; with an updater on a fake server → check gives `available`; apply without check → 422 `"no update available — check first"`; cashier / manager → 403.
- [ ] **Step 2: Run** `go test ./internal/api/ -run Update` → FAIL.
- [ ] **Step 3: Implement**; `make types`.
- [ ] **Step 4: Run** `go test ./internal/api/ -run Update` → PASS.
- [ ] **Step 5: Commit** `feat(api): admin update status / check / apply / dismiss`

### Task 5: Windows launcher restarts on code 75

**Files:**
- Modify: `cmd/setup/hidden.go`
- Test: `cmd/setup/hidden_test.go`

**Interfaces:** `func runChild(start func() (int, error)) int` — loops while the exit code is `update.RestartCode`, returns any other code. `runHidden` calls it.

- [ ] **Step 1: Write failing test** `TestRunChildRestarts`: fake `start` returning 75, 75, 0 → called 3 times, returns 0; returning 1 → called once, returns 1.
- [ ] **Step 2: Run** `go test ./cmd/setup/ -run RunChild` → FAIL.
- [ ] **Step 3: Implement** (the log file stays open across restarts). Linux needs nothing: the unit has `Restart=always` and Setup already `chown -R`s the install folder to the service user, so the swap can rename the binary — add one line to the Setup test asserting the chown stays.
- [ ] **Step 4: Run** `go test ./cmd/setup/` → PASS.
- [ ] **Step 5: Commit** `feat(setup): karots-start restarts Karots after an update (exit 75)`

### Task 6: Builder — release key, R2, publish

**Files:**
- Create: `cmd/bootstrap/release.go`, `cmd/bootstrap/r2.go` (fork karots-pos `cmd/release/r2.go`), `cmd/bootstrap/notes.go` (fork `cmd/release/notes.go`)
- Modify: `cmd/bootstrap/profiles.go` (`Built.Version`, `Built.Kind`, `Shop.Published *Published{Version, Commit string; At time.Time}`), `cmd/bootstrap/shopbuild.go` (`shopBuildOpts.Version`, `.Kind`; bake the five `-X` values via `ExtraX`)
- Test: `cmd/bootstrap/release_test.go`, `cmd/bootstrap/r2_test.go` (forked)

**Interfaces:**
- Produces:
  - `type store interface { put(key, path, contentType, cacheControl string) error; delete(key string) error }` (`*r2` and a test fake)
  - `func releaseKey(envPath string) (ed25519.PrivateKey, string, error)` — reads `SUITE_RELEASE_KEY`; missing → generates, appends to `.env`, returns; the log line says "new release key written to .env — keep it: shops only take updates signed by it".
  - `func versionAt(t time.Time) string`
  - `func treeDirty() (bool, error)` — ignores the three Builder-written paths.
  - `func publish(s *Shop, dir, version, notes, head, baseURL string, st store, priv ed25519.PrivateKey, log io.Writer) error` — fetch previous manifest (404 → none), refuse a version not newer, gzip, sign, history (3) + changelog (10), upload gz then manifest, prune, set `s.Published`.
  - `func defaultNotes(s Shop, all []manifest) string` — `changesSince(s.Published.Commit, …)`, "" when never published.

- [ ] **Step 1: Write failing tests**
  - `TestPublishOrder`: fake store records calls → `<channel>/karots-suite-<v>.gz` before `<channel>/manifest.json` (cache `no-store`); manifest verifies with the public key; `s.Published.Version == v`.
  - `TestPublishPrunes`: previous manifest history of 3 → 4th oldest deleted, history length 3.
  - `TestPublishNotNewer`: previous version equal → error `"version … is not newer than the published …"`, nothing uploaded.
  - `TestReleaseKeyGenerates`: temp `.env` without the key → key appended, a second call returns the same public key.
  - `TestTreeDirtyIgnoresBuilderFiles` (temp git repo): only `shops.json` changed → false; another tracked file → true.
  - `TestShopBuildBakesVersion`: `buildShop` with a fake `b` → `ExtraX` contains `main.version=<v> main.buildKind=test main.updateBaseURL=… main.updateChannel=<channel> main.updatePubKey=…`.
- [ ] **Step 2: Run** `go test ./cmd/bootstrap/` → FAIL.
- [ ] **Step 3: Implement.** R2 missing → `r2FromEnv` returns `errR2NotSetUp` ("R2 not set up — add R2_* to .env").
- [ ] **Step 4: Run** `go test ./cmd/bootstrap/` → PASS.
- [ ] **Step 5: Commit** `feat(bootstrap): release key, R2 publish, version stamping`

### Task 7: Builder — Test build / Release / Publish / Release all

**Files:**
- Modify: `cmd/bootstrap/app_shops.go` (split if it passes 500 lines: `app_release.go`), `cmd/bootstrap/app.js`, `cmd/bootstrap/app.html`, `cmd/bootstrap/shopbuild.go` (`buildShops` CLI: `-shop` = Release; new `-test` flag = Test build), `cmd/bootstrap/main.go`
- Test: `cmd/bootstrap/app_shops_test.go`

**Interfaces:**
- Endpoints: `POST /api/shops/{key}/build` (`{installer}`) = **Test build** (kind `test`, no upload); `GET /api/shops/{key}/notes` → `{notes}`; `POST /api/shops/{key}/release` (`{installer, notes}`) = clean-tree check → build (kind `release`) → publish; `POST /api/shops/{key}/publish` = publish the files already in `dist/<key>/` with `LastBuilt.Version` (only when `LastBuilt.Kind == "release"` and not yet published); `POST /api/shops/release-all` (`{installer, notes}`) — one version for every shop.
- Saving: `saveBuilt` also stores `Published`.

- [ ] **Step 1: Write failing tests**: release with a fake store → `published` saved; release with `errR2NotSetUp` → build kept, `lastBuilt.kind=release`, `published` unchanged, streamed error says "R2 not set up"; publish afterwards with a working fake → `published.version == lastBuilt.version`; test build on a never-published shop → streamed log contains "This shop has no release yet — the first install at a shop should be a Release"; release with a dirty tree → 409 "commit first …".
- [ ] **Step 2: Run** `go test ./cmd/bootstrap/` → FAIL.
- [ ] **Step 3: Implement** the endpoints and the row: buttons **Test build**, **Release…** (dialog: notes textarea pre-filled from `/notes`, Release / Cancel), **Publish** only when not published; status line: `Published <version> · <at>` or red **Not published**, and amber `Test build <version>` when newer than published; header **Release all…** beside Build all.
- [ ] **Step 4: Run** `go test ./cmd/bootstrap/` → PASS; open the Builder (`make bootstrap`), screenshot a shop row in all three states, check it.
- [ ] **Step 5: Commit** `feat(bootstrap): Test build and Release per shop, Publish retry, Release all`

### Task 8: SPA — Updates card + reload on new version

**Files:**
- Create: `web/src/features/data/UpdateCard.tsx`, `web/src/lib/version.ts`, `web/src/lib/version.test.ts`
- Modify: `web/src/features/data/queries.ts`, `web/src/features/data/DataPage.tsx`, `web/src/api/client.ts`, `web/src/features/till/TillProvider.tsx`

**Interfaces:**
- `version.ts`: `export function noteVersion(v: string | null): void` (client.ts calls it with each response's `X-Karots-Version`); `export function setReloadGuard(fn: () => boolean): () => void` (returns unregister); `export function shouldReload(seen: string | null, now: string | null, guard: () => boolean): boolean`. First version seen is remembered; a different one → reload when the guard allows, else retry each time `noteVersion` runs and every 5 s.
- TillProvider registers the guard `() => state.lines.length === 0` (use the real cart field name).

- [ ] **Step 1: Write failing Vitest** `version.test.ts`: first value → no reload; same → no; changed with guard true → reload; changed with guard false → no, then guard true on the next tick → reload; null header → ignored.
- [ ] **Step 2: Run** `cd web && bunx vitest run src/lib/version.test.ts` → FAIL.
- [ ] **Step 3: Implement**; the admin side reloads at once after a 1.5 s banner "Karots was updated — reloading" (no toast). `UpdateCard` (first card on the Data page, `lg:col-span-2`, theme tokens only, works at 360px): states from Task 4 — Up to date · Update available (version, what's new as pre-wrapped text, **Update** with confirm "Tills will pause for about a minute while Karots restarts. Sales already rung up are safe.") · No update published for this shop yet · Couldn't check (quiet) · bad signature (warning) · Installing… (progress, polls every 2 s) · Rolled back (+ Dismiss) · no updater (version only; `Test build` badge when `kind === 'test'`). **Check now** button; check on mount.
- [ ] **Step 4: Run** `make test-web` → PASS; screenshots at 360 px and desktop in Studio + Night, light + dark; self-review.
- [ ] **Step 5: Commit** `feat(web): Updates card on Data, reload after an update (till waits for an empty cart)`

### Task 9: Acceptance + docs

**Files:**
- Modify: `AGENTS.md` (layout: `internal/update`; Dev workflow: Test build vs Release, `.env` keys), `docs/status.md`, `deploy/docker` README note ("no self-update: rebuild the image")
- Create: `e2e/update.spec.ts` only if the card can be driven without a real bucket — otherwise skip (Go tests cover it).

- [ ] **Step 1:** `make test` (`go test -p 1 ./...`, core-only and `make plugins P=all`), `make test-web`, `make verify`, `make verify-portable`, full Playwright → all pass.
- [ ] **Step 2: Bucket ready** (user): bucket `karots-suite` public at the r2.dev URL, R2 token in `.env`.
- [ ] **Step 3: Acceptance (user + me):** Release the test shop → install the three files by hand → Data page **Up to date**. Release again after any commit (M8c2 or a trivial change) → test PC: **Check now** → **Update available** with notes → **Update** → back on the new version, sales intact, till reloaded only after its cart emptied.
- [ ] **Step 4: Commit** `docs: M8c self-update + release done`
