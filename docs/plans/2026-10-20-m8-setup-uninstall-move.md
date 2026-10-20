# Setup: Uninstall, Move, Mode change — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Setup can uninstall any install (backup first, backups kept), bring a shop back from a backup at first
install, and carry a shop's data across a file ⇄ server mode change on Update.

**Architecture:** The shop's own program gains `-backup <dir>` and `-restore <file>` one-shot flags; Setup never
touches shop data itself, it runs those (old binary dumps, new binary restores). Setup gets a backup folder
(`C:\Karots backups` / `~/Karots backups`), a "safety-backup" + "restore" step pair in the install steps, and a
separate list of uninstall steps (Linux + Windows bodies via `pick`), all through the existing `Job` / `Step` /
`Runner` seams so they test with the `rec` runner and `root` prefix.

**Tech Stack:** Go (cmd/server, cmd/setup), internal/backup, PowerShell (Windows steps), systemd/psql (Linux), the
Setup page (`cmd/setup/page.html`, vanilla JS).

**Spec:** `docs/specs/2026-10-20-m8-setup-uninstall-move-design.md`

## Global Constraints

- Backups are never deleted by Setup. If the final / before-move backup fails, nothing is removed or changed.
- Backup folder default: Windows `C:\Karots backups`, Linux `<shop user's home>/Karots backups` (owned by the shop user).
- Backup tags: `final` (uninstall), `before-move` (mode change). File names come from `backup.WriteSnapshot`.
- File mode's PostgreSQL port `filedb.DefaultPort` (5434); server mode the profile's `DB.Port` (normally 5432).
- On Linux the shop's program runs as the shop user (`runuser -u <user> --`): file mode refuses root.
- "Also remove PostgreSQL" is **off** by default; a remote database (`DB.Local == false`) is never touched.
- Uninstall never removes Chrome / Chromium / curl / CUPS / VC++ files.
- File → server leaves `data/` renamed `data-before-server-<YYYY-MM-DD>`; server → file leaves the old database alone.
- Files ≤ 500 lines; comments in the surrounding style; no Claude attribution in commits.

## Review Focus

