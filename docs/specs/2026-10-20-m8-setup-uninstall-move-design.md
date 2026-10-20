# Setup: Uninstall, Move to another PC, Mode change — design

The user, 2026-10-20, after the Xubuntu file + server mode checks: Setup needs (1) an **Uninstall** for Windows and
Linux that removes any install — server mode or file mode — but **backs the shop up first and keeps every backup**,
(2) a way to **move a shop to another PC** (uninstall here → the same three files on the new PC → the shop comes back
from the backup, either after login on the Data page or by giving Setup the backup at install), and (3) Setup must
**notice a mode change** (file → server or server → file) on an Update and carry the shop's data across by itself.

## Why

- Today there is no uninstall: a test VM, a broken install or a shop leaving a PC is cleaned by hand (service,
  `/opt/karots-suite`, autostart, policies) — and the file-mode shop's data lives *inside* that folder, so a careless
  `rm -rf` takes the shop with it.
- A mode change today silently goes wrong (checked in the code, 2026-10-20):
  - **Over-the-air Update** (Data → Update) only swaps the binary: the new mode's binary can't start with the old
    `.env` → BootGuard rolls back after two failed boots. Safe, but nothing moves. **Out of scope** — a mode change
    is a Setup job; that rollback is the right answer for an OTA.
  - **Setup Update, file → server:** Setup makes an **empty** server database and starts on it. The shop's data is
    still in `data/`, but nothing reads it — the owner sees an empty shop.
  - **Setup Update, server → file:** `applyInstalled` (`cmd/setup/installed.go`) keeps every `.env` line the Builder
    didn't send, so the old `DATABASE_URL` is **kept** and the shop quietly stays in server mode. **A bug — fixed
    here** whatever else is built.

## Goals

1. **Uninstall** (Setup page button + `-uninstall` for technicians), Windows and Linux, main PC (file or server mode)
   and cashier PC.
2. **Bring back a shop at install** — a first install can restore a backup before the till first opens.
3. **Mode change on Update** — detected, explained, confirmed, and done: backup → new database → restore.
4. Backups are never deleted by any of this.

## Non-goals

- No change to the Data page restore (it keeps working, after login, exactly as today).
- No OTA mode change (see Why).
- No removing Google Chrome / Chromium / curl / CUPS / the VC++ files Setup may have installed — other things use them.
- No moving data between two PCs over the network: the backup file is carried (USB stick, cloud folder).

## Anti-goals (a failure even if it "works")

- **Any path that ends with the shop's data gone and no backup on disk.** If the final backup fails, nothing is
  removed.
- A mode change that starts the till on an empty database.
- A restore that runs while the till is taking sales (the service is stopped first).

## The pieces

### 1. Two program flags (cmd/server)

Setup never talks to a database itself; it asks the shop's own program, which already knows how:

- **`karots-suite -backup <folder>`** — writes one full snapshot (`backup.WriteSnapshot`, tag `final` or
  `before-move`) into `<folder>` and prints its path. In file mode (no `DATABASE_URL`, file-mode build) it starts the
  shop's own PostgreSQL as usual (`startFileMode`), dumps, and stops it. Runs **no migrations** — it is the *installed*
  (old) binary dumping its own schema.
- **`karots-suite -restore <file>`** — migrates (core + plugins) then `backup.Restore`s the file, prints the
  `RestoreReport` (rows, skipped tables, dropped columns), exits non-zero on failure. File mode starts / stops its own
  PostgreSQL the same way. It is the *new* binary restoring into its own schema (Restore already copes with an older
  or newer backup).

Both refuse to run while the till's port is answering (the service must be stopped — `startFileMode` already
claims the port first; server mode checks the port too).

### 2. The backup folder

One folder for the final backup, a move, and finding a backup at install:

