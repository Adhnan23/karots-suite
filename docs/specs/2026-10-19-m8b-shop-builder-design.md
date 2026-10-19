# M8b — Shop Builder: shop profiles + install scripts (design)

The user, 2026-10-18: the Shop Builder keeps a **profile per shop** (plugins, database mode, install id, update channel,
and that shop's `.env`: JWT secret, DB URL), and karots-pos-style **install scripts** (`install.sh`, `build.ps1`,
`install.ps1`) put the profile's `.env` beside the binary. Self-update and publishing are **M8c**; this part only makes
"build shop X again, exactly as last time, and install it" one click.

**User, 2026-10-19:** `shops.json` is **committed** (the repo goes private); every shop has its **own ports** in the
Builder; **redo the install scripts** so the Builder makes the config and the scripts only apply it; the rest as
recommended (shop builds refuse without `POS_SUPPORT_SECRET`, default port 3000, `-import-pos`).

## 1. How it works today (checked 2026-10-19)

- **Suite `cmd/bootstrap`** (824 lines): a loopback browser app + flags. Picks plugins (with `requires`), OS / CPU,
  binary name, **mode** (server | file), install id; builds into `dist/`. It remembers nothing: every build re-picks
  everything, and a shop's install id must be typed back in by hand or its support PIN changes.
  `POS_SUPPORT_SECRET` unset → it warns and builds with the shared fixed PIN (the Windows VM's 2273).
- **karots-pos `cmd/release`** keeps `shops.json` (key, name, os, arch, plugins, install_id, channel), committed.
  `add` / `list` / `release` / `-no-upload`. The channel is the shop's secret bucket folder (M8c).
- **karots-pos install scripts** decide a lot on the shop PC: they **generate** the DB password and JWT secret (or dig
  them back out of an installed `.env` with `sed` on a re-run), hard-code the port, DB name / user and folders, and take
  the role (server / cashier terminal / no kiosk) from environment variables typed at the shop. So nothing at home knows
  what a shop really runs, and the scripts are long (`install.sh` 383, `install.ps1` 323 lines).
- **Suite config:** `DATABASE_URL` + `JWT_SECRET` required in server mode; a file-mode binary needs **no `.env`**
  (secret in `data/secret`, backups in `data/backups`), but reads one beside it (`SERVER_PORT`, `FILE_DB_PORT`).
  The app ignores keys it doesn't know.

## 2. What it does

### A. One profiles file: `shops.json` (committed)

At the repo root, **committed** (the repo is private; it is the record of every shop). `-shops <path>` points
elsewhere. It holds each shop's secrets (JWT secret, DB password) and is the **only copy** of each shop's install id
and update channel. Saved atomically (temp file + rename), keys sorted, so diffs stay readable and a crash mid-save
never loses every shop. The Builder commits nothing itself — the user commits it like any other change.

```json
{ "shops": [ {
  "key": "al-raj",                  // folder name, a-z 0-9 -
  "name": "Al Raj",
  "os": "windows", "arch": "amd64",
  "mode": "server",                 // file | server
  "plugins": ["aicatalog", "alternatives", "productplus"],
  "installId": "7C1E09A2",          // minted on create, never changes
  "channel": "3f9c…32 hex",         // minted on create, used by M8c
  "port": 3000,                     // the till's HTTP port
  "db": {                           // server mode only
    "host": "localhost", "port": 5432,
    "name": "suite_db", "user": "suite_user", "password": "…32 hex…"
  },
  "fileDbPort": 5434,               // file mode only: its own PostgreSQL (loopback)
  "jwtSecret": "…48 hex…",          // server mode only (file mode keeps its own in data/)
  "install": {
    "dir": "C:\\karots-suite",      // default /opt/karots-suite | C:\karots-suite
    "kiosk": true,
    "lanAddress": "192.168.1.10"    // optional: pre-fills a cashier PC's main-PC address
  },
  "env": { "JWT_EXPIRES_IN": "10h" }, // any extra .env keys
  "lastBuilt": { "at": "2026-10-19T10:00:00Z", "commit": "fbf53cd" }
} ] }
```

- **Ports are per shop** (port, DB port, file-DB port), defaults 3000 / 5432 / 5434. The Builder warns when two of a
  shop's ports clash, and when the till port is 3000 on a PC that still runs karots-pos (it's the same port — the suite
  replaces it there, not beside it).
- Secrets are minted on create; **Rotate** next to each one makes a new value (a new JWT secret logs everyone out; a new
  DB password is applied by the next install run — §E).
