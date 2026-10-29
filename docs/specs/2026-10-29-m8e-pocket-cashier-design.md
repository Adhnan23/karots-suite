# M8e Pocket Cashier — design

Brainstormed with the user and **approved on 2026-10-29**. It replaces the idea note `docs/ideas/pocket-cashier.md`,
which is removed (git history keeps it). Look approved: phone mockups (sign in, sell, pay, PC down) in Studio / Night /
Bazaar, light and dark.

## Why

The user's product rule is **never stop a sale** (sell, then flag for review; never block the counter). In Sri Lanka a
shop keeps selling through power cuts, a dead PC or an internet cut. If the system ever becomes the reason a sale
can't happen, the shop drops the system, not the sale. A second need came up while brainstorming: in a **wholesale
shop the stock sits out the back or in a store room**. The cashier should be able to sell there, on a phone, while
the PC works normally.

So Pocket Cashier is **a phone cashier with two jobs**:

1. **Every day, PC on:** sell anywhere in the shop (back store, second counter) live against the server.
2. **PC (or internet) down:** keep selling from a copy kept on the phone. When the server is back, send the sales so
   it rings them up in order.

## Goals

- A phone cashier page, **rebuilt for a phone** (not the desktop till squeezed down), that follows the shop's palette
  (built-in or custom) and the phone's light/dark mode, like every other screen.
- Works on **Android and iPhone** as an installed web app (PWA). The user rejected a native app (Capacitor / Tauri)
  because shops may have iPhones.
- Reaches the shop server over the **shop's own wifi with no internet** (LAN IP). It works the same way over
  Tailscale or a VPS.
- **Offline selling** from a copy, and the sales are **replayed** on the server through the normal sale code when it is
  back. The databases are never merged.
- **Never refuse an offline sale after the fact:** differences are applied and flagged for an admin to review.

## Non-goals

- **No separate admin app.** The admin panel already works at 360px, and owners watch remotely over Tailscale. On a
  phone, the admin panel is the normal suite in the browser (or the same installed app, see Sign-in).
- **No printing from the phone.** Online, receipts print on the shop's printer through the server. Offline, there is
  no paper (user: "sales come first"). Receipts can be printed later from the PC.
- **No database merge or two-way sync.** The server is the only truth.
- **No native app, no app stores.**
- **No background syncing promises.** A web app can't sync reliably while closed (iPhone never, Android only rarely),
  so the copy is as fresh as the last time the app was open.

## Anti-goals (failure even if it "works")

- A sale that the phone took and the server later **refuses or loses**.
- **Double-booked** sales after an interrupted or repeated send.
- One person selling on **two devices at once** and the drawer count not matching.
- Core code that **knows about this plugin**.
- A setup that a technician can't finish reliably in a few minutes per phone.

## Product shape

- **It's sold as an app, built as a plugin.** Internally it's a normal compile-time plugin (key `pocket`), ticked per
  shop in Shop Builder. Shops are told about it as **"Pocket Cashier app"**, an add-on next to the suite, not as one of
  the plugins.
- **Price: 5,000 LKR** (user). It's included in **Everything**, which **stays 95k**. *(My recommendation; the user said
  to go with the recommended choices.)*
- **Docs and legal (my call, following AGENTS.md):**
  - an app section in the README and guides (not a row in the plugin table)
  - its own tick box in the First Schedule, set apart from plugins
  - its own Shop Rules §15 price line
  - `internal/docscheck` taught that this key is presented as an app, but still required in all those places
  - then `make agreement`

## One app, not two

- **The installed app is Karots Suite itself:** the same SPA, served by the shop server on the plugin's HTTPS phone
  address, with a manifest and service worker added there.
- **Who signs in decides what it shows:** a cashier or manager gets the phone cashier screens; an admin (online only)
  gets the normal admin panel.
- **The PC is unchanged:** the browser or kiosk at the normal address. The phone address is a different origin, so
  its service worker and offline copy never touch the PC.
- **Shops without the app** have no phone address and nothing to install. Owners use the admin panel in the phone's
  browser, as today.

