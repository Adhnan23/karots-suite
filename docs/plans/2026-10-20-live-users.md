# Live users Implementation Plan

**Spec:** `docs/specs/2026-10-20-live-users-design.md`

**Goal:** Admins see live who is signed in, locked, idle or away, where in the app, and their till / cart.

**Constraints:** core only (plugin-inert); in-memory store; ≤ 500-line files; 360px + Studio/Night × light/dark;
no Claude attribution; one commit per task.

- [x] **1. Store** — `internal/features/presence`: `Beat`, `SignOut`, `List(now)` with state derivation (active /
  idle / locked / away / signed out, 12 h drop), UA → "Windows · Chrome". Unit test.
- [x] **2. API** — `POST /presence` (jwt), `GET /admin/presence` (admin only, + till opened / sales today), Logout
  marks the device signed out; system user skipped. DTOs, `make types`, handler test.
- [x] **3. Heartbeat** — `X-Ks-Device` on every call (`client.ts`), `usePresence()` in `_authed` and `/lock`.
- [x] **4. UI** — Dashboard "Who's on now" card, Users page dot + where. Screenshots 360 / desktop, both palettes.
- [x] **5. e2e** — a cashier on the till shows up live on the admin's dashboard; locks → Locked.
- [x] **6. Docs** — status.md handoff.