- **Windows:** `C:\Karots backups` · **Linux:** `~/Karots backups` (the shop user's home, owned by them).
- Setup suggests it; the person can pick another (the existing folder picker, `folders.go`) — e.g. a USB stick.
- File mode's automatic backups (`data/backups`, or a `BACKUP_DIR` inside the install folder) are **copied** there
  before the install folder is removed. A `BACKUP_DIR` outside the install folder is left where it is.

### 3. Uninstall

On the Setup page, when Karots is installed on this PC (as today's Update detection: the binary in `InstallDir`):
an **Uninstall** button beside Install / Update. Headless: `karots-setup -uninstall -yes [-backup-dir …]
[-remove-postgres]`.

Page: what will be removed (the same "What will happen" list style), the backup folder (changeable), and on a
server-mode main PC whose database is on this PC a checkbox **"Also remove PostgreSQL"** — **off** by default.
A confirm step ("Uninstall Karots from this PC").

Steps, in order (main PC):

1. **Final backup** — stop the service, `karots-suite -backup <folder>` (tag `final`), copy the automatic backups (§2).
   **If it fails, stop here** — nothing removed; the page shows why. (Technicians: `-no-backup` skips it, never on the
   page.)
2. **Stop and remove startup** — Linux: disable + remove the systemd unit, `daemon-reload`. Windows: what
   `windowsStartup` made (karots-start, its scheduled task / startup entry), kill `karots-start`.
3. **Remove the till opening** — kiosk script + autostart entry, Chromium/Chrome DevTools policy files Setup wrote,
   desktop + app-menu shortcuts (Linux), the Windows equivalents from `windowsOpen`; kill a running kiosk browser.
4. **Network** — remove the firewall rule Setup added for the port (when it added one).
5. **Database (server mode, local DB only)** — drop the shop's database and its role. With "Also remove PostgreSQL":
   uninstall the package (apt / dnf / pacman / zypper; Windows: the EDB uninstaller, unattended). A remote database
   (`DB.Local` false) is never touched.
6. **Remove the program folder** — `InstallDir` with `.env`, `setup.json`, the binary and its `.old` / `.failed`
   copies, and (file mode) `data/`. Only after step 1 succeeded.
7. **Done** — "Karots is removed. The shop's backup is in `<path>` — keep it: it brings the shop back on any PC."

Cashier PC: steps 3 only (no data there), no backup.

### 4. Bring back a shop at install

On a **first install** (not an Update) of a main PC, the page has a section **"Bring back a shop"**:

- Setup looks for `*.json.gz` backups in the default backup folder (§2) and beside `karots-setup` (a USB stick), and
  offers the **newest**, showing its shop name and date (read from the archive without touching any database).
  "Choose another…" opens the folder picker. Default: **don't restore** unless a backup was found *beside Setup* —
  my call: a file deliberately put next to the three files means "bring this one back"; one merely found in the home
  folder is offered but not ticked.
- When chosen, a step **"Bring back the shop"** runs after Settings and before Start with the PC:
  `karots-suite -restore <file>`, its report written to the log and summarised on the page ("12 403 rows restored").
- The install id stays the one the binary was built with (`adoptBakedInstallID` already makes the database agree on
  every start), so the support PIN keeps working after a restore from an older PC.
- Headless: `-restore <file>`.

### 5. Mode change on Update

Setup knows the **installed mode** from the installed `.env`: `DATABASE_URL` present → server, else (a `data/`
folder beside the binary) → file. When it differs from `install.conf`'s `Mode`:

- The page says so plainly at the top: **"This changes how the shop is kept: One PC → Server"** (or back), what will
  happen, and needs an explicit confirm. Headless needs `-change-mode`.
- `applyInstalled` **drops `DATABASE_URL`** (and other Builder-owned database keys) from what it keeps — always,
  not only on a mode change (the bug above).
- Steps:
  1. Stop the service; **`before-move` backup** with the *old* binary and old `.env` into the backup folder (§2).
     Failure → stop, nothing changed.
  2. The normal Update steps for the new mode (Database for server mode, Program, Settings…).
  3. **`karots-suite -restore <backup>`** with the *new* binary and new `.env`.
  4. Start, health check as today.
- The old data is **left in place**, renamed so nothing reads it by mistake: file → server renames `data/` to
  `data-before-server-<date>/`; server → file leaves the old server database untouched. The Done screen says where
  both the backup and the old data are, and that they can be removed once the shop looks right.
- Ports don't clash: file mode's own PostgreSQL is on **5434** (`filedb.DefaultPort`), a server-mode one normally
  **5432** — both can run during the change.

## Where things go

- `cmd/server`: `-backup`, `-restore` (new small file, e.g. `cmd/server/backupcli.go`).
- `cmd/setup`: `uninstall.go` (steps + page wiring), `linux_uninstall.go`, `windows_uninstall.go`, `restore.go`
  (find/offer a backup + the restore step), mode detection in `installed.go`, page sections in `page.html`.
  Files stay ≤ 500 lines.
- Tests as today's: the `rec` runner + `root` prefix for Linux paths, PowerShell script assertions for Windows;
  a DB-gated test for `-backup` → `-restore` round trip in both modes.

## Acceptance (user, VMs)

1. Xubuntu file mode: Uninstall → backup in `~/Karots backups`, nothing of Karots left (service, folder, kiosk).
2. Same VM: install server mode with that backup beside Setup → till opens with the old shop's products and sales.
3. Server → file via Release + Setup Update → same data, `DATABASE_URL` gone, old database untouched.
4. File → server the same way; `data-before-server-<date>` left behind.
5. Windows VM: 1 and 2 (and one mode change).
6. Uninstall with "Also remove PostgreSQL" on a server-mode VM → PostgreSQL gone, backup kept.

## Left to the builder

- Exact Windows removal commands (mirror whatever `windowsStartup` / `windowsOpen` / `windowsNetwork` created).
- Whether `-backup` / `-restore` live in `cmd/server/main.go`'s flag block or a small helper file (≤ 500 lines).
- Page layout of the Uninstall and "Bring back a shop" sections — follow the existing Setup page style.