1. **Final backup fails** (program won't start, disk full, port busy) → uninstall stops at step 1, folder intact. Test in Task 5.
2. **A dangerous `InstallDir`** (`/`, `/home/x`, `C:\`, a folder without the binary) → folder removal refuses. Test in Task 5.
3. **Half-installed / already-removed PC** (no unit, no task, no firewall rule, no database) → uninstall still completes. Test in Task 5.
4. **A bad restore file** (not gzip, wrong version) → install stops before "Start with the PC", the till is never started on an empty database. Test in Task 4.
5. **Server → file Update without a mode change asked for** (headless, no `-change-mode`) → refused with words, nothing changed. Test in Task 6.

---

### Task 1: Program flags `-backup` and `-restore`

**Files:**
- Create: `cmd/server/backupcli.go`
- Modify: `cmd/server/main.go` (flag block, one-shot handling after `app.Migrate` / before it for `-backup`)
- Test: `cmd/server/backupcli_test.go` (DB-gated on `DATABASE_URL`, like other DB tests)

**Interfaces:**
- Produces (CLI, used by Task 3):
  - `karots-suite -backup <dir> [-tag <tag>]` → prints exactly one line `backup: <absolute path>` on stdout, exit 0; exit 1 with the error on stderr.
  - `karots-suite -restore <file>` → migrates, restores, prints `restored: <rows> rows` (+ `skipped tables: …` / `dropped columns: …` lines when non-empty), exit 0; exit 1 on error.
- Produces (Go): `func backupTo(ctx context.Context, db *sqlx.DB, dir, tag string) (string, error)`, `func restoreFrom(ctx context.Context, db *sqlx.DB, file string) (backup.RestoreReport, error)`.

- [ ] **Step 1: Write the failing test** `TestBackupRestoreRoundTrip`: migrate the test DB, insert one product, `backupTo(ctx, db, t.TempDir(), "final")` → path ends `-final.json.gz` and exists; delete the product; `restoreFrom(ctx, db, path)` → `rep.Rows > 0` and the product is back. `TestRestoreRefusesBadFile`: a file with `"hello"` → error containing `not a valid backup file`, data untouched.
- [ ] **Step 2: Run** `go test ./cmd/server -run 'BackupRestore|RestoreRefuses' -v` — FAIL (undefined).
- [ ] **Step 3: Implement** `backupTo` (= `backup.WriteSnapshot(ctx, db, dir, time.Now(), tag)`) and `restoreFrom` (open file, `backup.ReadArchive` first so a bad file is refused before anything, then `backup.RestoreArchive`). Wire the flags in `main.go`: both count as `oneShot` (no BootGuard); `-backup` runs **before** `app.Migrate` (the installed binary dumps its own schema) and closes `ln`; `-restore` runs after `app.Migrate`. Server mode (no file DB): before connecting, try `net.Listen` on `SERVER_PORT` — busy → exit 1 `the till is running (port N) — stop it first`. File mode already claims the port in `startFileMode`. Return (not `os.Exit`) so file mode stops its PostgreSQL.
- [ ] **Step 4: Run** the tests — PASS; `go vet ./cmd/server`.
- [ ] **Step 5: Commit** `feat(server): -backup <dir> and -restore <file> one-shot flags (for Setup)`

### Task 2: Installed mode + keep no `DATABASE_URL` (the bug)

**Files:**
- Modify: `cmd/setup/installed.go`, `cmd/setup/main.go` (`newJob`), `cmd/setup/steps.go` (`Job` fields live where `Job` is declared — find it with `grep -n "type Job struct" cmd/setup`)
- Test: `cmd/setup/installed_test.go`

**Interfaces:**
- Produces: `func installedMode(installDir string) string` → `"server"` (installed `.env` has `DATABASE_URL`), `"file"` (no `DATABASE_URL` and a `data` dir beside the binary), `""` (unknown / not installed). `Job.FromMode string` — set by `newJob` to the installed mode **only when** updating and it differs from `Conf.Mode`; `""` otherwise. `Job.ChangeMode bool` (confirmed by the person / `-change-mode`).

- [ ] **Step 1: Write failing tests** `TestInstalledMode` (three cases on temp dirs) and `TestApplyInstalledDropsDatabaseURL`: installed env `{DATABASE_URL: "postgres://…", MY_LINE: "x"}`, conf env without `DATABASE_URL` → result has `MY_LINE`, no `DATABASE_URL`, and `kept` doesn't list it.
- [ ] **Step 2: Run** `go test ./cmd/setup -run 'InstalledMode|DropsDatabaseURL' -v` — FAIL.
- [ ] **Step 3: Implement**: a `builderOwned = []string{"DATABASE_URL"}` list skipped by `applyInstalled`'s keep loop; `installedMode`; `newJob` sets `FromMode`.
- [ ] **Step 4: Run** `go test ./cmd/setup` — PASS (existing tests too).
- [ ] **Step 5: Commit** `fix(setup): an Update never keeps the installed DATABASE_URL (server → file stayed on the server); know the installed mode`

### Task 3: Backup folder + running the program's backup / restore

**Files:**
- Create: `cmd/setup/restore.go`
- Test: `cmd/setup/restore_test.go`

**Interfaces:**
- Consumes: Task 1's CLI output lines.
- Produces:
  - `func (j *Job) keepDir() string` — `Job.KeepDir` if set, else `C:\Karots backups` (Windows) / `<homeFn(j.User)>/Karots backups`.
  - `func programBackup(j *Job, tag string) (string, error)` — makes `keepDir` (Linux: `chown` to the shop user), runs the **installed** `j.binPath()` `-backup <keepDir> -tag <tag>` (Linux via `runuser -u <user> --`), returns the path from the `backup: ` line; no such line → error with the program's output.
  - `func programRestore(j *Job, file string) (string, error)` — runs `j.binPath() -restore <file>` the same way, returns the `restored: …` summary line.
  - `type foundBackup struct { Path, Name string; Modified time.Time; BesideSetup bool }` and `func findBackups(j *Job) []foundBackup` — `*.json.gz` (not `.tmp`) in `keepDir()` and `j.Dir`, newest first.
  - `Job.KeepDir string`, `Job.Restore string` (file to bring back).

- [ ] **Step 1: Write failing tests** with `linuxJob`/`rec`: `TestProgramBackupLinux` — `rc.answers["runuser -u shop -- /opt/karots-suite/karots-suite -backup /home/shop/Karots backups -tag final"] = "…log…\nbackup: /home/shop/Karots backups/al-raj-x-final.json.gz\n"` → returns that path, and `chown` of the folder ran; missing line → error containing the output. `TestProgramBackupWindows` — no `runuser`, path `C:\Karots backups`. `TestFindBackups` — two files in keepDir + one in `j.Dir` (newest by mtime) → order newest first, `BesideSetup` true only for the `j.Dir` one; `.tmp` ignored.
- [ ] **Step 2: Run** `go test ./cmd/setup -run 'ProgramBackup|FindBackups' -v` — FAIL.
- [ ] **Step 3: Implement** in `restore.go` (all `run` calls so the log and `busy` heartbeat work).
- [ ] **Step 4: Run** — PASS.
- [ ] **Step 5: Commit** `feat(setup): the Karots backups folder; run the shop program's -backup / -restore`

### Task 4: Install steps — "Back up first" and "Bring back the shop" (restore at install + mode change)

**Files:**
- Modify: `cmd/setup/steps.go` (`Steps`), `cmd/setup/restore.go`, `cmd/setup/linux_system.go` / `cmd/setup/windows_system.go` (factor the stop out of `linuxProgram` / `windowsProgram` into `linuxStop(j) error` / `windowsStop(j) error`, reused here and in Task 5)
- Test: `cmd/setup/restore_test.go`, `cmd/setup/steps_test.go`

**Interfaces:**
- Consumes: Task 2 `FromMode`, `ChangeMode`; Task 3 `programBackup`, `programRestore`.
- Produces: steps `safety-backup` ("Back up the shop first") and `restore` ("Bring back the shop") in `Steps`.
  - Order: `checks, safety-backup, database, program, settings, backups, restore, startup, network, printing, open, health`.
  - `safety-backup` applies when `j.FromMode != ""` (main PC): stop the till (`linuxStop` / `windowsStop`), `programBackup(j, "before-move")`, sets `j.Restore` to it. Errors stop everything (nothing changed yet).
  - `restore` applies when `j.server() && j.Restore != ""`: `programRestore`, note the summary on the page; then if `FromMode == "file"` rename `<InstallDir>/data` → `data-before-server-<YYYY-MM-DD>` (exists → add `-2`, `-3`…).
  - Detail lines: `"Back up the shop to <keepDir> before changing how it is kept"`, `"Bring back the shop from <file name>"`.

- [ ] **Step 1: Write failing tests**: `TestStepsOrderWithModeChange` (ids in the order above for a file → server Update); `TestModeChangeFileToServer` — run all steps with `rec`: the backup command runs **before** `apt-get` / the binary copy, the restore command runs **after** the `.env` write and **before** `systemctl enable`; `data` renamed. `TestRestoreBadFileStopsBeforeStartup` (Review Focus 4): `rc.fail` on the `-restore` command → `RunAll` errors at `restore`, no `systemctl enable karots-suite` in `rc.all()`. `TestFirstInstallRestore`: `Restore` set, not updating → `restore` step present, no `safety-backup`.
- [ ] **Step 2: Run** `go test ./cmd/setup -run 'ModeChange|StepsOrder|RestoreBadFile|FirstInstallRestore' -v` — FAIL.
- [ ] **Step 3: Implement** the two steps, `linuxStop` / `windowsStop`.
- [ ] **Step 4: Run** `go test ./cmd/setup` — PASS.
- [ ] **Step 5: Commit** `feat(setup): back up before a mode change and bring the shop back after it (and at a first install)`

### Task 5: Uninstall steps (Linux + Windows)

**Files:**
- Create: `cmd/setup/uninstall.go` (steps, guards, shared), `cmd/setup/linux_uninstall.go`, `cmd/setup/windows_uninstall.go`
- Modify: `cmd/setup/steps.go` (`Steps` returns `uninstallSteps(j)` when `j.Uninstall`)
- Test: `cmd/setup/uninstall_test.go`

**Interfaces:**
- Consumes: `programBackup`, `keepDir`, `linuxStop` / `windowsStop`, `noKiosk`, `stopLauncherPS`, `psql` / `winPSQL`, `packageManager`.
- Produces: `Job.Uninstall bool`, `Job.RemovePostgres bool`, `Job.NoBackup bool`; `func uninstallSteps(j *Job) []Step` with ids, in order:
  1. `final-backup` (main PC, unless `NoBackup`): stop, `programBackup(j, "final")`, copy `<InstallDir>/data/backups/*.json.gz` (and a `BACKUP_DIR` inside `InstallDir`) into `<keepDir>/automatic/`. Error → stop.
  2. `remove-startup`: Linux `systemctl disable --now karots-suite` (error ignored when the unit is missing), remove the unit, `daemon-reload`. Windows: `Unregister-ScheduledTask` `windowsTask`, `stopLauncherPS`, stop the binary.
  3. `remove-open`: Linux: `pkill -f kiosk.sh`, `pkill -f [-]-kiosk`, remove `~/.config/autostart/karots-kiosk.desktop`, `~/.karots-kiosk`, the three `karots-kiosk.json` policy files, `~/Desktop/karots-suite.desktop`, `~/.local/share/applications/karots-suite.desktop`. Windows: `noKiosk` + the shortcuts `windowsOpen` makes (CommonDesktop / CommonPrograms `Karots Suite.lnk`).
  4. `remove-network` (main PC): Linux `ufw delete allow <port>/tcp` / `firewall-cmd --remove-port=<port>/tcp --permanent` + reload when that firewall is on; Windows `Remove-NetFirewallRule -DisplayName 'Karots Suite'`. Missing rule = fine.
  5. `remove-database` (applies `j.localDB()`): `DROP DATABASE IF EXISTS <name> WITH (FORCE)`, `DROP ROLE IF EXISTS <user>`; with `RemovePostgres`: Linux remove the packages `linuxDatabase` installs for that package manager (apt `postgresql-17 postgresql-common`, dnf/zypper `postgresql-server postgresql`, pacman `postgresql`); Windows run `C:\Program Files\PostgreSQL\17\uninstall-postgresql.exe --mode unattended`.
  6. `remove-folder` (main PC): `safeToRemove(j)` then remove `InstallDir`.
  7. `done-note`: note `"Karots is removed. The shop's backup is in <path> — keep it: it brings the shop back on any PC."`
  - Cashier PC: `remove-open` only.
  - `func safeToRemove(j *Job) error` — refuses unless `InstallDir` is absolute, has ≥ 2 path parts, isn't the shop user's home, a drive root, `/opt`, `/usr`, `/home`, `C:\Program Files`, and contains `j.Conf.Binary`.

- [ ] **Step 1: Write failing tests**: `TestUninstallLinuxFileMode` (all commands in order, folder gone under `root`, keep dir has the copied automatic backups); `TestUninstallBackupFailsKeepsEverything` (Review Focus 1: `-backup` fails → error at `final-backup`, `InstallDir` still exists, no `systemctl disable` ran); `TestSafeToRemove` table (Review Focus 2: `/`, `/opt`, `/home/shop`, `C:\`, a dir without the binary → error; `/opt/karots-suite` with binary → nil); `TestUninstallHalfInstalled` (Review Focus 3: `systemctl disable` and `ufw delete` fail → still `@@DONE`-equivalent: `runSteps` returns nil); `TestUninstallServerRemovePostgres` (apt purge listed only with `RemovePostgres`; remote DB → no `DROP`); `TestUninstallWindows` (scripts contain `Unregister-ScheduledTask`, `Remove-NetFirewallRule`, the uninstaller only with `RemovePostgres`); `TestUninstallCashier` (only `remove-open`).
- [ ] **Step 2: Run** `go test ./cmd/setup -run 'Uninstall|SafeToRemove' -v` — FAIL.
- [ ] **Step 3: Implement** the three files.
- [ ] **Step 4: Run** `go test ./cmd/setup` — PASS; every new file ≤ 500 lines (`wc -l cmd/setup/*.go`).
- [ ] **Step 5: Commit** `feat(setup): Uninstall — final backup first (kept), then service, kiosk, firewall, database, folder`

### Task 6: Page, flags and headless

**Files:**
- Modify: `cmd/setup/main.go` (`Choice`, flags, `applyChoice`, `headless`), `cmd/setup/page.go` (`/api/plan`, `/api/run`), `cmd/setup/page.html`
- Test: `cmd/setup/page_test.go`, `cmd/setup/steps_test.go`

**Interfaces:**
- Consumes: Tasks 2–5 `Job` fields, `findBackups`, `keepDir`.
- Produces:
  - `Choice` gains `Uninstall, RemovePostgres, ChangeMode bool`, `Restore, KeepDir string` (JSON `uninstall, removePostgres, changeMode, restore, keepDir`).
  - `/api/plan` adds `fromMode`, `keepDir`, `backups` (`[{path,name,modified,besideSetup}]`), `installed` (bool = `Updating`), `localDb` (bool).
  - Flags: `-uninstall`, `-remove-postgres`, `-no-backup` (technicians; never on the page), `-restore <file>`, `-change-mode`, `-keep-dir <dir>`.
  - `applyChoice` refuses: a mode change (`FromMode != ""`) without `ChangeMode` → `"this Update changes how the shop is kept (<from> → <to>) — tick 'Change it' on the page, or add -change-mode"`; `Restore` on an Update without a mode change → `"bring a backup back on the Data page — this PC already has the shop"`; a relative `KeepDir` (same message style as the backup folder).
  - Page: when installed, an **Uninstall** button beside Update → a panel listing the uninstall steps (from `/api/plan?uninstall=1`), the keep folder (folder picker), "Also remove PostgreSQL" (only when `localDb`, unticked), a confirm button "Uninstall Karots from this PC"; the Done screen shows the backup path. First install of a main PC: "Bring back a shop" section — the newest found backup offered (ticked only when `besideSetup`), "Choose another…" (folder picker filtered to `.json.gz`, or a path box). Update with `fromMode`: a banner at the top "This changes how the shop is kept: One PC → Server" (or "Server → One PC") with what will happen and a "Change it" checkbox that must be ticked before Update runs.

- [ ] **Step 1: Write failing tests**: `TestApplyChoiceModeChangeNeedsConfirm` (Review Focus 5), `TestApplyChoiceRestoreOnUpdateRefused`, `TestPlanListsBackupsAndFromMode` (`/api/plan` JSON has the new fields), `TestPlanUninstallSteps` (`?uninstall=1` → the Task 5 ids).
- [ ] **Step 2: Run** `go test ./cmd/setup -run 'ModeChangeNeedsConfirm|RestoreOnUpdate|PlanLists|PlanUninstall' -v` — FAIL.
- [ ] **Step 3: Implement** flags, `Choice` plumbing, API fields, page sections (follow the existing page's markup and style; no new libraries).
- [ ] **Step 4: Run** `go test ./cmd/setup` — PASS. Look at the page with `karots-setup -dry` beside an install.conf for: first install with a backup beside it, an Update with a mode change, Uninstall (screenshot each, check the 360px-wide window too).
- [ ] **Step 5: Commit** `feat(setup): Uninstall, Bring back a shop and the mode-change banner on the page; -uninstall / -restore / -change-mode`

### Task 7: Docs and handoff

**Files:**
- Modify: `AGENTS.md` (Layout: `cmd/setup` line mentions uninstall / restore / mode change; Dev workflow: `-uninstall`, `-restore`, `-change-mode`; `cmd/server` flags list gains `-backup -restore`), `docs/status.md` (handoff: built, acceptance list from the spec for the user's VMs, then Fedora / Arch, then M8c2)

- [ ] **Step 1: Edit** both files.
- [ ] **Step 2: Run** `make test` (Go) and `go vet ./...` — PASS.
- [ ] **Step 3: Commit** `docs: Setup uninstall / move / mode change — AGENTS + status handoff`