## How the phone reaches the server (HTTPS)

A PWA must be served over trusted HTTPS to install and work offline. A shop on plain `http://192.168.x.x` can't have
that, and a public domain needs internet DNS, which the user rejected. **Decided:**

- **The shop server makes its own root certificate (CA)** once, keeps it in the data folder and includes it in
  backups and restores. From it, the server issues its own HTTPS certificate covering its current LAN IPs (and
  `localhost`). It **re-issues automatically** when its IPs change, so phones trust the CA, not an address.
- The server listens on an **extra HTTPS port** for phones (as well as its normal port). Advise a router DHCP
  reservation so the PC keeps its address.
- **VPS or Tailscale shops:** a normal public certificate (the user owns a domain; Tailscale gives HTTPS names). There
  is no CA to install on the phone.
- **Phone setup page** (plain HTTP on the normal port, shown with a QR code on the PC). Lessons from the spike:
  - It offers **`.crt` for Android, `.mobileconfig` for iPhone** and `.pem` for others, with **exact per-phone steps**.
    On Android: Settings → *CA certificate*. Installing through Wi-Fi settings makes it a Wi-Fi certificate, which
    Chrome ignores. On iPhone: install the profile **and** turn on full trust under Certificate Trust Settings.
  - It **tests whether the phone trusts the certificate before showing the app link.** A clicked-through Chrome
    warning "sticks" for that address and blocks the service worker, so the phone must never reach the warning page.

**Spike status** (throwaway, `~/Projects/go/karots-pwaspike`):
- **Android (OnePlus 13) passed:** trusted CA, installed PWA, closed and reopened offline, sold from the copy, sent 4
  sales back in order.
- **iPhone 13 test is pending (2026-10-30).** If the iPhone can't run it offline with a home-made certificate, revisit
  this section before planning.

## Phone pairing and security

- **Admin-only approval** (user). The admin opens a **Phones** list (admin page, plugin-owned) and scans or approves a
  QR code shown by the new phone.
  - The list shows each phone's name, who's on it, its copy age and last contact.
  - It has a **Block** button: the phone's key stops working at once.
- **The phone holds its own device key**, separate from any person. With it, the phone **syncs even when nobody is
  signed in**, as long as the app is open (the sign-in screen counts).
- **The copy on the phone is encrypted at rest** with a key bound to the device pairing (exact scheme left to the
  builder; WebCrypto, non-extractable where possible).

## Sign-in and one person, one device

- **The phone's sign-in screen** works like the till lock screen: tap your name, PIN.
- **Only cashier and manager PIN hashes go into the copy**, so those people can sign in and **switch offline**.
  - A 4–6 digit PIN hash taken from a stolen phone can be cracked in minutes, whatever the encryption, and it would
    then work on the shop PC. **Admin and system users are therefore never in the copy.** They sign in on the phone
    only while the server is reachable.
  - Offline the phone only sells anyway, so an owner who must sell during a cut uses a cashier account.
  - This replaces an earlier idea of carrying the system PIN offline.
