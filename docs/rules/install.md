# Installing and running a shop: backups, copies, updates, Setup, the Shop Builder

Code: `internal/backup` (backup.go: dump / restore; scheduler.go: automatic backups and rotation; files.go: the
list and safe names; restore_rows.go: old / new columns, sequences; buckets.go: partial and setup-only exports;
copies*.go: the copy folder and Karots cloud; seal.go), `internal/api/admin_data.go` + `admin_copies.go` (the Data
page), `internal/update` (manifest.go: signature, versions; check.go; apply.go: download, backup, swap;
bootguard.go), `cmd/server` (backupcli.go: `-backup` / `-restore` for Setup; update.go; filemode.go),
`cmd/setup` (steps.go: the install; uninstall.go; restore.go: keep dir, program backup / restore, stop the till),
`internal/filedb`, `cmd/bootstrap` (build.go, shopbuild.go, release.go, app_release.go).

Nothing here moves money or stock. What it must never do is **lose, change or invent the shop's data**, leave a
shop with a program that won't start, or install something Karots didn't sign. So the "auditor" for this area is the
**round trip**: data → backup → restore → the same data, and `ledgercheck` still passing.

## The words the rules use

- **Automatic backup** — the scheduler's snapshot in `BACKUP_DIR` (every `BACKUP_INTERVAL`, newest `BACKUP_KEEP`).
- **Safety copy** — a backup taken just before something replaces data or the program: `before-restore`,
  `before-update-<version>`, `before-move`, `final` (uninstall).
- **Copy place** — the owner's copy folder (USB / network) or Karots cloud (sealed, paying shops).
- **Swap** — the running program moved to `<exe>.old`, the new one put in its place.
- **Keep dir** — the Karots backups folder outside the program folder (`C:\Karots backups`, `~/Karots backups`).

## The grid

Columns: **Do** · **While selling** (tills keep working beside it) · **Cut** (power cut / the program killed part
way) · **Twice** (at once, or run again) · **Bad input** (damaged file, wrong key, bad signature, bad name) ·
**Versions** (older / newer build or schema) · **Trouble** (disk full, folder gone, offline).

| Feature | Do | While selling | Cut | Twice | Bad input | Versions | Trouble |
|---|---|---|---|---|---|---|---|
| Automatic backup | BAK-1,2 | BAK-1 | BAK-3 | BAK-4 | — a | BAK-7 | BAK-12 |
| Download a backup (full, plugin groups, setup only) | BAK-2,10 | BAK-1 | — b | — c | BAK-11 | BAK-7 | — b |
| Restore (Data page, Setup `-restore`) | BAK-5 | BAK-6 | BAK-5 | BAK-6 | BAK-5,11 | BAK-7 | BAK-6 |
| Secrets, sessions after a restore | BAK-8,9 | BAK-9 | — c | — c | — c | BAK-8 | — c |
| Copy folder | CPY-1 | CPY-6 | CPY-1 | CPY-2 | CPY-5 | — d | CPY-1,6 |
| Karots cloud | CPY-3 | CPY-6 | CPY-3 | CPY-2,3 | CPY-4 | — d | CPY-6 |
| Fetch from a copy place | CPY-5 | BAK-6 | CPY-1 | CPY-2 | CPY-4,5 | BAK-7 | CPY-6 |
| Update: check | UPD-1,2 | — c | — c | — c | UPD-1 | UPD-2 | UPD-7 |
| Update: install | UPD-3 | UPD-8 | UPD-4 | UPD-7 | UPD-1 | UPD-6 | UPD-3,7 |
| BootGuard (went back) | UPD-5 | — c | UPD-5 | UPD-5 | — c | UPD-6 | — c |
| Setup: install (main / cashier PC) | SET-1,2 | — e | SET-2 | SET-2 | SET-1 | — d | SET-2 |
| Setup: Update | SET-3 | — e | SET-2 | SET-2 | SET-1 | SET-3 | SET-2 |
| Setup: mode change (file ⇄ server) | SET-4 | — e | SET-4 | SET-4 | — c | BAK-7 | SET-4 |
| Setup: Uninstall | SET-5 | — e | SET-5 | SET-2 | SET-5 | — d | SET-5 |
| Setup: Bring back a shop | SET-6 | — e | BAK-5 | SET-6 | BAK-5 | BAK-7 | SET-6 |
| Terms, Setup's log | SET-7,8 | — c | — c | — c | — c | — c | — c |
| File mode | SET-9 | — c | SET-9 | SET-9 | SET-9 | SET-9 | SET-9 |
| Shop Builder: build | BLD-1,2,7 | — c | BLD-1 | — c | BLD-2 | — d | BLD-1 |
| Shop Builder: Release / Publish | BLD-3,4,6 | — c | BLD-3,5 | BLD-3 | BLD-4,6 | BLD-3 | BLD-5 |

