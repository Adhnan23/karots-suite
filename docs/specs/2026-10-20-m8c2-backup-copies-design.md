# M8c2 — Backup copies (design)

Captured 2026-10-20 from a conversation with the user. Builds on `docs/ideas/backup-copies.md` (which this narrows:
no owner S3 places), M8c (R2 client, release bucket) and the Setup Uninstall / Move work (Bring back a shop).

## Why

A shop's automatic backups land in one folder on the same PC as the data. In file mode (one PC) the database and its
backups share a disk: a dead disk, a stolen PC, a fire or a wiped Windows takes everything at once. In server mode the
mirror case holds: if the database lives elsewhere (VPS, Neon) and that is gone, the shop wants a copy on its own
side. You never know what will happen to the PC — so **every backup should also exist somewhere that won't die with
the thing it backs up.**

Pointing the backup folder itself at a USB stick is not enough: a pulled or forgotten stick means **no backups at
all**, silently. A copy keeps the PC's own backups and lets the stick catch up when it is back.

## Two copies, nothing else

1. **Copy folder (the owner's)** — one optional folder: a USB stick, `D:\`, `\\server\share`, a mounted path. The
   owner (an **admin**) sets, changes, switches off or clears it on the **Data page** — their freedom, their
   responsibility. Plain backup files (not encrypted): the owner can bring a shop back with nothing but the file.
2. **Karots cloud backup (the vendor's, paid)** — the vendor's own R2 bucket (a second bucket beside the releases
   one), one folder per shop. Only the **vendor** turns it on: ticked in the Shop Builder, or on the Data page **as
   the system user**. The owner sees it and **can switch it off** (their choice) but cannot switch it on or point it
   elsewhere. Copies are **encrypted** with a per-shop key. Billing is outside the software.

No owner S3 / cloud places (user, 2026-10-20: "only our bucket").

## Goals

- After every automatic backup, the copy folder gets the new file and anything it missed (catch-up); the cloud gets
  about one a day (below).
- **Never stop a sale.** A missing USB, an unreachable share or no internet = skip, retry later, warn. Copying never
  blocks or slows the till and never fails a backup.
- The copy folder gets every backup and keeps the newest **N** (= `BACKUP_KEEP`). **The cloud gets only the newest
  backup of each day** and keeps **30 days** (user, 2026-10-20: storage is the vendor's cost; ~30 files per shop).
- Plain status per copy: "USB — last copied 10 min ago", "Karots cloud — failed: no internet (3 waiting)". The admin
  home warns when an enabled copy has not copied for 24 h.
- **Restore from a copy** on the Data page: list its backups, bring one back (same flow and safety copy as a restore
  from the backups folder).
- **Bring back a shop** in Setup also lists the copy folder's backups (when it exists on this PC) and the shop's
  Karots cloud backups (when the build carries it). Neither known → exactly today's behaviour.

## Non-goals

- No owner S3 buckets, SFTP, Google Drive, Dropbox (a share or a synced folder can be the copy folder).
- No more than one copy folder.
- No encryption of the copy folder.
- No backup-health reporting back to the vendor (same call as M8c).
- No billing, licensing or expiry of the cloud service in code.

## Anti-goals

- A copy problem that shows up at the counter (a blocked sale, a slow till, a modal).
- A backup file that carries the cloud credentials or key.
- One shop able to read or wipe another shop's cloud copies.

## Design

### Where the settings live

- New table **`backup_copies`** (migration `00006_backup_copies.sql`): one row per copy — kind (`folder` | `cloud`),
  enabled, folder path (folder only), status columns (last copied at, last copied file, last error, last tried at).
- The table is **never dumped and never touched by a restore** (added to `skipTable`): it describes *this PC* (a
  drive letter, this build's cloud switch), not the shop's data.
- **Preset folder** from `.env`: `BACKUP_COPY_DIR` (written by the Builder or the Setup page). On start, when no folder
  row exists *and* this preset has not been seeded before, a row is inserted with it; the seeded value is remembered
  so a folder the owner cleared does not come back on the next start. *(Builder's call: exact marker.)*
- **Karots cloud** from `.env`: `CLOUD_BACKUP_URL` (endpoint + bucket), `CLOUD_BACKUP_KEY_ID`,
  `CLOUD_BACKUP_SECRET`, `CLOUD_BACKUP_FOLDER` (the shop's key), `CLOUD_BACKUP_CRYPT` (32-byte encryption key, hex),
  `CLOUD_BACKUP=on|off` (the initial switch). With credentials present, the server keeps one `cloud` row, created
  enabled when `CLOUD_BACKUP=on`; after that the row's switch counts. Credentials gone from `.env` (an Update of a build
  without the tick) → the row is removed and the card no longer shows it.
- **Only paying shops carry the cloud keys** (user, 2026-10-20): the Builder writes them only when the shop's profile
  has Karots cloud ticked. Stopping the service = untick → Release → the shop's next Update drops the keys. *(My
  note: one shared write key cannot be revoked for a single shop; if a key leaks, make a new bucket token, put it in
  the Builder's `.env` and Release all. Per-shop tokens are out of scope.)*
- **A new preset wins:** the seeded marker stores the preset's value; when `.env` carries a different
  `BACKUP_COPY_DIR` (changed on the Setup page at an Update), the folder row takes the new path. Like `BACKUP_DIR`,
  `BACKUP_COPY_DIR` is the PC's own choice: an Update keeps it unless changed on the Setup page. *(My call.)*
- The cloud is never listed on a timer: the row's last-copied time decides the next upload (R2 free plan: 10 GB,
  1 M Class A ops / month — ~30 uploads per shop per month).

### Copying

- A **copy pass** runs right after each scheduled backup and every 30 minutes (list + compare is cheap). Per enabled
  copy: list what it holds, copy every `*.json.gz` in the local backups folder that it lacks (oldest first), then (folder
  only) prune to `BACKUP_KEEP`. Status columns updated; errors logged and stored on the row, never surfaced at the till.
- Only runs where the scheduler runs (main PC with `BACKUP_DIR` set).
- **Folder:** write `<path>/<name>.tmp`, then rename (a pulled stick never leaves a file that looks restorable). A path
  that does not exist (stick not plugged in) is "not there right now" — never created by the pass. Saving the folder on
  the Data page / Test may create it when its parent exists.
- **Cloud:** move `cmd/bootstrap/r2.go`'s stdlib SigV4 client to a shared package (e.g. `internal/s3`: put / get /
  list), the Builder keeps using it. Files are `<name>.json.gz.enc`, AES-256-GCM (stdlib) with the shop's key, in
  `<CLOUD_BACKUP_FOLDER>/`, named by date and time only (no shop name — a lifted key reveals as little as possible
  about other shops). The pass sends the newest local backup when the last cloud copy is
  more than 20 hours old (about one a day, always the freshest data; a PC that was off sends one as soon as it is back).
  No catch-up of older files for the cloud. *(My call.)* **Shops never delete** in the vendor bucket: retention is an R2 **lifecycle rule** (e.g.
  delete after 30 days) and a **bucket lock** rule refuses deletes / overwrites inside the window — so a key lifted
  from one shop's `.env` can add files but not destroy anyone's copies, and per-shop encryption stops it reading them.
  One-time bucket setup goes in the plan. *(My call — R2 tokens cannot be limited to one folder.)*
- Encryption in memory (a backup is tens of MB) — `ponytail:` comment naming the ceiling.

### Data page

- New card **Backup copies** (admin), built from the admin kit, works at 360px and in all four palette/mode
  combinations:
  - **Copy folder** row: path, status, switch, Copy now, Change folder (path field + Test: writes and removes a small
    file), Clear.
  - **Karots cloud** row (only when the build carries credentials): status, switch, Copy now. Admin can switch it
    **off**; only the system user can switch it back **on**. No edit. A build without credentials shows nothing (the
    vendor ticks it in the Shop Builder and the shop runs Update).
  - **Backups here…** on each row: its list, newest first; **Restore** downloads (and decrypts) the file into the local
    backups folder, then the existing restore-file flow runs (safety copy first, same dialog).
- Admin home: a warning item when an enabled copy has not copied for 24 h, via the dashboard's existing warning list.

### Shop Builder

- Profile `backups` gains `copyDir` (optional preset) and `cloud` (bool, "Karots cloud backup"). The per-shop cloud
  key is generated on first tick and kept in `shops.json` (masked like other secrets; no Rotate — it would orphan old
  copies).
- The Builder's own `.env` gains `CLOUD_BACKUP_*` (endpoint, bucket, write key) — separate from the release keys.
- Shop build writes `BACKUP_COPY_DIR` and, when `cloud` is ticked, the `CLOUD_BACKUP_*` keys with `CLOUD_BACKUP=on`.
  Both join `refusedEnv`.
- A vendor action to open a shop's cloud copy without the shop: list / download + decrypt to a plain `.json.gz`
  (Builder page button or a flag).

### Setup

- The Setup page's backup section gains one optional **"Also copy backups to"** folder → `BACKUP_COPY_DIR`.
- **Bring back a shop** also lists backups in the preset copy folder (if it exists on this PC) and in the shop's Karots
  cloud folder (if install.conf carries it), labelled by place, newest first. Picking a cloud one downloads and
  decrypts it into the Karots backups folder, then the existing restore step runs. No network → the cloud list is
  missing with one line saying why.

## Constraints

- Stdlib only for S3 and crypto.
- Core stays plugin-inert.
- Files ≤ 500 lines, by concern.
- DTOs in `dto_data.go`, then `make types`.

## Left to the builder

- Column names, endpoint paths, the preset's "seeded" marker.
- Request timeouts, how "waiting" is counted.
- UI layout within the admin kit (screenshot gate at 360px + desktop, Studio/Night, light/dark).
