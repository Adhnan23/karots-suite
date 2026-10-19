# M8b Shop Builder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Shop Builder keeps a profile per shop in `shops.json` and turns one into three files — `karots-setup`,
`install.conf`, the binary — and `karots-setup` installs it from a page, no command line.

**Architecture:** `internal/shopconf` is the one contract between Builder and installer (`install.conf`, JSON).
`cmd/bootstrap` gains profiles (`profiles.go`), shop builds (`shopbuild.go`), karots-pos import and shop endpoints +
page. New `cmd/setup` = a list of modular steps (each says whether it applies to a role / mode and returns the OS
commands it runs) + a loopback page; an unelevated launcher opens the page and starts the elevated worker.

**Tech Stack:** Go stdlib (+ `golang.org/x/sys/windows`, already in `go.mod`), embedded plain HTML/JS pages as the
Builder has today. Linux: systemd, apt / dnf / pacman / zypper, `pkexec`. Windows: PowerShell 5, winget, Scheduled
Tasks, UAC.

**Spec:** `docs/specs/2026-10-19-m8b-shop-builder-design.md`

## Global Constraints

- A shop build is exactly three files in `dist/<key>/`: `karots-setup(.exe)`, `install.conf` (0600), `karots-suite(.exe)`; **Add installer** off → only the binary.
- `shops.json` at the repo root, committed; saved atomically (temp + rename), indented, shops sorted by key.
- Defaults: till port **3000**, DB port **5432**, file-DB port **5434**, DB `suite_db` / `suite_user`, install dir `/opt/karots-suite` | `C:\karots-suite`, open `kiosk`.
- Secrets: JWT 48 hex, DB password 32 hex, DB superuser password 32 hex, channel 32 hex (`crypto/rand`).
- `env` refuses `POS_SUPPORT_SECRET`, `POS_SYSTEM_PIN` and the owned keys `APP_ENV`, `SERVER_PORT`, `DATABASE_URL`, `JWT_SECRET`, `BACKUP_DIR`, `FILE_DB_PORT`.
- A shop build refuses without `POS_SUPPORT_SECRET`; a quick build only warns (today's behaviour).
- The installer never generates a secret or edits a setting; it refuses to run without `install.conf` beside it.
- Never stop the shop: a failed step stops the install with words + **Try again**, never touches existing data. No Uninstall.
- Core stays plugin-inert: `cmd/setup` and `internal/shopconf` never name a plugin (`make check-core`). Files ≤ 500 lines.
- No Claude / AI attribution in commits.

## Review Focus

1. **Update over a running install** — the binary is in use: the program step must stop the service / task (and wait for the process to exit) before copying, then start it; file-mode `data/` untouched. Test in Task 7 / 8.
2. **Till port already taken by another program** (karots-pos on 3000) — Checks fail with "port 3000 is used by another program — stop it or change the shop's port in the Shop Builder", but **our own** running install on that port passes. Test in Task 6.
3. **PostgreSQL already on a Windows PC** with an unknown `postgres` password — the DB step fails asking for it; the page shows one password field and Try again sends it. Test in Task 8 / 9.
4. **Password with URL-special characters** in a hand-edited profile — `DATABASE_URL` escapes it (`url.UserPassword`). Test in Task 2.
5. **Old `install.conf` from an earlier Builder** (unknown or missing field) — Read refuses an unknown `version` with "this installer is older than the shop build — rebuild in the Shop Builder". Test in Task 1.

---

### Task 1: `internal/shopconf` — the install.conf contract

**Files:**
- Create: `internal/shopconf/shopconf.go`, `internal/shopconf/shopconf_test.go`

**Interfaces:**
- Produces:
  ```go
  const Version = 1
  const FileName = "install.conf"
  type DB struct { Host string `json:"host"`; Port int `json:"port"`; Name, User, Password string; SuperPassword string `json:"superPassword"`; Local bool `json:"local"` } // camelCase json tags throughout
  type Conf struct {
      Version int; Shop, Key, InstallID string; OS string // "linux" | "windows"
      Mode string   // "file" | "server"
      Binary string // file name beside the installer, e.g. "karots-suite.exe"
      InstallDir string; Port int; Open string // "kiosk" | "background"
      LanAddress string; DB *DB             // nil in file mode
      Env map[string]string                  // the final .env
  }
  func Read(path string) (Conf, error)            // validates Version, Mode, Open, Port, Binary (no path separators), DB present iff server
  func Write(path string, c Conf) error           // 0600, indented JSON
  func (c Conf) EnvFile() []byte                  // KEY=value\n, keys sorted, no quotes/comments
  ```

- [ ] **Step 1: Write the failing tests** — `TestRoundTrip` (Write then Read equal, file mode 0600); `TestReadRefuses` table: version 2 → error containing "rebuild in the Shop Builder"; mode "x"; server with nil DB; binary `"../x"`; port 0; open "x". `TestEnvFile`: `{"B":"2","A":"x y"}` → `"A=x y\nB=2\n"`; a value containing `\n` is refused by Read.
- [ ] **Step 2: Run** `go test ./internal/shopconf/` → FAIL (package missing).
- [ ] **Step 3: Implement** in `shopconf.go`.
- [ ] **Step 4: Run** `go test ./internal/shopconf/` → PASS.
- [ ] **Step 5: Commit** `feat(shopconf): install.conf — the Shop Builder ⇄ installer contract`

### Task 2: Shop profiles (`shops.json`)

**Files:**
- Create: `cmd/bootstrap/profiles.go`, `cmd/bootstrap/profiles_test.go`

**Interfaces:**
- Consumes: `shopconf.Conf`, `shopconf.DB`, `support.NewInstallID()`, `withRequired`, `manifest`.
- Produces:
  ```go
  type ShopDB struct { Host string; Port int; Name, User, Password, SuperPassword string }
  type ShopInstall struct { Dir, Open, LanAddress string }
  type Built struct { At time.Time; Commit string }
  type Shop struct {
      Key, Name, OS, Arch, Mode string; Plugins []string; InstallID, Channel string
      Port int; DB *ShopDB; FileDBPort int; JWTSecret string
      Install ShopInstall; Env map[string]string; LastBuilt *Built
  }
  type Shops struct { Shops []Shop `json:"shops"` }
  func LoadShops(path string) (Shops, error)        // missing file → empty, no error
  func SaveShops(path string, s Shops) error         // sorted by key, temp file + rename in the same dir
  func NewShop(key, name, osName, arch, mode string) (Shop, error) // mints install id, channel, secrets; defaults per Global Constraints
  func (s *Shop) SetMode(mode string) error          // server→file drops DB+JWT; file→server mints them
  func (s *Shop) Rotate(what string) error           // "jwt" | "db" | "dbsuper"
  func (s Shop) Check(all []manifest) (warnings []string, err error) // key ^[a-z0-9][a-z0-9-]{0,39}$, OS linux|windows (+ filedb.Supported for file), known plugins, refused env keys, ports 1–65535
  func (s Shop) Conf(binary string) shopconf.Conf    // the final env + install fields; DB.Local = host is localhost / 127.0.0.1 / ::1
  func (s Shop) Masked() Shop                        // secrets → "••••" (JWT, DB passwords, env values whose key contains SECRET|PASSWORD|KEY|TOKEN)
  ```
  `-shops <path>` flag (default `shops.json`, env `SHOPS_FILE`) is added in Task 3.

- [ ] **Step 1: Write the failing tests**
  - `TestNewShopServer`: 8-char install id, 32-hex channel, 48-hex JWT, DB `localhost:5432 suite_db suite_user`, 32-hex passwords, port 3000, dir `/opt/karots-suite` (linux) / `C:\karots-suite` (windows), open `kiosk`.
  - `TestNewShopFile`: DB nil, JWT empty, FileDBPort 5434.
  - `TestConfEnvServer`: `Env` = `APP_ENV=production`, `SERVER_PORT=3000`, `DATABASE_URL=postgres://suite_user:<pw>@localhost:5432/suite_db?sslmode=disable`, `JWT_SECRET`, `BACKUP_DIR=/opt/karots-suite/backups` (windows `C:\karots-suite\backups`), plus extras; a password `p@ss:/w` comes out escaped (`p%40ss%3A%2Fw`).
  - `TestConfEnvFile`: only `APP_ENV`, `SERVER_PORT`, `FILE_DB_PORT=5434` + extras; `DB` nil.
  - `TestCheckRefuses`: env `POS_SUPPORT_SECRET`, env `DATABASE_URL`, unknown plugin `gamma`, key `Al Raj`, OS `darwin`; `TestCheckWarns`: port == FileDBPort → one warning.
  - `TestSetMode`, `TestRotate` (value changes, length kept), `TestMasked` (no secret survives; original untouched).
  - `TestSaveAtomic`: save over an existing file where the temp write fails (dir made read-only after creating the file) → old content intact; normal save sorted by key.
- [ ] **Step 2: Run** `go test ./cmd/bootstrap/ -run 'Shop|Conf|Check|Save|Mode|Rotate|Masked'` → FAIL.
- [ ] **Step 3: Implement** `profiles.go`.
- [ ] **Step 4: Run** the same → PASS.
- [ ] **Step 5: Commit** `feat(bootstrap): shop profiles in shops.json (secrets, ports, install settings)`

### Task 3: Shop build → three files, CLI

**Files:**
- Create: `cmd/bootstrap/shopbuild.go`, `cmd/bootstrap/shopbuild_test.go`
- Modify: `cmd/bootstrap/main.go` (flags `-shops`, `-shop <key|all>`, `-list`, `-no-installer`; doc comment), `cmd/bootstrap/build.go` (`supportFlags` gets a `require bool`)

**Interfaces:**
- Consumes: `Shop`, `LoadShops/SaveShops`, `build(buildOpts, io.Writer)`, `shopconf.Write`.
- Produces:
  ```go
  type shopBuildOpts struct { Out string; Installer bool }
  // buildShop empties <out>/<key>, builds the binary (via b, Name "karots-suite", Mode, Plugins, InstallID, Require secret),
  // then with Installer: cross-builds ./cmd/setup into it and writes install.conf; records LastBuilt (now, `git rev-parse --short HEAD`).
  func buildShop(s *Shop, all []manifest, o shopBuildOpts, b func(buildOpts, io.Writer) (buildResult, error), setup func(osName, arch, out string, log io.Writer) error, log io.Writer) (buildResult, error)
  func buildSetup(osName, arch, out string, log io.Writer) error // go build -ldflags="-s -w" -o out/karots-setup[.exe] ./cmd/setup, CGO_ENABLED=0, GOAMD64=v1
  ```
  `buildOpts` gains `RequireSecret bool`; `build` fails with "POS_SUPPORT_SECRET is not set — a shop build needs it (each shop gets its own support PIN)" when set and the env var is empty.

- [ ] **Step 1: Write the failing tests** (fake `b` writes a dummy binary, fake `setup` writes a dummy file):
  - `TestBuildShopThreeFiles`: windows shop → `dist/al-raj/` has exactly `karots-setup.exe`, `install.conf` (0600, `Read` OK, `Binary == "karots-suite.exe"`, `InstallID` = profile's), `karots-suite.exe`; a stale file placed there before is gone; `LastBuilt` set.
  - `TestBuildShopNoInstaller`: only the binary.
  - `TestBuildShopPassesProfile`: `b` receives Mode, Plugins (with requires), InstallID, `RequireSecret: true`.
  - `TestBuildRequiresSecret`: `build` with `RequireSecret` and empty `POS_SUPPORT_SECRET` → that error, nothing built.
- [ ] **Step 2: Run** `go test ./cmd/bootstrap/ -run 'BuildShop|RequiresSecret'` → FAIL.
- [ ] **Step 3: Implement** `shopbuild.go`, the `RequireSecret` check, and the flags in `main.go`: `-shop all` builds each shop in key order and stops at the first failure; `-list` prints `key  name  os/arch  mode  port  plugins  last built`; after a shop build `SaveShops` (LastBuilt).
- [ ] **Step 4: Run** `go test ./cmd/bootstrap/` → PASS (old tests too).
- [ ] **Step 5: Commit** `feat(bootstrap): a shop build is three files — karots-setup, install.conf, the binary`

### Task 4: `-import-pos`

**Files:**
- Create: `cmd/bootstrap/importpos.go`, `cmd/bootstrap/importpos_test.go`
- Modify: `cmd/bootstrap/main.go` (flag `-import-pos <path>`)

**Interfaces:**
- Produces: `func importPOS(path string, all []manifest, into *Shops) (added []string, dropped map[string][]string, err error)`

- [ ] **Step 1: Write the failing test** `TestImportPOS` with a karots-pos-shaped file (`key, name, os, arch, plugins, install_id, channel`): keeps key / name / os / arch / install id, mode server, port 3000, channel **differs** from the karots-pos one, secrets minted; plugin `oldthing` → `dropped["shop"] == ["oldthing"]`; a key already in `into` is skipped (not overwritten) and not in `added`.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(bootstrap): -import-pos brings karots-pos shops over (same install ids, new channels)`

### Task 5: Builder page — shops first

**Files:**
- Create: `cmd/bootstrap/app_shops.go`, `cmd/bootstrap/app_shops_test.go`, `cmd/bootstrap/app.js` (move the page's script out once `app.html` would pass 500 lines; embed both)
- Modify: `cmd/bootstrap/app.go` (`appHandler` takes the shops path; mounts the shop routes), `cmd/bootstrap/app.html`

**Interfaces:**
- Consumes: Task 2–3 functions.
- Produces (all JSON; every POST requires `Content-Type: application/json`, as `/api/build`; one `sync.Mutex` guards load-modify-save):
  - `GET /api/shops` → `{"shops":[Masked…], "warnings":{key:[…]}}`
  - `POST /api/shops` `{key,name,os,arch,mode}` → created shop (masked); duplicate key 409
  - `POST /api/shops/{key}` full masked shop back; a `"••••"` secret means "unchanged"; mode change through `SetMode`
  - `POST /api/shops/{key}/rotate` `{"what":"jwt"|"db"|"dbsuper"}`
  - `POST /api/shops/{key}/reveal` → unmasked shop (Show)
  - `POST /api/shops/{key}/env` → `text/plain` `.env` body, `Content-Disposition: attachment; filename="<key>.env"`
  - `POST /api/shops/{key}/delete` `{"confirm":"<key>"}`
  - `POST /api/shops/{key}/build` `{"installer":true}` → the same streamed log + `@@RESULT` / `@@ERROR` lines as `/api/build`; shares its busy lock

- [ ] **Step 1: Write the failing tests** (temp shops file): create → list masked (no secret string present in the body); duplicate 409; update with masked secrets keeps the real ones; rotate changes the stored JWT; env download has `DATABASE_URL=`; delete needs the matching confirm; build streams `@@RESULT` with a fake builder; a `text/plain` POST is 415.
- [ ] **Step 2: Run** `go test ./cmd/bootstrap/ -run AppShops` → FAIL. **Step 3: Implement** handlers.
- [ ] **Step 4: Page:** opens on the shop list (name, OS, mode, port, plugin count, last built; Build / Edit / Download .env / Delete); **+ New shop**; Edit groups **Shop · Database · Network · Install · More settings** (spec §B), secrets masked with Show + Rotate, install id / channel read-only, mode switch confirms; Build has **Add installer** (ticked) and streams the log; **Build without a shop** = today's form. Same look as the current page.
- [ ] **Step 5: Run** `go test ./cmd/bootstrap/` → PASS; run `go run ./cmd/bootstrap -shops /tmp/…/shops.json`, create / edit / build a file-mode linux shop, screenshot list + edit at 1366 and 360, self-review.
- [ ] **Step 6: Commit** `feat(bootstrap): Builder opens on the shop list — edit, rotate, download .env, build with installer`

### Task 6: `cmd/setup` — steps, plan, headless run

**Files:**
- Create: `cmd/setup/main.go`, `cmd/setup/steps.go`, `cmd/setup/steps_test.go`, `cmd/setup/checks.go`

**Interfaces:**
- Consumes: `shopconf.Read`.
- Produces:
  ```go
  type Role string // "server" | "cashier"
  type Job struct {
      Conf shopconf.Conf; Role Role; Address string // cashier: main PC host[:port]
      User string   // the shop user (Linux login / Windows DOMAIN\user)
      Dir string    // the folder holding the three files
      Updating bool // InstallDir already holds Conf.Binary
      SuperPassword string // typed on the page when an existing PostgreSQL's password is unknown
      Run Runner; Log io.Writer
  }
  type Runner interface { Run(name string, args ...string) (string, error); Script(ps string) (string, error) } // Script = PowerShell (windows only)
  type Step struct { ID, Title string; Applies func(*Job) bool; Do func(*Job) error }
  func Steps(j *Job) []Step          // in spec §E order, filtered by Applies
  func RunAll(j *Job, mark func(id string, err error)) error // stops at first failure
  ```
  Step IDs: `checks`, `database`, `program`, `settings`, `backups`, `startup`, `network`, `printing`, `open`, `health`.
  `main.go` flags: `-yes` (headless), `-role server|cashier`, `-address`, `-addr`, `-token`, `-user` (the last three are passed by the launcher, Task 9). With `-yes` it runs `RunAll` printing `✓ Title` / `✗ Title: err` and exits 0/1.

- [ ] **Step 1: Write the failing tests** — `TestStepsPerRole` table, IDs expected:
  - server + local DB: all ten; server + remote DB: all ten (`database` only checks); file: no `database`, no `backups`; cashier: `checks`, `open`, `health`.
  - `TestMissingConf`: no `install.conf` in Dir → error "build this shop in the Shop Builder".
  - `TestChecksPortTaken`: port held by a plain `net.Listen` (not ours) → error naming the port and "Shop Builder"; port answering our `/health` (test server returning the suite's body) → passes.
  - `TestRunAllStops`: a failing step → later steps not run, `mark` got the error.
- [ ] **Step 2: Run** `go test ./cmd/setup/` → FAIL. **Step 3: Implement** (OS-specific `Do` bodies are stubs returning nil until Tasks 7–8; `checks` real: admin, systemd on Linux, binary beside, InstallDir not a karots-pos folder (`karots-pos` binary present), port free or ours).
- [ ] **Step 4: Run** → PASS. **Step 5: Commit** `feat(setup): karots-setup — modular install steps per role and mode, headless -yes`

### Task 7: Linux steps

**Files:**
- Create: `cmd/setup/linux_db.go`, `cmd/setup/linux_system.go`, `cmd/setup/linux_open.go`, `cmd/setup/linux_test.go`

Ported from karots-pos `install.sh` (read it while porting), with values from `Conf`, never generated:
- `database`: package manager detect (apt / dnf / pacman / zypper), install PostgreSQL, distro initdb (dnf `postgresql-setup --initdb`, pacman `initdb --locale=C.UTF-8 -E UTF8`), enable + start, `ALTER SYSTEM SET port` + restart when `SHOW port` ≠ `DB.Port`, role create / `ALTER ROLE … PASSWORD` to the profile's, database create, `pg_hba` loopback scram rule (marker comment `karots-suite install`). Remote DB: TCP dial `host:port` (5 s) or error "the database at host:port doesn't answer".
- `program`: when updating `systemctl stop karots-suite` first; copy binary 0755 into InstallDir; chown to User.
- `settings`: write `Conf.EnvFile()` as `.env` 0600 owned by User. `backups`: mkdir `<InstallDir>/backups`.
- `startup`: `/etc/systemd/system/karots-suite.service` (User, WorkingDirectory, EnvironmentFile, ExecStart, `Restart=always`; `After/Wants=postgresql.service` only when the DB is local), daemon-reload, enable, restart.
- `network`: ufw / firewalld open `Port/tcp` when active. `printing`: install + enable CUPS.
- `open`: kiosk = Chromium install (snap fallback), `kiosk.sh` relaunch loop + DevTools policy + autostart `.desktop` (as karots-pos), URL `http://localhost:Port` or `http://Address`; background = `~/Desktop/karots-suite.desktop` + `~/.local/share/applications/karots-suite.desktop` running `xdg-open http://localhost:Port`, owned by User.
- `health`: GET `/health` up to 30 × 1 s.

- [ ] **Step 1: Write the failing tests** with a recording `Runner` (and a temp root for written files via a package var `root = "/"`): server-local apt → commands include `apt-get install … postgresql`, `ALTER ROLE suite_user WITH LOGIN PASSWORD '<profile pw>'`; file mode → no postgres command at all, unit has no `postgresql`; updating → `systemctl stop karots-suite` comes before the copy; background → `.desktop` files, no `kiosk.sh`; kiosk cashier → URL `http://192.168.1.10:3000`; `.env` written 0600 equal to `EnvFile()`.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** **Step 4: Run** → PASS.
- [ ] **Step 5: Commit** `feat(setup): Linux steps — PostgreSQL, program, .env, backups, systemd, firewall, CUPS, kiosk / background`

### Task 8: Windows steps

**Files:**
- Create: `cmd/setup/windows_db.go`, `cmd/setup/windows_system.go`, `cmd/setup/windows_open.go`, `cmd/setup/windows_test.go`

Ported from karots-pos `install.ps1`; each step is one PowerShell script via `Runner.Script`:
- `database`: winget `PostgreSQL.PostgreSQL.17` `--custom "--superpassword <DB.SuperPassword> --serverport <DB.Port>"`; find `psql.exe`; log in as `postgres` with `DB.SuperPassword`, else `Job.SuperPassword`; neither works → error `ErrNeedSuperPassword` ("PostgreSQL is already on this PC — enter its 'postgres' password"); then role / database as karots-pos.
- `program`: when updating `Stop-ScheduledTask KarotsSuite` + stop the process whose path is the installed exe, wait for exit; copy.
- `settings`, `backups`: as Linux (ACL: owner only via `icacls`).
- `startup`: Scheduled Task `KarotsSuite`, at logon of User, Interactive, Highest, restart on failure; action `powershell.exe -WindowStyle Hidden -Command "& '<exe>' *>> '<dir>\logs\karots.log'"` (no window; logoff / shutdown still reach the app as SIGTERM, M8a), WorkingDirectory InstallDir; start it.
- `network`: `New-NetFirewallRule -DisplayName "Karots Suite"` for Port. `printing`: no step on Windows (Applies false).
- `open`: kiosk = Chromium / Chrome via winget, `kiosk.cmd` relaunch loop + Startup shortcut (as karots-pos); background = `Karots Suite.url` (`URL=http://localhost:Port`) on the user's Desktop and Start Menu.

- [ ] **Step 1: Write the failing tests** on the recorded scripts: winget line carries the profile's super password and port; `ErrNeedSuperPassword` when both logins fail (fake runner answers); a typed `Job.SuperPassword` is used on the retry; updating stops the task before `Copy-Item`; task action contains `-WindowStyle Hidden`; background writes `.url` files, no `kiosk.cmd`; no `printing` step on Windows.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** **Step 4: Run** → PASS (tests run on Linux; scripts are strings).
- [ ] **Step 5: Commit** `feat(setup): Windows steps — PostgreSQL, program, .env, Scheduled Task, firewall, kiosk / background`

### Task 9: Setup page + elevation

**Files:**
- Create: `cmd/setup/page.go`, `cmd/setup/page.html`, `cmd/setup/page_test.go`, `cmd/setup/elevate_windows.go`, `cmd/setup/elevate_unix.go`
- Modify: `cmd/setup/main.go`

Flow: **launcher** (not admin) picks a free loopback port + 32-hex token, starts the **worker** elevated with `-addr -token -user <current user>` (Windows `ShellExecute` verb `runas` via `x/sys/windows`; Linux `pkexec <self> …`; no `pkexec` → print "run: sudo <path>" and open nothing), waits for the worker's page, opens it in the user's browser, exits. Already admin (e.g. `sudo`) → serve directly, user from `SUDO_USER` / `PKEXEC_UID`; root with neither → "run from your normal user".

Worker routes (every request needs `?t=` / `X-Setup-Token` = token, else 403):
- `GET /` page · `GET /api/plan?role=&address=` → `{shop, installId, updating, steps:[{id,title,detail}]}` (detail = the plain line from spec §E "What will happen")
- `POST /api/test-address` `{address}` → `{ok, message}` (GET `http://address/health`)
- `POST /api/run` `{role, address, superPassword}` → streamed log with `@@STEP <id> ok` / `@@STEP <id> fail <message>` / `@@NEED superPassword` / `@@DONE`
- `POST /api/close` → exits; the worker also exits after 30 min idle.

Page (spec §E): shop name, **This PC is…** (main PC / cashier PC + address + Test), What will happen, **Install** / **Update**, ticks + hidden Details log, failure words + Try again (+ the postgres password field on `@@NEED`), Done → Open the till + install id line ("support reads the PIN from this"), Close.

- [ ] **Step 1: Write the failing tests**: no / wrong token → 403 on `/` and `/api/plan`; plan for a file-mode conf has no `database`; run with a fake step list streams `@@STEP` lines then `@@DONE`; `ErrNeedSuperPassword` → `@@NEED superPassword`; text/plain POST → 415.
- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** **Step 4: Run** → PASS.
- [ ] **Step 5:** run the worker against a temp folder with a fake runner (`-dry` flag: records commands, runs nothing) and screenshot the page — main PC, cashier PC, a failure, Done — at 1366 and 360, Studio-like Builder look, self-review.
- [ ] **Step 6: Commit** `feat(setup): the install page — role, what will happen, live steps, Try again; UAC / pkexec elevation`

### Task 10: build.ps1, docs, shops.json, real installs

**Files:**
- Create: `build.ps1`, `shops.json` (via `-import-pos ../karots-pos/shops.json`)
- Modify: `Makefile` (`setup:` = `go build -o bin/karots-setup ./cmd/setup`), `AGENTS.md` (Layout: `cmd/setup/`, `internal/shopconf/`, `shops.json`; Dev workflow line), `docs/status.md`

- [ ] **Step 1:** `build.ps1` targets `bootstrap [args]`, `build`, `test`, `test-web`, `types`, `web`, interactive menu with no args (karots-pos shape; needs Go + bun).
- [ ] **Step 2:** `go run ./cmd/bootstrap -import-pos ../karots-pos/shops.json`; check install ids match karots-pos; commit `shops.json`.
- [ ] **Step 3: Real install, Linux** — in a throwaway systemd container / VM: file-mode shop `-yes` install → service up, `/health` 200, re-run as Update after Rotate jwt → restarted, data kept; server-local shop → role password = profile's. Record results in status. Windows double-click + USB exec bit + pkexec dialog: listed in status for the user's VM check.
- [ ] **Step 4: Verify** `go test ./cmd/... ./internal/shopconf/`, `make verify`, `make check-core`, `make verify-portable`.
- [ ] **Step 5: Commit** `docs: M8b Shop Builder done — status, AGENTS layout; shops.json imported from karots-pos`
