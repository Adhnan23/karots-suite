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
    "lanAddress": "192.168.1.10"    // optional: makes a cashier-terminal folder too
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
  install.conf       the install script's settings, generated from the profile
  install.ps1        (windows) — or install.sh (linux)
  INSTALL.txt        what to run, the till URL, the support login
  terminal/          only when the profile has a LAN address:
    install.conf       kiosk-only settings → http://192.168.1.10:3000
    install.ps1
```

- **`.env`** (generated, never typed): `APP_ENV=production`, `SERVER_PORT`, then server mode `DATABASE_URL` (from `db`)
  + `JWT_SECRET` + `BACKUP_DIR=backups`; file mode `FILE_DB_PORT`; then the `env` extras. Written in the
  systemd-`EnvironmentFile`-safe form (KEY=value, no spaces, no comments).
- **`install.conf`** (same KEY=value form): `ROLE=server|terminal`, `MODE=file|server`, `APP=karots-suite`,
  `BINARY=<name>`, `INSTALL_DIR`, `PORT`, `KIOSK=yes|no`, `KIOSK_URL`, and for a local server-mode DB `DB_NAME`,
  `DB_USER`, `DB_PASSWORD`, `DB_PORT`, `DB_LOCAL=yes|no` (host is not this PC → `no`).
- A shop build **refuses without `POS_SUPPORT_SECRET`** (a quick build still only warns).
- `dist/<key>/` is emptied first; the build records `lastBuilt` (time + commit) on the profile.
- The scripts are **embedded in the Builder** (`cmd/bootstrap/install/`, `go:embed`) and copied in, so the folder works
  without the repo.

### D. CLI

`go run ./cmd/bootstrap -shop al-raj` builds a profile (`-shop all` every shop, one after another). `-list` prints the
profiles. `-import-pos ../karots-pos/shops.json` adds karots-pos shops: **same key, name, OS / CPU, install id** (the
support PIN the shop knows stays), plugins mapped by key (unknown ones listed and dropped), mode server, port 3000, a
**new channel** (the suite has its own feed — never a karots-pos binary over the suite), fresh secrets. The other flags
stay as they are.

### E. Install scripts, redone: apply, don't decide

The scripts **only apply** what the Builder wrote. They read `install.conf` beside them and copy the `.env` as it is.
They never generate a secret, never parse a `.env`, take no role / port / folder from the shop PC's environment, and
**refuse to run without `install.conf`** ("build this shop in the Shop Builder"). Re-running is safe and is how a
changed profile reaches the shop: new `.env`, `ALTER ROLE … PASSWORD` to the profile's, service restarted.

`install.sh` / `install.ps1`, each as numbered steps keyed on `install.conf`:

1. **Preflight:** admin / root, systemd (Linux) or PowerShell 5+ (Windows), the binary beside the script, static-binary
   check (Linux), not a karots-pos folder.
2. **Packages:** Chromium / Chrome if `KIOSK=yes`; CUPS (Linux) on a server; PostgreSQL **only** when `MODE=server` and
   `DB_LOCAL=yes`.
3. **Database** (server + local only): init + start PostgreSQL, role + database with `DB_PASSWORD` on `DB_PORT`, loopback
   password login in `pg_hba` (as karots-pos). `DB_LOCAL=no`: check the DB host's port answers, say so if not.
4. **Files:** binary + `.env` (0600) into `INSTALL_DIR`, owned by the shop user (file mode writes `data/` there).
5. **Start on boot:** systemd service (Linux; `After=postgresql` only when the DB is local) / logon Scheduled Task as
   the shop user (Windows — closing the window stops Postgres cleanly, M8a). Firewall opens `PORT` for the LAN.
6. **Wait** on `http://localhost:PORT/health`.
7. **Kiosk** (`KIOSK=yes`): the karots-pos relaunch loop + DevTools policy, opening `KIOSK_URL`.

`ROLE=terminal` runs only 1, 2 (browser) and 7. The kiosk-exit sentinel stays the same file the server writes. Expected
size: well under karots-pos's, since the generating, re-reading and role guessing go.

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
- macOS / FreeBSD install scripts (builds still allowed; no shop runs them).

## 4. Checks

- Go tests: create mints install id / channel / secrets per mode; refused `env` keys; port clash warning; the
  generated `.env` and `install.conf` for server-local, server-remote, file and terminal; `-import-pos` keeps install
  ids, drops unknown plugins, mints a new channel; atomic save keeps the old file on a failed write; app endpoints (list
  masks, JSON-only POSTs, build writes `dist/<key>/` with the right script).
- Script checks (by hand, recorded in status): `install.sh` on a Linux VM in **file mode**, **server mode** and a
  re-run after **Rotate** (role password follows, service up, kiosk); a terminal folder against it; `install.ps1` file
  mode on the M8a Windows 10 VM (fresh, no redist); `shellcheck install.sh`; a folder without `install.conf` is refused.
- `make verify`, `make check-core` (the Builder names plugins only via `plugin.json`, as today).
