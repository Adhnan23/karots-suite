# Live users — who's on now (design, 2026-10-20)

## Why

The user (2026-10-20): an admin only sees the staff *list* on Users. They want to see, live, whether each person is
signed in, locked, and what they're doing — on the dashboard or somewhere. Asked for before games-as-plugins.

## What the admin sees

- **Dashboard → "Who's on now"** card (admins only), refreshed every 5 s. One row per device (PC / phone / tablet) a
  staff member is on: state dot + name + role · where (Till · Sell, Stock, Lock screen…) · a note · device and IP ·
  till open since HH:MM · N sales today. Online rows first, then away / signed-out rows (greyed, "last seen 10:42").
  Nobody on → "Nobody is signed in right now."
- **Users page**: each user card gets the same state dot and a one-line "where" (their freshest device).
- **States:** *Active* (input in the last 2 min) · *Idle N min* · *Locked* (lock screen; "with 3 items in the cart"
  when a cart waits) · *Away* (no signal for 60 s: browser closed, PC off, network down — "last seen HH:MM") ·
  *Signed out* (pressed Sign out). Away / signed-out rows drop after 12 h.
- **Till note:** a non-empty cart shows "Serving · 3 items · Rs 1,250".
- The system user never appears. Managers don't see it (admin only, like Users).

## How

- **Heartbeat:** every signed-in screen (inside `_authed`, and the lock page) posts `/api/v1/presence` on mount, on
  every route change, on tab focus and every 20 s: `{ path, idleSec, cartItems, cartTotal }`. jwt-only (works while
  locked); `locked` comes from the token, not the client. The cart summary is read from the saved cart
  (`loadCart(userId)`), so the till isn't touched.
- **Device:** a random id in `localStorage` (`ks-device`), sent as `X-Ks-Device` on every API call, so Sign out can
  mark that device signed out at once. IP = the socket IP; device label = OS · browser from the User-Agent.
- **Store:** in memory in the server (`internal/features/presence`), keyed by device id. A restart forgets it; live
  screens refill it within 20 s. No table, nothing backed up.
- **Admin read:** `GET /api/v1/admin/presence` (admin only) — the store's rows plus, from the DB, each user's open
  till (`opened_at`) and completed sales today (`shopDay`).
- **Where** is the path mapped to a label on the viewer's side: admin pages by `whereIs(adminNav)`, till tabs by
  `tillTabList`, `/admin` = Home, `/lock` = Lock screen, plugin pages by `slots.apps` / `slots.tillTabs`; else the path.

## Non-goals

- No history ("who was on at 3 pm") — Activity covers actions.
- No keystroke / screen capture; only the area of the app and the cart summary.
- Remote action: **Lock** only (added 2026-10-20) — refused while serving (cart changed < 5 min); a cart unchanged
  10+ min is flagged. No remote sign-out.
- No push (SSE / websockets): 5 s polling on one admin screen is cheap on a shop LAN.