- **Admin PIN (online) → the normal admin panel.** Cashier or manager PIN → the phone cashier.
- **One person, one device (user's rule):** signing in on the phone **locks that person's PC session**, and the
  reverse. It applies to admins too.
  - The locked device says where the person went ("Nimal is now on the phone").
  - Signing in on the locked device takes the person back to it.
  - Other people can still use that PC with their own PIN.
  - *My call:* if the phone goes offline while someone is signed in, the server keeps counting them as on the phone,
    so they can't sell on the PC and on an offline phone at once. An admin can release it from the Phones list
    (for a lost or dead phone).
  - *My call:* if the person's PC has a sale on screen when they sign in on the phone, **that cart is held
    automatically** (it shows in Held on both devices), so nothing is lost.
  - Today the lock is a claim inside the JWT (`lck`) with no session state on the server, so this needs a
    **server-side record of the user's active device** that protected requests check. That's a core seam (below), and
    the desktop lock and auto-lock must keep working.

## The phone cashier page

From the approved mockups:
- **Top bar:** a Live / PC-not-reachable pill, a drawer pill, and the avatar.
- **Bottom tabs:** Sell / Held / Counter / Menu.
- **Sell:** search, **barcode scan** and category chips; tap to add. A cart bar shows the item count, customer and
  total, plus **Pay ›**. Tapping the bar shows the lines.
  - **Barcodes:** the phone camera (Android uses the browser's built-in reader; a bundled reader library covers the
    iPhone and works offline), or a Bluetooth scanner acting as a keyboard.
- **Pay:** its own page with **‹ Back to items** that keeps the cart.
  - The cart lines have ±.
  - Tenders: Cash / Card / Cheque / Credit, plus any tender a plugin adds.
  - Cash shows quick amounts and the change to give.
  - Online it shows "Receipt prints at <printer>". The sale goes through the normal print call, which already picks
    the cashier's own counter printer or the shop printer, and follows the shop's ask-to-print setting.
  - **A phone sale does not pop the drawer** (my call; nobody is at the counter).
- **Held:** hold and resume, the same as the till.
- **Menu:** open and close the session, withdraw, deposit, plus any plugin cashier menu items and actions **that exist
  on that server**. Plugins appear through the same slots the desktop till uses, so the phone page needs phone-sized
  renderings of those slots.
- **Offline state:**
  - The same pages in an amber state: "PC not reachable".
  - **Waiting N** shows the sales not yet sent.
  - "Selling from the copy of HH:MM". Stock is shown as approximate.
  - **Counter greyed out.**
  - The copy's age is always visible, with a warning when it's old.

## The money: one drawer per person

- **All of a person's sales go into their own till session**, phone or PC (user: otherwise the count mismatches). The
  drawer belongs to the person, and whichever device they're on feeds it.
- The copy includes the **money places** (core lockers and tills) with their balances, and **which cashier may use
  which**. So a cashier can **open a session on the phone**: online straight away, offline saved and sent back
  **before** its sales.
- **A session can be closed offline too** (user asked). The cashier counts the cash on the phone, and the close is
  saved as an action sent **after** that session's sales. On replay the server works out the expected cash with
  every sale in, compares it to the count and shows short or over as usual. The close report (Z) prints when the PC
  is back. The existing "till left open" failsafe (prompt to close and reopen, admin force close) stays as the safety
  net for anyone who doesn't close.
- **Which day and session an offline sale counts in** (my recommendation, user agreed):
  - **Its session is still open** (the usual case): the sale counts **when it was sold**, in that session and on that
    day.
  - **Its session was already closed** (the Z report may be printed): closed figures never change. The server opens a
    **new session** for that person at arrival time, puts the sales in it and **flags it for review**. The sales count
    on the day they arrived, and **each one shows its real sale time**.
  - Core today ties sales to sessions only by `cashier_id` and time (`created_at >= opened_at`). The builder decides
    whether that holds with these rules, or whether sales get an explicit session link. The rule above is the
    behaviour to keep.

## The copy (what the phone keeps)

- Products with prices, packs, barcodes and stock (and lots where core uses them)
- Categories
- Customers with credit balances and limits
- Cashier and manager users with PIN hashes and permissions (never admin or system)
- Money places and who may use them
- Open sessions, held sales and settings
- Each compiled-in plugin's till data, *if* that plugin says it works offline

Refresh:
- **While the app is open**, it refreshes on a timer (about a minute) **and listens to the server live**, so changes
  arrive within seconds. This shrinks the "sold on the PC just before the cut" gap to about the last second.
- Closed: no refresh (see Non-goals). The admin's **Who's on now / Phones** shows each phone's copy age, so a stale
  phone stands out.

## Sending sales back (replay)

- Everything the phone did offline is a list of **actions**: session open, sale, hold, withdraw, deposit, session
  close (with the counted cash). They're sent **oldest first** and applied through the **same core services** as a live action. Stock, lot
  (FEFO) cost, credit, the drawer and P&L stay right because there is no second copy of the domain logic.
- **Duplicates, two layers:**
  1. **Exact:** every action carries a one-time ID. A repeated send is skipped silently.
  2. **Look-alike:** a replayed sale that closely matches one the server already has (same items and total, a few
     minutes apart, e.g. rung on both during the cut) is **applied and flagged**. The admin keeps both, or removes
     one. **Removing one is a full admin return** (the existing admin return already picks where the refund money
     comes from), preset to take the refund from **the drawer that took the sale**. There is no new "cancel sale"
     feature. Nothing is deleted automatically.
- **Flagged, never refused:**
  - **Stock that ran out** in the meantime: core's existing found-at-till path, flagged.
  - **A price that changed** meanwhile: the sale **keeps the price the phone showed**, and the difference is flagged.
  - **A credit limit passed** because the phone's balance was old: flagged.
- **The time it was rung** is kept on the sale; the time it arrived is recorded too.
- **Review screen (admin):** what came in from each phone, per send, with the flagged items and their actions.
- **Worked example:** the phone's copy is from 8:00. The PC sells at 8:00:30 and dies at 8:00:40. That sale stays on
  the server, and the phone's later sales land on top of it. Only stock and credit can be off by what that one sale
  changed, and both are flagged.

## Core seams this needs (core stays plugin-inert)

Generic hooks or options in core that the plugin uses. The exact shapes are left to the builder; they must make sense
without Pocket Cashier:

- **Extra listener:** a plugin may serve the app on an additional HTTPS listener with a certificate it supplies.
- **Device takeover:** a server-side record of each user's active device ("this user is now active on device X"),
  checked by protected requests, which locks the user's other sessions. Today's JWT `lck` lock and auto-lock keep
  working.
- **Sale creation with a one-time ID, a "rung at" time, and "sold at this price, flag if different"**, plus a way to
  flag a sale for review. All are useful to any client.
- **Session open with a recorded opening time and float**, and a way to say "these sales belong to that session".
- **A live change feed** (or reuse of an existing one) that a client can listen to for catalog, stock, customer and
  session changes.
- **Plugin "works offline" declaration:** each plugin says whether its till parts work offline. Ones that need the
  server live (recharge float, AI catalog, …) are greyed out offline.

## Install and setup

- *My call:* **karots-setup opens the firewall port** for the phone HTTPS listener on Windows and Linux when the shop
  has the app, and Shop Builder carries any setting it needs (a `shopconf` field). It's part of release 1.
- The server issues and re-issues its own certificate. No technician step except the per-phone setup.

## Release order (user agreed)

1. **First:**
   - online and offline selling, Pay, Held
   - session open and close, withdraw and deposit
   - online printing at the counter
   - phone pairing and blocking, one person one device
   - certificates and the setup page
   - replay with flags and the review screen
2. **Second:** the counter pages (receipts, credit, suppliers, expenses, returns, …) and plugin menus, **online
   only**.
3. **Later:** whichever counter pages the user wants offline too.

## Left to the builder

- Plugin key and package names; route paths; the HTTPS port number; table design; how the copy is stored on the
  phone (IndexedDB layout) and encrypted.
- The exact look-alike rule (time window, matching).
- The live feed technology (SSE / WebSocket / long-poll).
- How the plugin's slots render at phone size.
- The barcode reader library (it must work offline and on iPhone).
- How the server re-issues its certificate when its IP changes.

## My calls (check while reading)

- Everything stays 95k and includes the app.
- Only cashier and manager PINs go on phones; admin and system users sign in on a phone only online.
- A phone sale doesn't pop the counter drawer.
- An offline-but-signed-in phone keeps the person locked off the PC until it sends back or an admin releases it.
- A PC cart on screen is held automatically when its person signs in on the phone.
- A late sale for a closed session goes into a new flagged session (arrival day, real sale time shown); otherwise it
  counts when it was sold.
- A look-alike duplicate is removed with the existing admin return, refund from the drawer that took it.
- karots-setup opens the phone port in the firewall.
- Docs and legal present it as an app, with docscheck taught about it.