- `env` refuses `POS_SUPPORT_SECRET` and `POS_SYSTEM_PIN` (the secret is baked, never shipped; a fixed PIN defeats the
  rotation) and the keys the fields above own.
- No "version it runs" yet — that is M8c's (the feed knows it).

### B. Builder app: shops first

The page opens on the **shop list** (name, OS, mode, port, plugin count, last built). Per shop: **Build**, **Edit**,
**Download .env**, **Delete** (asks; says the install id and channel go with it). **+ New shop** is today's form plus
name / key and the fields above with their defaults. **Build without a shop** stays (a quick one-off binary, nothing
saved, no install folder — today's flow).

Edit groups the profile as **Shop** (name, plugins, OS / CPU), **Database** (mode, then DB host / port / name / user /
password or the file-DB port), **Network** (till port, LAN address), **Install** (folder, kiosk), **More settings**
(extra `.env` rows). Secrets are masked with Show (screen sharing). Install id and channel are read-only. Changing mode
server → file asks first ("move the data first: Setup → Data has the steps").

### C. A shop build is an install folder

`dist/<key>/` holds everything to carry to the shop on a USB stick:

```
dist/al-raj/
  karots-suite.exe   the binary (install id + support seed baked in)
  .env               the app's settings, generated from the profile (0600)
  install.conf       the installer's settings, generated from the profile
  karots-setup.exe   the installer (§E), built for the shop's OS / CPU
  INSTALL.txt        "double-click karots-setup (Linux: sudo ./karots-setup)", the support login
```

- **`.env`** (generated, never typed): `APP_ENV=production`, `SERVER_PORT`, then server mode `DATABASE_URL` (from `db`)
  + `JWT_SECRET` + `BACKUP_DIR=backups`; file mode `FILE_DB_PORT`; then the `env` extras. Written in the
  systemd-`EnvironmentFile`-safe form (KEY=value, no spaces, no comments).
- **`install.conf`** (same KEY=value form): `SHOP=<name>`, `MODE=file|server`, `BINARY=<name>`, `INSTALL_DIR`,
  `PORT`, `KIOSK=yes|no`, `LAN_ADDRESS` (may be empty), and for server mode `DB_NAME`, `DB_USER`, `DB_PASSWORD`,
  `DB_PORT`, `DB_LOCAL=yes|no` (host is not this PC → `no`).
- A shop build **refuses without `POS_SUPPORT_SECRET`** (a quick build still only warns).
- `dist/<key>/` is emptied first; the build records `lastBuilt` (time + commit) on the profile.
- The installer is generic (one program, every shop): it reads `install.conf` and `.env` from beside itself, so the
  Builder just cross-compiles `cmd/setup` for the shop's OS / CPU into the folder (static, like the server).

### D. CLI

`go run ./cmd/bootstrap -shop al-raj` builds a profile (`-shop all` every shop, one after another). `-list` prints the
profiles. `-import-pos ../karots-pos/shops.json` adds karots-pos shops: **same key, name, OS / CPU, install id** (the
support PIN the shop knows stays), plugins mapped by key (unknown ones listed and dropped), mode server, port 3000, a
**new channel** (the suite has its own feed — never a karots-pos binary over the suite), fresh secrets. The other flags
stay as they are.

### E. Shop Setup: an installer program with a page, no command line

**User, 2026-10-19:** the installer is its own binary with a web page — people don't like the command line. So the
shop never types anything: **double-click `karots-setup`**, a page opens, press **Install**.

`cmd/setup` → `karots-setup(.exe)`, one generic program for every shop (it reads `install.conf` + `.env` from beside
itself), the same loopback page + live log pattern as the Builder (`app.html`, `go:embed`, no React).

**Getting admin rights without a terminal:**
- **Windows:** not elevated → it relaunches itself through the normal UAC prompt ("Yes"), then carries on.
- **Linux:** not root → it relaunches itself with **`pkexec`** (the desktop's graphical password box). The page is
  opened **as the logged-in user** (a browser refuses to run as root), so the shop user is also who the service and
  kiosk run as. No `pkexec` (rare) → the page still opens and says the one line to run.
- The page's address carries a random token (it runs as admin; no other site or local user can drive it).

**The page** (the shop's name at the top, from `install.conf`):
1. **This PC is…** **The shop's main PC** (the till server; the default) or **A cashier PC** — kiosk only, pointed at
   the main PC (address pre-filled from `LAN_ADDRESS`, editable, with **Test** that calls its `/health`). This replaces
   a separate terminal folder.
2. **What will happen** — plain lines from `install.conf`: "Install Chrome", "Keep the shop's data on this PC (one PC)"
   or "Install PostgreSQL" / "Use the database on 192.168.1.20", "Start Karots when this PC starts", "Open the till
   full-screen", the folder and port. Nothing to fill in — the Builder decided it.
3. **Install** → each step ticks ✓ with a live log under **Details** (hidden by default). A failed step says what to do
   in words and offers **Try again**; Copy log for support.
4. **Done** → **Open the till** + the support login line; on a cashier PC, "the till opens full-screen at next login".

Already installed (folder exists) → the page opens on **Update** instead: same steps, applies a changed profile (new
`.env`, `ALTER ROLE … PASSWORD`, service restarted), data untouched. **Uninstall** is not offered (a shop's data must
not vanish with one click).

**Apply, don't decide.** It never generates a secret, never edits the `.env`, and **refuses without `install.conf`**
("build this shop in the Shop Builder"). The steps, ported from karots-pos's scripts into Go (one file per concern, all
≤ 500 lines, `exec` of the OS tools — no new dependency; `golang.org/x/sys` is already in `go.mod`):

1. **Checks:** admin, systemd (Linux), the shop binary beside it, not over a karots-pos folder, port free (or ours).
2. **Programs:** Chromium / Chrome when the kiosk is on (apt / dnf / pacman / zypper, winget on Windows); CUPS (Linux);
   PostgreSQL **only** for server mode with a local DB.
3. **Database** (server + local): start PostgreSQL, role + database with the profile's password on its port, loopback
   password login in `pg_hba`. Remote DB: check it answers, say so if not.
4. **Files:** binary + `.env` (0600) into the install folder, owned by the shop user (file mode writes `data/` there).
5. **Start with the PC:** systemd service (Linux) / logon Scheduled Task as the shop user (Windows — closing the window
   stops Postgres cleanly, M8a); firewall opens the port for the LAN.
6. **Wait** on `/health`.
7. **Kiosk:** karots-pos's relaunch loop + DevTools policy, opening the till (or the main PC's address).

A cashier PC runs 1, 2 (browser only) and 7. **Headless Linux** (a VPS over SSH, no screen): `sudo ./karots-setup
-yes` runs the same steps in the terminal — the only command-line path, for the technician.

`build.ps1`: the Windows stand-in for the Makefile, cut down to what the suite needs on Windows — `bootstrap [args]`
(the Builder), `build`, `test`, `test-web`, `types`, `web`. Needs Go + bun on PATH. Lives at the repo root (it builds),
not in install folders.

### F. Safety

- The Builder stays on loopback with its JSON-only POST guard; the new endpoints follow it. A shop `GET` masks
  secrets; **Download .env** and Show are POSTs (a cross-site page can't read them).
- `.env` and `install.conf` in `dist/` are 0600; `dist/` stays git-ignored.

## 3. Not in M8b

- Publishing, the feed, self-update, "who's behind" — **M8c**.
- Moving a karots-pos shop's data — already works (karots-pos backup → suite Restore).
- A zip of the install folder (a folder copies to USB fine).
- Setup for macOS / FreeBSD (builds still allowed; no shop runs them).
- karots-pos-style `install.sh` / `install.ps1` — replaced by Shop Setup.

## 4. Checks

- Go tests: create mints install id / channel / secrets per mode; refused `env` keys; port clash warning; the
  generated `.env` and `install.conf` for server-local, server-remote and file; `-import-pos` keeps install
  ids, drops unknown plugins, mints a new channel; atomic save keeps the old file on a failed write; app endpoints (list
  masks, JSON-only POSTs, build writes `dist/<key>/` with the binary, setup, `.env`, `install.conf`).
- Setup tests: `install.conf` parsing (missing → refused), the step list per role / mode (server-local,
  server-remote, file, cashier PC), the page token, Update vs Install detection; the OS commands behind a small runner
  so tests see the commands without running them.
- By hand on VMs (recorded in status): **double-click from a USB stick** on Linux (Xubuntu / Mint: the exec bit and
  `pkexec` prompt) and Windows 10 (UAC), file mode and server mode, an Update after **Rotate** (role password follows,
  service up, kiosk), a cashier PC against it; `-yes` over SSH.
- `make verify`, `make check-core` (the Builder names plugins only via `plugin.json`, as today).
