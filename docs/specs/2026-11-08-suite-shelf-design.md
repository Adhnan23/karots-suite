# Karots Suite Shelf: design

**Status:** written from brainstorming with the user, 2026-11-08; approved by the user 2026-11-08 ("now continue").

## 1. What it is and why

**Karots Suite Shelf** (home screen: **Karots Shelf**) is an Android app for **a shop's own staff** to put the
shop's products into its Karots Suite from their phones: walk the shelves, scan a barcode, read the label with
OCR, use AI Identify, set prices and stock, one product at a time, several people at once.

Why (user): shop people saw Karots Suite Onboard (the user's own setup app) at work and asked for it. Several
phones walking the shelves fill a shop much faster than one person at one PC, and phone scanning / OCR / Identify
is easy for them.

It is **Onboard stripped down to one shop**: no catalogue, no shop profiles, no support secrets, no install-ID
code. People sign in with **the user accounts they already have in the shop's Suite**.

## 2. Goals

- Sign in with a shop user's **phone number and PIN** (the Suite's own login). Only the shop the app connected
  to; nothing else.
- **Pull** the shop's products, suppliers, categories, units, Product Plus fields and packs (what Onboard's pull
  reads for a shop), so the phone has its own copy.
- **Add and edit products** in the shop, with everything Onboard's shop editor has: barcode scan, extra barcodes,
  OCR on every field, AI Identify, category, unit (the shop's default first), supplier, cost / selling /
  wholesale, packs, Product Plus fields, old stock (none / one batch / several batches with expiry and prices),
  a product's batches (count, expiry, price), Make a copy, disable in shop.
- **Quick add** (scan → cost, price, quantity, expiry) against the shop's own products.
- **Offline**: after the first sign-in the whole app works with no network. Work waits in a **queue** and goes
  up when the shop's Suite is reachable again (on its own, or Push now). Anything refused stays in the queue
  with the reason.
- Pull again to see what the others added: the shop's Suite is the only meeting point.
- The count sheet export (Onboard has it), from the last pull.

## 3. Non-goals

- No catalogue, no shop profiles, no list of shops, no supplier catalogue.
- No support secrets, no install-ID / hourly code, no system user.
- No team sync, device sync or backup file: phones never talk to each other.
- No till, sales, money, customers or reports.
- Never syncs with Karots Suite Onboard. Both just read from and write to the same Suite; each sees the other's
  work after a pull.
- No iPhone, no Linux build (user: "another Android app").

## 4. Anti-goals (a failure even if it works)

- A cashier getting into the Suite's admin web pages because of this.
- A phone reaching any shop other than the one it signed in to, or anything beyond products and their
  supporting lists.
- Breaking a shop's books: old stock that books money (it's opening stock, as Onboard does it).
- Two people's offline work making twin products of the same barcode, or one person's queue being pushed as
  another person.
- Touching or breaking Karots Suite Onboard.

## 5. Constraints

- The Suite's rules (`AGENTS.md`): core stays plugin-inert, API types in `dto*.go` + `make types`, files ≤ 500
  lines, docs move with the product, money only through `cashflow` (the app books none).
- The Suite's login limiter, session cookie and "one person, one device" (migration 00018) exist; the builder
  must make Shelf live with them (see §8).
- Reaches the shop over its Wi-Fi (or Tailscale), like Onboard.

## 6. Decided (by the user)

- **Name:** Karots Suite Shelf; short name Karots Shelf.
- **Who:** admins and managers always; **cashiers and sales people only when the admin ticks a new permission
  for them on the Users page** (wording e.g. "Can use Karots Shelf"). Unticked, the phone tells them to ask the
  owner. After setup the admin unticks them.
- **The tick is the whole trust.** The phone number and PIN are only for signing in; the app ignores every other
  permission (Can see cost etc.). Anyone with access can do everything the app does: costs, prices, stock,
  batches, expiry, suppliers, categories. Changes go straight into the shop, no approval step.
- **Pulls** products, suppliers and categories of the shop (and what the editor needs: units, fields, packs).
- **Offline:** works offline; work waits in the queue.
- No sync between devices; pulling from the shop is enough.
- The user keeps using Onboard alongside it; the two never sync.
- Android app, mostly a copy of Onboard with small changes.
- **Only for a shop's early setup:** staff use it while the shop is being filled, then stop (the admin unticks
  them).
