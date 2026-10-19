# M8c Self-update + release — design

The user, 2026-10-19: begin M8c; do it like karots-pos did, **but more modern and differently**. Bucket: a new R2
bucket **`karots-suite`**, read publicly at **`https://pub-e1fb05003fb841199f70633558a66fd2.r2.dev`** — suite shops
only, never mixed with karots-pos.

## Why (what went wrong in karots-pos)

A shop's first self-updating binary has to be carried in by hand — an old binary can't update itself. In karots-pos
the user built with `cmd/release -no-upload`, carried the binary to the shop and swapped it. That build was **never
uploaded**, so the shop's channel folder in the bucket stayed empty and the Data page showed **"update server answered
404"** instead of "up to date". Checked 2026-10-19: of karots-pos's three shops only `mum-digital` has a manifest;
`mum-motors` and `al-raj` answer 404. Two flaws, both fixed here by design:

1. A **build-only path that looks like a release**. In the suite **every shop build is a release**: the binary carried
   to the shop is the same file published to its folder, same version.
2. **"Nothing published yet" shown as an error.** A 404 manifest is a calm "No update published for this shop yet".

## Acceptance test (the user's, 2026-10-19)

1. Build M8c in the Builder for a test shop → carry the three files, install by hand (Setup). The same binary is in
   the bucket. The test PC shows **Up to date**, never a 404.
2. Build M8c2 and **Release** it to the bucket only — no visit.
3. The test PC notices M8c2 by itself, shows what's new, and updates when **Update** is pressed; it comes back on the
   new version (sales, settings intact).

## Goals

- Builder: **Test build** (local only, marked as such) and **Release** (build + publish); **Release** per shop and **Release all**; each row
  shows the **published** version and the last test build (no running version — see below).
- Shop: checks its own folder, shows current / latest / what's new on the **Data** page (admin + system), installs
  on **Update** (never by itself), comes back by itself, rolls back if the new binary can't stay up.
- Tills and admin browsers pick up the new SPA after an update without a stale page — but never mid-sale.

## Non-goals

- Backup copies to USB / network / S3 (**M8c2**).
- Auto-install at night / on a schedule (**user's call via my default, see Decisions** — can be added later as a
  setting; the button is the only path now).
