# Idea: the big Shop Builder (M8)

Captured 2026-10-12 from the user. It pulls together the M8 ideas already in `docs/status.md` (Shop Builder, self-update,
file mode, practice mode). **Not planned yet**: it gets a rethink and a spec at M8. **The phone app is NOT part of
M8** (user, 2026-10-12) — it stays the later [Pocket Cashier](pocket-cashier.md) project; the last section only
keeps the note of how it could plug in.

## Start from what karots-pos already has (CLI)

karots-pos has the release + update half working as a CLI (fork it, as the domain code was):

- **`cmd/release`** — `keygen` (signing key into `.env`), `add` (register a shop: OS / CPU / plugins), `list`, then
  `release [-notes "…"] [shop …]` builds every (or the named) shop's binary from **`shops.json`** and publishes it
  to the update host (Cloudflare R2, S3 API); `-no-upload` builds only, for a first install. Each shop has a
  **secret channel** (its bucket folder), so `shops.json` stays in a private repo and the bucket is never listable.
  Needs `POS_RELEASE_KEY`, `POS_SUPPORT_SECRET`, `UPDATE_BASE_URL`, `R2_*` in `.env`.
- **`internal/update`** — the shop side: check the feed, download, install only a signed release whose SHA-256
  matches, back up first, swap keeping `.old`, relaunch, roll back after two failed boots (Windows relaunch too).

So M8 = port those two (the suite's own key, feed and bucket path), then make the browser Shop Builder a **UI over
the same `shops.json`** (add / edit a shop, build, publish, see who's behind) — the CLI stays for scripts. New on top:
each shop's **mode** (file / server).

## One place that knows every shop

Today's suite Shop Builder (`cmd/bootstrap`) builds one binary at a time. The idea: it works from the **list of
shops** (`shops.json`, as karots-pos `cmd/release` keeps it). For each shop it holds:

- the shop's name, **install id** (support PIN), OS / processor;
- its **plugins**;
- its **mode**: **file mode** (one PC: binary + `data/` folder, its own embedded Postgres, no database install) or
  **server mode** (a real PostgreSQL, LAN or VPS); moving up = backup → install Postgres → set `DATABASE_URL` →
  restore;
- its **update channel** and which version it runs.

From that list it can **build** a shop's binary, **publish** an update only that shop receives (signed, SHA-256
checked, backup first, rollback after two failed boots, as karots-pos `internal/update` does), and show which
shops are behind.

## Later, not M8: the phone takes the shop's plugins automatically

The phone app shouldn't be a per-shop build. Proposal: **the shop's own binary serves the phone app** (a
PWA, an installable web app) from the same SPA build it already embeds. So:

- the phone gets **exactly that shop's plugins**, the ones compiled into its binary, with nothing to choose or build;
- when the shop updates, the phone app updates with it (same version, so the replay always matches the backend);
- no app store; the cashier opens the shop's address once and adds it to the home screen.

Offline: the PWA caches the app and keeps the shop's data in the phone's local storage (IndexedDB), as Pocket Cashier
describes. A PWA needs **HTTPS** to work offline: Tailscale gives each device an HTTPS name and certificate; on a
plain LAN the binary would need its own certificate. That's the main thing to settle.

If a real native app is ever needed (Bluetooth printing, background sync), it can still read the plugin list from
the shop (`/till/config` can say which plugins are on) and show only those.

## Notes

- "PGlite" was the word used for file mode. PGlite is Postgres in WebAssembly for JavaScript; in a Go binary the
  same idea is an **embedded Postgres** (as Demo Room's `internal/demodb`). Both keep the Postgres-only SQL working,
  unlike SQLite.
- The suite needs its **own** release feed, key and bucket path: never a karots-pos binary over the suite, or the
  reverse.
