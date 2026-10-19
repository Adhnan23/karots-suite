# M8b — Shop Builder: shop profiles + install scripts (design)

The user, 2026-10-18: the Shop Builder keeps a **profile per shop** (plugins, database mode, install id, update channel,
and that shop's `.env`: JWT secret, DB URL), and karots-pos-style **install scripts** (`install.sh`, `build.ps1`,
`install.ps1`) put the profile's `.env` beside the binary. Self-update and publishing are **M8c**; this part only makes
"build shop X again, exactly as last time, and install it" one click.

## 1. How it works today (checked 2026-10-19)

- **Suite `cmd/bootstrap`** (824 lines): a loopback browser app + flags. Picks plugins (with `requires`), OS / CPU,
  binary name, **mode** (server | file), install id; builds into `dist/`. It remembers nothing: every build re-picks
  everything, and a shop's install id must be typed back in by hand or its support PIN changes.
  `POS_SUPPORT_SECRET` unset → it warns and builds with the shared fixed PIN (the Windows VM's 2273).
- **karots-pos `cmd/release`** keeps `shops.json` (key, name, os, arch, plugins, install_id, channel), **committed** —
  it holds no secrets. `add` / `list` / `release` / `-no-upload`. The channel is the shop's secret bucket folder (M8c).
- **karots-pos install scripts:** `install.sh` (Linux, apt / dnf / pacman / zypper: PostgreSQL + CUPS + Chromium,
  `pos_db` role with a generated password, `/opt/karots-pos` + `.env` + `backups/`, `-init`, systemd service, Chromium
  kiosk relaunch loop + DevTools policy; `SERVER=<ip>` = cashier terminal, kiosk only; `KIOSK=no`). `install.ps1` is the
  Windows twin (winget PostgreSQL + Chrome, `C:\karots-pos`, logon Scheduled Task, firewall, kiosk). Both **generate**
  the DB password and JWT secret on the shop PC (reusing an installed `.env` on a re-run). `build.ps1` is a Windows
  stand-in for the Makefile (Windows has no make / bash).
- **Suite config:** `DATABASE_URL` + `JWT_SECRET` required in server mode; a file-mode binary needs **no `.env`**
  (secret in `data/secret`, backups in `data/backups`), but still reads one beside it (e.g. `SERVER_PORT`).

## 2. What it does

### A. One profiles file: `shops.json`

At the repo root, **git-ignored**, written `0600`; `-shops <path>` (or `SHOPS_FILE`) points elsewhere. It holds secrets
(each shop's JWT secret and DB password) — unlike karots-pos's committed one — so it never goes in git. **It is the
only copy of each shop's install id and (M8c) update channel**: the Builder says so on first save and the user keeps a
copy (private repo or their own backups).

```json
{ "shops": [ {
  "key": "al-raj",                 // folder / file name, a-z 0-9 -
  "name": "Al Raj",
  "os": "windows", "arch": "amd64",
  "mode": "file",                  // file | server
  "plugins": ["aicatalog", "alternatives", "productplus"],
  "installId": "7C1E09A2",         // minted on create, never changes
  "channel": "3f9c…32 hex",        // minted on create, used by M8c
  "env": { "SERVER_PORT": "3000" } // that shop's .env, KEY → value
} ] }
```

- **`env`** is the shop's `.env`. On create: server mode gets `JWT_SECRET` (48 hex), `DATABASE_URL`
  (`postgres://suite_user:<32 hex>@localhost:5432/suite_db?sslmode=disable`), `SERVER_PORT=3000`, `APP_ENV=production`,
  `BACKUP_DIR=backups`; file mode gets only `SERVER_PORT=3000` (its secret lives in `data/`). Any other key can be added
  (CORS, `JWT_EXPIRES_IN`, …). `POS_SUPPORT_SECRET` and `POS_SYSTEM_PIN` are **refused** in `env` (the secret is baked,
  never shipped; a fixed PIN defeats the rotation).
- **No "version it runs"** field yet — that is M8c's (the feed knows it).

### B. Builder app: shops first

The page opens on the **shop list** (name, OS, mode, plugin count, last built). Per shop: **Build**, **Edit**,
**Download .env**, **Delete** (asks; says the install id and channel go with it). **+ New shop** is today's form plus
name / key. **Build without a shop** stays (a quick one-off binary, nothing saved — today's flow).

Edit shows plugins, OS / CPU, mode, the `.env` as key / value rows (secrets masked with Show), install id and channel
read-only. Changing **mode** server → file drops `DATABASE_URL` from `env` after a confirm ("move the data first:
Setup → Data has the steps"); file → server adds a fresh one.

### C. A shop build is an install folder

`dist/<key>/` holds everything to carry to the shop on a USB stick:

```
dist/al-raj/
  karots-suite.exe     the binary (its install id + support seed baked in)
  .env                 from the profile (0600); left out when env is empty
  install.ps1          (windows) — or install.sh (linux)
  INSTALL.txt          five lines: run the script as admin, the till URL, support login
```

- A shop build **refuses to start without `POS_SUPPORT_SECRET`** (a quick build still only warns). A shop binary with
  the shared PIN is a support hole.
- Before writing, `dist/<key>/` is emptied; the build records `lastBuilt` (time + commit) on the profile.
- The scripts are **embedded in the Builder** (`cmd/bootstrap/install/`, `go:embed`) and copied in, so the folder works
  without the repo.

### D. CLI

`go run ./cmd/bootstrap -shop al-raj` builds a profile (`-shop all` builds every shop, one after another). `-list`
prints the profiles. `-import-pos ../karots-pos/shops.json` adds karots-pos shops: **same key, name, OS / CPU, install
id** (the support PIN the shop knows stays), plugins mapped by key (unknown ones listed and dropped), mode server, a
**new channel** (the suite has its own feed — never a karots-pos binary over the suite), fresh secrets. The other flags
stay as they are.

### E. Install scripts (forked from karots-pos, one each)

Same shape and guarantees as karots-pos (idempotent, distro-agnostic, kiosk relaunch loop, DevTools policy, cashier
terminal via `SERVER=<ip>` / `-Server`, `KIOSK=no` / `-NoKiosk`). Renamed for the suite: `/opt/karots-suite`,
`C:\karots-suite`, service / task `karots-suite`, `suite_db` / `suite_user`. Changes:

1. **The `.env` beside the script wins.** Order: `.env` next to the binary (the profile's) → the installed one (re-run)
   → generate (a binary built without a profile). For server mode the script creates `suite_user` **with the password
   in that `DATABASE_URL`**, so the profile and the shop always agree.
2. **Mode from the `.env`:** a `DATABASE_URL` → server mode (as karots-pos). None → **file mode**: no PostgreSQL install,
   no role, no `-init` step, the service has no `postgresql` dependency, the app user owns the folder (it writes `data/`).
   Firewall, CUPS, kiosk and the cashier-terminal path are the same in both.
3. **Server elsewhere:** `DATABASE_URL` host is not `localhost` / `127.0.0.1` → skip installing PostgreSQL here (a LAN
   server or VPS already has it); the script checks it can reach that port and says so if not.
4. **Health check** waits on `/health`, port from the `.env`.
5. Windows file mode: the Scheduled Task runs the exe at logon as the user (as karots-pos), so closing its window =
   SIGTERM = Postgres stopped cleanly (M8a).

`build.ps1`: the Windows stand-in for the Makefile, cut down to what the suite needs on Windows — `bootstrap [args]`
(the Builder), `build`, `test`, `test-web`, `types`, `web`. Needs Go + bun on PATH. Lives at the repo root (it builds),
not in install folders.

### F. Safety

- The Builder stays on loopback with the JSON-only POST guard it has; the list / edit endpoints follow the same rule.
  `GET` of a shop masks secrets; **Download .env** is a POST (a cross-site page can't trigger it).
- `shops.json` is written atomically (temp file + rename) so a crash mid-save never loses every shop.

## 3. Not in M8b

- Publishing, the feed, self-update, "who's behind" — **M8c**.
- Moving a karots-pos shop's data — already works (karots-pos backup → suite Restore).
- A zip of the install folder (a folder copies to USB fine).
- macOS / FreeBSD install scripts (builds still allowed; no shop runs them).

## 4. Checks

- Go tests: profile create mints install id / channel / secrets per mode; refused keys; mode switch edits `env`;
  `-import-pos` keeps install ids, drops unknown plugins, mints a new channel; atomic save; app endpoints (list masks,
  JSON-only POSTs, build writes `dist/<key>/` with the right script and `.env`).
- Script checks (by hand, recorded in status): `install.sh` on a Linux VM in **file mode** and **server mode with the
  profile `.env`** (role password matches, service up, kiosk) and a re-run; `install.ps1` file mode on the Windows 10
  VM from M8a (fresh, no redist); `shellcheck install.sh`.
- `make verify`, `make check-core` (the Builder names plugins only via `plugin.json`, as today).

## 5. Decisions for the user

1. **`shops.json` git-ignored at the repo root** (recommended — it holds secrets) vs a private repo of its own.
2. **Default shop port 3000** (config's default; karots-pos uses it too, so a karots-pos PC is switched over, not run
   side by side) vs 3100.
3. **Shop builds refuse without `POS_SUPPORT_SECRET`** (recommended) vs warn only.
4. **`-import-pos`** for the existing karots-pos shops (recommended, small) vs add them by hand.