- Delta / patch updates, multiple release channels (beta / stable), a release server of our own.
- Updating karots-pos shops, or moving a karots-pos shop onto the suite (that's `-import-pos` + a hand install).

## Anti-goals (fails even if it "works")

- An update that **stops a sale**: a till must never lose its cart or reload mid-sale because of an update.
- A shop binary that installs anything not signed by the suite release key, or a karots-pos binary over the suite.
- The channel (the shop's secret folder name) showing up in any page, log or error text.
- A Builder row that looks fine when the shop has nothing published.

## What stays from karots-pos (battle-tested)

Fork `internal/update` into `internal/update` (suite): per-shop secret **channel** folder (already minted in
`shops.json`, M8b); `manifest.json` signed **ed25519** over version + SHA-256 of the unpacked binary; gzip download;
**IPv4 first** (Sri Lankan ISPs stall big IPv6 transfers); stall watchdog (2 min); swap keeps `<exe>.old`;
**BootGuard** — after an update, two failed boots put `.old` back; `MarkHealthy` after it stays up; changelog of recent
releases so a shop that skipped some sees every change (`NotesSince`); keep the last 3 binaries, prune older (manifest
`history`, the bucket can't be listed). Version = `YYYY.MM.DD.HHMM` (fixed width, string order = version order);
`dev` is older than any release.

## What's different (the "more modern" part)

1. **Two buttons, and a local build can't pass for a release** (user, 2026-10-19: during testing and dev, small fixes
   must not each go to the bucket). **Test build** = build only, into `dist/<key>/`, nothing uploaded; the binary is
   stamped `main.buildKind=test` (updater still on, so the update flow itself can be tested) and the Data page shows
   **Test build** beside the version. **Release** = build + publish. The Builder row shows the last test build in amber
   ("test build, not released") apart from the published one; Test build on a shop with **nothing published yet** warns
   "This shop has no release yet — the first install at a shop should be a Release". If publishing fails (offline, R2
   error) the release build is kept and the row shows **Not published** in red with **Publish** (uploads the
   already-built files; no rebuild, same version).
2. **Release lives in the Builder page**, not a separate CLI: per-shop **Release…** (dialog: notes box pre-filled with
   the commits since that shop's last release, editable) and **Release all** (sequential; a failing shop doesn't stop
   the rest; failures listed). `-shop <key|all>` from the command line also publishes (technician path). karots-pos's
   `cmd/release` commands `add` / `list` / `keygen` are already the Builder's job, except **keygen**, which the Builder
   does on first use (writes `SUITE_RELEASE_KEY` to `.env` if missing, says so).
3. **Restart through the supervisor, not self-relaunch.** After a swap the server exits with a dedicated **restart
   exit code** (e.g. 75). systemd (`Restart=always`, 3 s) and the Windows launcher start the new binary. The launcher
   (`karots-start.exe -hidden`, `cmd/setup/hidden.go`) gains a loop: exit code 75 → start again at once (other exit
   codes behave as today, so the scheduled task's restart-on-failure still applies). No orphaned children, logs keep
   going to `karots.log`. A binary not run by a supervisor (dev) never has the updater on, so this never strands it.
4. **The SPA follows the server's version.** Every API response carries `X-Karots-Version`. When the SPA sees it change
   it reloads itself **at the next safe moment**: the admin side at once (after a toast-free banner "Karots was
   updated — reloading"), the till only when the cart is empty and no dialog is open. While the server is restarting,
   the till's existing offline/retry handling covers the gap (builder: confirm it does; if a request fails during the
   ~5 s restart, Pay must retry, never double-book — the sale request's client id from M8's list is the guard if it
   exists by then, otherwise Pay shows the normal "couldn't reach Karots, try again").
5. **Update is a short pause, announced.** Pressing Update shows: "Tills will pause for about a minute while Karots
   restarts. Sales already rung up are safe." Then progress (Downloading… 45% · Backing up · Restarting…). The backup is
   the core logical backup (`internal/backup`) into the shop's normal backups folder, named `before-update-<version>`.
6. **Separate key and message.** Suite release key (`SUITE_RELEASE_KEY` in the dev `.env`, never in `shops.json`),
   signed message prefix `karots-suite update\n` — a karots-pos signature can never validate here.

## The shop side

- Baked at build (`-X`, via the Builder's existing `ExtraX`): `main.version`, `main.updateBaseURL`,
  `main.updateChannel`, `main.updatePubKey`. A binary without a channel (dev, demo, `make build`) has no updater and the
  Data page shows the version only.
- Checks: at start, then every 6 h, and when the Data page opens (button **Check now**). Offline is normal — shown
  quietly ("Last checked 3 h ago — couldn't reach the update server"), never as a red error.
- States on the Data page **Updates** card: **Up to date** (version, last checked) · **Update available** (version,
  what's new, **Update**) · **No update published for this shop yet** (404 manifest) · **Couldn't check** (offline /
  bad signature — a bad signature *is* shown as a warning, it means tampering or a wrong key) · **Installing…**
  (progress) · **Rolled back** (after BootGuard put the old binary back: "The update to X couldn't start; Karots went
  back to Y" — shown until dismissed).
- Who: admin and the system user. Manager and cashier never see the card.
- API: `GET /api/v1/admin/update` (status), `POST /api/v1/admin/update/check`, `POST /api/v1/admin/update/apply`
  (DTOs in `dto.go`, `make types`).
- File mode: the swap is of the binary only; `data/` is untouched. Server mode with cashier PCs: only the main PC runs
  the binary; cashier PCs are browsers and follow via `X-Karots-Version`.
- Linux: the service runs as the shop user (`User=` in the unit), so the **install folder must be writable by that
  user** for the swap. Setup must ensure this (builder: check what Setup does today; fix in Setup if not).

## The Builder side

- Env (dev `.env`): `SUITE_RELEASE_KEY`, `R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`,
  `R2_BUCKET=karots-suite`, `UPDATE_BASE_URL=https://pub-e1fb05003fb841199f70633558a66fd2.r2.dev`. Missing R2 creds →
  Build still builds, row says **Not published — R2 not set up** (the reason, not a stack trace). Fork karots-pos
  `cmd/release/r2.go` (S3 API PUT/DELETE, its tests) rather than adding the AWS SDK.
- Upload order: binary `.gz` first, `manifest.json` last with `Cache-Control: no-store`, so a shop never sees a
  manifest whose file isn't there.
- Refuse to release from a dirty tree (as karots-pos), so "what's new" matches what ships; the Build button says why.
- `shops.json` per shop gains `published` (`{version, commit, at}`), next to `lastBuilt` (which also records
  `kind: "test" | "release"` and `version`). The row shows **published** and, in amber, a newer test build; red when
  nothing is published.
- What's new = the release dialog's text + `git log --oneline` since the shop's previous published commit, filtered to
  core + that shop's plugins (fork `changesSince` / `notes.go`).

## Running version — not tracked (user, 2026-10-19)

The user first chose a Cloudflare Worker in front of the bucket to record each shop's running version, then dropped it:
**no extra moving parts**. Shops read the bucket directly at `UPDATE_BASE_URL` (the r2.dev URL). The Builder shows only
what it **published** (version, when) and the last **test build**; what a shop runs is seen on its Data page (in
person or via Tailscale). The row is red only when the shop has **nothing published**. A running column can come
later only by carrying a new binary to every shop (the feed URL is baked in) — accepted.

## Decisions I made (not the user's — check them)

- The shop **never installs by itself**; someone presses Update (follows "never stop a sale").
- Restart via a supervisor exit code instead of self-relaunch (different from karots-pos, fewer moving parts).
- Release is in the Builder page; no separate `cmd/release`.
- Check every 6 h + on opening the Data page.
- **Release** builds from HEAD (binary, `karots-setup`, `install.conf`) and publishes it, after a notes dialog
  pre-filled with the commits since the shop's last release (empty is fine). Release all = the same for every shop, one
  shared notes box. **Test build** skips the notes and the clean-tree check (user's dev / testing path).
- **Dirty-tree check ignores the Builder's own writes**: `shops.json` and the generated plugin lists
  (`cmd/server/enabled_plugins.go`, `web/src/plugins/enabled-plugins.ts`). `published.commit` = HEAD at build.
  Commit `shops.json` yourself as today.
- **One binary per channel** (profile OS / arch / mode), as karots-pos. A shop with a second PC of another OS gets a
  second profile.
- **Docker / VPS shops (`deploy/docker`) have no updater**: the image is rebuilt instead (the swap would be lost on
  container recreate, and `/app` isn't writable). The Docker build passes no channel, so the Data page shows the version
  only with "Updated by rebuilding the image".
- Migrations stay additive (as they are), so a BootGuard rollback onto the old binary still runs against the newer
  schema. A release whose migration isn't additive must say so in its notes; no automatic database rollback.

## Left to the builder

File layout inside `internal/update` and `cmd/bootstrap` (≤ 500 lines each), the restart exit code's value, exact
Builder row styling (match the M8b Builder page), test seams (fake R2 server in tests, as karots-pos `r2_test.go`).
Tests: manifest sign/verify, `Newer`, NotesSince, BootGuard, 404 → "not published" state, launcher loop on code 75,
Builder publish with a fake R2 (order: gz before manifest), SPA reload only with an empty cart (Vitest).
