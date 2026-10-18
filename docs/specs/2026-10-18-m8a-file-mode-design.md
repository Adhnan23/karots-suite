# M8a — File mode (design)

The user, 2026-10-18: do M8a first, "check how the things work", and put file mode into the Shop Builder too.

**File mode** is for a one-PC shop: the binary runs its **own PostgreSQL** from a `data/` folder next to it, so there's
nothing to install and no `.env` to write. **Server mode** (a real PostgreSQL on the LAN or a VPS, `DATABASE_URL`) stays
exactly as it is.

## 1. How it works today (checked 2026-10-18)

- **karots-pos Demo Room** already runs an embedded Postgres (`internal/demodb`, `fergusstrange/embedded-postgres`).
  It is a real Postgres subprocess (`pg_ctl` under the hood), not SQLite, so all of the suite's Postgres-only SQL works.
  But it **wipes its data on every start** and **downloads Postgres (~15 MB) on the first run**. A shop needs neither.
- **The suite's backup / restore is pure SQL** over the app's own connection (`internal/backup`: no `pg_dump`, no
  `psql`). So Backup, Restore, auto-backup and the karots-pos restore all work in file mode without any change.
- **Config** needs `DATABASE_URL` and `JWT_SECRET` (`internal/config`). Auto-backup is off unless `BACKUP_DIR` is set.
- **Shop Builder** (`cmd/bootstrap`) picks plugins, OS, processor, binary name and install id, then runs `go build`.

**Spike results** (embedded PG 17.5, linux/amd64, scratchpad):

| check | result |
|---|---|
| `pg_trgm` | ✅ ships in the bundle |
| reuse an existing data dir on restart | ✅ (the library skips `initdb` when `PG_VERSION` is there) |
| locale **C** | ❌ Sinhala trigram similarity = 0, `lower('É')` stays `É`, so **search breaks**. Never use plain C. |
| OS default (`en_US.UTF-8`) or **C.UTF-8** | ✅ Sinhala similarity 0.5, `lower` works |
| the app is killed (crash, closed window) | ❌ **Postgres keeps running**, and the next start fails with "process already listening". Fix: `pg_ctl stop -D data` first. ✅ That worked, and the data was reused. |
| bundle size | 14.9 MB `.txz` per OS / CPU |

## 2. What it does

- **A. One binary, two modes, decided at start:**
  - `DATABASE_URL` set → **server mode** (as today).
  - Not set, and the binary was built with file mode → **file mode**.
  - Not set, and built without it → the same error as today.

  So moving a file-mode shop to a server is just **backup → install PostgreSQL → set `DATABASE_URL` in `.env` →
  start → restore**. Going back is the same steps the other way. No rebuild either way.
- **B. Folder layout** (next to the binary; `-data <dir>` overrides it):
  ```
  karots-suite(.exe)
  data/
    pg/        the database (Postgres data dir)
    runtime/   Postgres program, unpacked from the binary on each start
    backups/   automatic backups (on by default in file mode)
    secret     generated JWT secret + DB password (0600)
    postgres.log
  ```
  Copying the whole folder while the app is stopped is a full copy of the shop.
- **C. Postgres comes inside the binary.** A file-mode build `go:embed`s the Postgres bundle for its OS / CPU
  (**+15 MB**). On start it is placed in the library's cache, so **no download ever** and it works offline from the
  first run. Server builds don't carry it (Go build tag `filemode`).
- **D. Start:**
  1. Stop any Postgres left running on `data/pg` from a crash (`pg_ctl stop -m fast`, only if `postmaster.pid` is
     there).
  2. Start Postgres 17 on **127.0.0.1 only**, port `FILE_DB_PORT` (default **5434**, next to dev's 5433).
     Encoding UTF8 with the OS locale; on Linux, if that fails, **C.UTF-8**. Never plain C.
  3. Create the database on first run, run migrations as usual, and serve.
  4. **Stop:** on SIGINT / SIGTERM (Windows maps closing the window, log-off and shutdown to SIGTERM) the server stops
     Echo, then Postgres (`pg_ctl stop -m fast`).
  5. If Postgres won't start, show the last lines of `data/postgres.log` in the error.
- **E. No `.env` needed.** In file mode a missing `JWT_SECRET` is generated once into `data/secret`, and
  **auto-backup is on** into `data/backups` (6 h, keep 28) unless `BACKUP_DIR` says otherwise. Every other `.env`
  setting still works when a `.env` is there.
- **F. Shows where it matters:**
  - Setup → Data says **"File mode: data in <path>"** or **"Server mode"**, plus the "move to a server" steps from A.
  - The system user's Konami info shows the mode.
  - The startup log line says it too.
- **G. Shop Builder.** A new **Database** choice under Build: **One PC (file mode)** / **Server (PostgreSQL)**.
  - File mode: the builder fetches that OS / CPU's Postgres bundle once into its own cache (the only download, done on
    the builder's PC, not the shop's), drops it where the `filemode` build embeds it, builds with `-tags filemode`,
    then removes it. The working tree stays clean, the same way the plugin lists are restored.
  - The summary shows the mode and the size.
  - The command line gets `-mode file|server` (default server).
  - It still builds on Windows and macOS with only Go + bun.
- **H. Refuses clearly:**
  - Run as **root on Linux** → "file mode can't run as root; run it as a normal user". Postgres itself refuses root.
  - A second copy started on the same folder → "already running from this folder".

## 3. Not in M8a

- Self-update stopping and starting Postgres around a swap (M8b, which builds on D's stop).
- A Windows service / auto-start (install scripts, M8d).
- Upgrading the embedded Postgres major version (17 → 18 would need dump → restore; the suite's SQL backup already
  does that, so we wire it when it's needed).
- Using the embedded Postgres for the suite's own Demo Room (M8d may reuse this package).

## 4. Checks

- **Go:**
  - `internal/filedb` tests (behind `-tags filemode`, skipped in short mode): start, then restart with data reused;
    a leftover Postgres is stopped first; the secret is generated once; Sinhala trigram search returns a hit.
  - Config: file mode without `DATABASE_URL` / `JWT_SECRET`.
  - Bootstrap: `-mode file` passes the tag, and the tree is clean afterwards.
- **Manual (user):**
  - a file-mode build on **Windows**: double-click → till; close the window → no `postgres.exe` left; start again →
    same data. Windows is where the locale / encoding and the kill behaviour most need a real check.
  - one move from file mode to server mode by backup / restore.
- `make verify` and `verify-portable` (the binary stays static; Postgres is a separate unpacked program).

## 5. Open questions (my recommendation first)

1. **Port 5434** fixed with `FILE_DB_PORT` to change it (rec.), or pick a free port each start? A fixed port lets
   support connect with psql when needed.
2. **Auto-backup on by default in file mode** (rec.; the backups sit on the same disk, so the Data page also nudges
   "copy backups to a USB / cloud folder"). Or keep it off like server mode?
3. **Data folder next to the binary** (rec.; easy to find and copy), or the OS app-data folder (`%ProgramData%`,
   `~/.local/share`)?