- **A Shelf sign-in is not a device** under "one person, one device": a cashier can stay on the till (or Pocket
  Cashier) and use Shelf at the same time; signing in to Shelf never throws a till off and is never refused for
  a till that's open.
- **Free:** part of the Suite for every shop. No plugin, no price row, no First Schedule tick, no legal change.
- **The user or the user's team installs the APK** on staff phones; shops don't download it, and the Suite
  doesn't serve it.

## 7. My calls (the user can change them)

- **One codebase with Onboard, two apps.** Shelf is a second Android flavour / entry point in the Onboard repo
  (`~/Projects/flutter/karots_suite_onboard`), with its own application id, name and icon, so it installs beside
  Onboard. Shared code (scanner, OCR, Identify, product form, packs, batches, push, queue) is shared, so a fix
  reaches both; Onboard-only screens simply aren't wired into Shelf. *(Rather than a copied repo, which would
  drift.)*
- **Connect once:** the app asks for the shop's address, then phone + PIN. The address is kept.
- **Lock:** locks after idle minutes; unlocks with that person's PIN checked on the phone (a salted hash kept
  from the last online sign-in), so it works offline. A different person signing in on the phone gets their own
  queue; one person's queued work is never pushed under another's sign-in. *(Fingerprint / phone lock as in
  Onboard, optional.)*
- **Offline knowledge:** the phone only learns that access was removed, or a PIN changed, at its next push or
  pull; then it stops that person (their queue stays on the phone, shown as "couldn't send: no access").
- **Twins:** a queued new product whose barcode the shop already has (someone else added it meanwhile) is
  refused and left in the queue with that reason, offering "open the shop's one"; it is never made twice.
- **Activity:** the shop's Activity log shows who added or changed each product, under their own name (not
  hidden like Onboard's system user).
- **AI Identify:** as Onboard: connected, through the shop's own AI Assistant (if it has it); offline, copy the
  prompt.
- **Sending after a long time offline:** the PIN is never stored. Unlocking needs the PIN, so while the app is
  unlocked it holds the PIN in memory, and if the shop's session has run out it signs in again by itself
  before pushing. Locked, nothing goes up until someone unlocks.
- **Stock-take turned off:** old stock and Quick add go through the shop's stock intake. If the shop has
  Stock-take off, the app asks "Turn on Stock-take in <shop>?" as Onboard does, and anyone with Shelf access
  may turn it on (the tick is the whole trust).
- **Suite access:** the new user flag lets those users' requests through to the exact API routes the app uses
  (products, packs, barcodes, batches, units, categories, suppliers, Product Plus, stock intake, AI identify),
  and nowhere else. The web admin stays admin / manager only.

## 8. Sketch of the how (builder may change)

**Suite side (one Release):**
- Migration: `users.can_use_shelf boolean NOT NULL DEFAULT false` (name for the builder). Users page: the tick,
  shown for cashier and sales person (admin / manager always have it). `/me` reports it.
- A guard: a request from a user with the flag (or admin / manager) passes on an allow-list of the routes the
  app calls; everything else under `/admin` stays as it is. Plugin routes the app uses (Product Plus,
  Alternatives read, AI identify) get the same guard through a generic seam, never core naming a plugin.
- Login from the app: the existing `/auth/login`; the builder checks how "one person, one device" and the
  device id treat a phone that is not a till (a cashier signed in on the till must not be thrown off by
  signing in to Shelf, and the reverse). If that needs a device kind, add it generically.
- Docs: a `docs/guide/` page for Shelf (what it is, the tick, that the setup team installs it), the Users page
  guide.

**App side:**
- A Shelf entry point and Android flavour; Onboard unchanged.
- Local data: the one shop's pulled copy (as Onboard keeps per shop), the queue (per signed-in user), the
  session, the PIN hash.
- Screens: connect + sign in · lock · the shop (Pull, Push now, queue, last pull) · New product · Quick add ·
  Products in this shop · Suppliers in this shop · Count sheet. All from Onboard's existing screens with the
  catalogue / profiles / secrets / team parts removed.

## 9. Left to the builder

Screen layout (Onboard's look), the exact route allow-list, local schema, how the flavour is wired, idle lock
minutes, icon.

## 10. Open questions for the user

None left. The builder confirms the route allow-list against the code before planning.