a — nothing to put in. b — streamed: a broken download is a broken file the restore refuses (BAK-5). c — doesn't
apply (a read, a one-shot, or no state). d — the same build on both sides. e — Setup stops the till first.

## Rules

```
BAK-1  A backup is one moment: every table is read in one snapshot, so a backup taken while tills sell restores
       to data the auditor passes — no sale without its lines, no payment without its sale.
       Tricky: a sale committing between two tables' reads; the safety copies (they use the same dump).
       Found in: ff82edf8 (each table was read on its own connection, no snapshot).
       Checked by: machine (backups taken during random shop days, each restored and audited), example.

BAK-2  The round trip: a full backup restored into an empty migrated database gives every table back row for row
       (bar BAK-8), and the shop carries on — ids and receipt numbers (S-, CR-, DP-) continue past the restored
       ones, never colliding.
       Checked by: example, machine.

BAK-3  A backup file is whole or not there: written as .tmp, synced, renamed. A power cut part way leaves no
       *.json.gz that won't restore; empty files from older builds are never listed, counted or copied.
       Checked by: faults (the real binary killed during -backup and during an automatic backup).

BAK-4  Rotation keeps the newest BACKUP_KEEP automatic backups and deletes only *.json.gz in that folder. Safety
       copies (and backups fetched from a copy place) are never counted: each is kept 30 days, then deleted.
       User's answer: Q1 (2026-11-22).
       Checked by: example.

BAK-5  A restore is all or nothing: a file that isn't a backup (not gzip, not JSON, another format version, a
       reference to a row it lacks) is refused before anything changes; a failure part way, or a power cut,
       leaves the data as it was.
       Checked by: example, faults (killed during -restore).

BAK-6  A restore loses only what it means to replace: a safety copy of the data as it was is written first (no
       copy, no restore), and nothing a till does lands between that copy and the restore — it waits, then
       goes into the restored shop or is refused, never into neither.
       Found in: 996866fa (a sale finishing between the safety copy and the restore was in neither).
       Checked by: example (a sale at once with a restore).

BAK-7  Versions: an older build's backup restores into a newer one (a missing column takes its default, or its
       legacy value — products.reorder_auto); a newer build's columns and tables are dropped and reported, never
       silently. A full backup clears tables it doesn't carry; a partial one (plugin groups) touches only its own.
       Checked by: example (restore_compat).

BAK-8  Secrets never travel: a plugin's Secret columns are blank in every backup and come in blank; backup_copies
       (where this PC copies to) and goose bookkeeping are never written or replaced.
       Checked by: example.

BAK-9  After a restore every session ends whose user id now names someone else (another name or role); the
       person who restored is signed out.
       Checked by: example.

BAK-10 A setup-only export holds catalogue, configuration and people only, restores into an empty shop (its
       tables reference only each other) and carries no balances (customers' and suppliers' are 0).
       Checked by: example.

BAK-11 Only an admin backs up or restores; a backup is named by its bare file name in the backups folder,
       anything else (a path, "..", another suffix) is refused.
       Checked by: refusals.

BAK-12 Backups never stop the till: one at start, then every interval; a failure is logged and the next one
       tries again.
       Checked by: example.

CPY-1  The copy folder gets every one of the newest BACKUP_KEEP local backups it lacks, oldest first, each through
       a .tmp (a pulled USB leaves nothing that looks restorable), keeping the backup's own time; it keeps the
       newest BACKUP_KEEP there. A folder that isn't there (USB out) is skipped, never made — only Test makes it,
       under a parent that exists.
       Checked by: example.

CPY-2  One copy at a time: the server's pass and a Data page Copy now never write the same file at once.
       Checked by: example (at once).

CPY-3  Karots cloud gets the newest backup about once a day (20 h; a last copy dated in the future — a wrong
       clock — copies at once), sealed with the shop's key, named by its time only (no shop name); the same
       backup is never sent twice; the shop never deletes there.
       Checked by: example.

CPY-4  A sealed backup opens only with its shop's key; a changed byte or another key is refused with words.
       Checked by: example.

CPY-5  Fetch brings back only a backup the place lists (the page never names a path), keeps its time, and then
       restores through BAK-5 / BAK-6.
       Checked by: refusals, example.

CPY-6  Copy trouble never reaches the till: it is stored on backup_copies and shown on the Data page; the folder
       preset is seeded once (an owner who cleared it keeps it cleared).
       Checked by: example.

UPD-1  Only what Karots signed is installed: the manifest's ed25519 signature over the version and the unpacked
       binary's SHA-256, by the suite release key; a bad signature is a warning on the Data page, never
       installed. The channel never shows in a page, a log line or an error.
       Checked by: example.

UPD-2  Only newer: versions compare as fixed-width dates, a dev build is older than any; a version that had to
       be rolled back is never offered again (a newer one is).
       Checked by: example.

UPD-3  An install goes: download to .new and check → safety copy (before-update-<v>) → swap → mark pending →
       restart (exit 75). A failure before the swap leaves the running program and the data as they were and
       says so; a safety copy that fails stops the update.
       Checked by: example.

UPD-4  A power cut at any moment of an install leaves a program that starts — the old one or the new one, never
       an empty or half-written file, and a new one that can't start is still rolled back.
       Found in: c1ecf742 (a cut between the swap's two renames left no program; a cut after the swap, before
       the boot marker, left a new program BootGuard never counted).
       Checked by: example (never a moment without a program; Recover; a marker for another version).

UPD-5  BootGuard: a new version that fails to stay up twice (up = 2 minutes) is replaced by the previous one, and
       the Data page says so; a version that stayed up is marked healthy, and the old program's timer never
       clears the new one's count.
       Checked by: example.

UPD-6  After going back, the previous version starts and sells on the database the new one migrated (all of it
       or part way): a release's migrations only add (new columns have defaults or allow empty; nothing the
       previous version reads or writes is dropped, renamed or made stricter in the same release).
       And reads skip columns a build doesn't know (db.Connect: sqlx Unsafe).
       User's answer: Q2 (2026-11-22). Found in: 5929334f (the released 2026.11.17.2028 refused every sale on
       today's schema: "missing destination name session_id" — it stays a known exception).
       Checked by: example (guard over new migrations), machine (the sales machine on a schema with extra
       columns), battle (each published release built and selling on this schema, make battle).

UPD-7  One install at a time; a download that receives nothing for 2 minutes gives up; offline is a calm state,
       never an error that stops anything.
       Checked by: example.

UPD-8  A restart for an update loses or doubles no sale: a Pay whose answer was lost is sent again with its
       one-time reference (SALE-50).
       Checked by: faults.

SET-1  Setup only applies install.conf: it never makes a secret or changes a shop setting; a main PC on another
       system than the build is refused; a cashier PC follows its own.
       Checked by: example.

SET-2  Running Setup again finishes what a failed or cut run left: each step can run twice.
       Checked by: example.

SET-3  An Update keeps this PC's own choices and the shop's data, and backs the shop up to the keep dir before
       the program is replaced; if that backup fails, nothing is changed.
       User's answer: Q3 (2026-11-22).
       Checked by: example.

SET-4  A mode change (file ⇄ server) happens only when confirmed: safety copy → new mode → restore, and the data
       after is the data before (BAK-2). A restore that failed is tried again on the next run without a new
       backup; the before-move backup is kept.
       Checked by: example, round trip.

SET-5  Uninstall backs the shop up first (Karots stopped) and copies out the automatic backups inside the program
       folder; if that fails nothing is removed. Only Karots's own folder is ever deleted (a full path, not a
       drive, system, home or mount folder, holding the program, not holding the backups folder). PostgreSQL goes
       only if ticked.
       Checked by: example.

SET-6  Bring back a shop is offered only on a first install, never over a shop already on this PC; a restore
       into a database that already holds a shop keeps a safety copy first; -backup and -restore refuse while
       the till is running.
       Checked by: example.

SET-7  An install or Update needs "I agree"; the record names the SHA-256s of the LICENSE and Shop Rules that
       Setup carried.
       Checked by: example.

SET-8  Setup's log (support gets a copy) never shows the database passwords.
       Checked by: example.

SET-9  File mode: one running copy per shop (the till's port is claimed before the database starts); a data
       folder of another PostgreSQL version is refused, never wiped; the database never uses the plain C locale.
       Checked by: example.

BLD-1  A build leaves the tree core-only — plugin lists and the PostgreSQL bundle put back — whether it worked
       or failed.
       Checked by: example.

BLD-2  A Release refuses uncommitted changes (but shops.json and the plugin lists); each shop build carries its
       own install id and support seed, never the master secret.
       Checked by: example.

BLD-3  Publish puts the binary up first, then the manifest (no-store), then prunes old binaries, so a shop
       reading the manifest at any moment finds its file; the changelog keeps the last 10.
       Checked by: example.

BLD-4  Shops take only manifests signed by the release key (UPD-1); the key is made once and reused.
       Checked by: example.

BLD-5  A Release whose upload fails keeps its build; Publish sends it later without building again.
       Checked by: example.

BLD-6  A Release builds only code `make test-all` passed: the run records its commit only when Go, Vitest,
       Playwright and check-core all passed, the tree had no uncommitted changes when it started and the commit
       didn't move during it (`~/.cache/karots-logs/passed-commits`). Later commits still count as tested when
       they change only shops.json, the plugin lists or `docs/` (never `docs/legal/`: Setup embeds its PDF).
       Anything else is refused with "run make test-all first". A Test build is never held back.
       Checked by: example.

BLD-7  Every shop build carries the published catalogue's address, `UPDATE_BASE_URL/CATALOGUE_DIR` (CTL-2):
       a shop with the Catalogue plugin syncs from it; a build without the plugin ignores it. No CATALOGUE_DIR
       in the Builder's .env → no address, and the plugin's page says so (the build still goes).
       Checked by: example.
```
