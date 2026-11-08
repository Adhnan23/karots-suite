# Karots Suite Onboard: design

**Status:** written from brainstorming with the user, 2026-11-08; awaiting the user's approval.

## 1. What it is and why

An **internal Flutter app for the setup team** (the user and helpers who install Karots Suite in shops). It
does for Karots Suite what Karots POS Onboard (`~/Projects/flutter/karots_pos_onboard`) does for Karots POS,
and more:

1. **One catalogue for every kind of shop** (pharmacy, grocery, hardware, motor spares, phone shop…), grown by
   pulling the products and suppliers of the shops already running the Suite. Iman Pharmacy's medicines are
   there when the next pharmacy is set up.
2. **Set up a new shop fast** from that catalogue: pick products, set prices, old stock and supplier, then put
   them into the shop directly (connected) or hand over CSVs.
3. **Keep working on a running shop** when connected: create products in it (each one also saved to the
   catalogue), and change existing ones (name, category, prices…).

Why: setting up a shop's catalogue by hand is the slowest part of an install, and every new shop of a kind
re-types the same products. The old Onboard app proved the catalogue + shop profile idea; it can't speak to
the Suite's richer products (packs, lots, custom fields, Alternatives) and never connects to a shop.

## 2. Goals

- A **catalogue** of products and suppliers, tagged by kind of shop, with where each came from.
- **Pull** from a connected shop: its products, suppliers, plugins, categories, units, Product Plus fields
  (with the categories each applies to, and values) and Alternatives groups. Anything the catalogue already
  has is skipped; everything new goes to a **Review** list to accept, fix or drop.
- **Shop profiles**: one per shop, its address, kind, and what was pulled from it.
- **Add products to a shop**, from the catalogue or new, each with:
  - supplier: one from the catalogue, a new one, or none;
  - cost and selling price (and wholesale);
  - **old stock**, one of: none (product only) · one batch (a quantity) · several batches, each with its own
    quantity, expiry, cost and price;
  - **packs** (Card ×10, Box ×100…) with their own price and barcode; extra barcodes;
  - **Product Plus** custom field values the shop has.
- **Change existing products** in a connected shop (name, category, prices, fields…).
- **AI help**: Identify with copy-the-prompt, filling custom fields (as the Suite's Settings → Identify does).
- **OCR on every input**, instead of typing: each text field (name, local name, barcode, prices, expiry,
  quantity, a custom field…) has a scan button; photograph the label, box or invoice, tap the words that
  belong in that field, one field at a time (the old app did this for the name only).
- **Duplicates**: find and merge duplicate products in the catalogue, and inside a connected shop (through
  the Suite's own product merge, so every plugin fixes its own rows).
- **CSV export** of a shop's products (with custom field columns) and suppliers, for a shop that can't be
  reached; the shop imports them through Products / Suppliers → Sheets.

## 3. Non-goals

- Not a till and not for shop staff: only the setup team uses it.
- Old stock is **not a delivery**: no supplier bill, no money, no supplier balance change (user: "old stocks,
  not tracked"). New deliveries afterwards go through the shop's own purchases.
- No customers, sales, money or reports.
- Never touches Karots POS or the old Onboard app (separate repo; code may be copied, not shared).

## 4. Anti-goals (would be a failure even if it works)

- Breaking a shop's books: opening stock that books money, or products that can't be sold.
- A pull or push that silently overwrites a shop's own work.
- Core Suite code that knows about this app or a plugin (AGENTS: core is plugin-inert; seams only).
- A setup that only works with the shop online (it must work offline and push later, or export CSV).

## 5. Constraints

- **Flutter**, like the old app; one codebase. Android and Linux (user). Camera OCR and scanning on Android;
  on Linux a USB scanner or typing, and OCR from a photo file.
- Local storage on the device (SQLite), so it works with no network.
- **Signs in to each shop as its system user** (`0000000001`), whose PIN rotates hourly (install ID + HMAC
  of the support secret). Every shop has it; staff never see it.
- The Suite's rules hold: money only through `cashflow`, plugins change each other only through hooks, new
  API types in `dto*.go` + `make types`, files ≤ 500 lines, docs and legal move with the product (this is an
  internal tool, so the shop-facing legal pack only changes if the Suite gains a shop-visible feature).

## 6. Decided (by the user)

- Catalogue fed by pulling shops; for all kinds of shops.
- Pull: already-known products skipped, new ones to Review.
- Stock entered here is old stock: none / one batch / several batches with expiry and prices.
- Supplier per product: existing, new, or none.
- Packs, AI Identify with custom fields.
- Duplicates merged both in the catalogue and inside a connected shop.
- Alternatives: pulled, and also created in a shop the app sets up.
- Name **Karots Suite Onboard**, its own repo at `~/Projects/flutter/karots_suite_onboard`.
- OCR on every input field, filled one by one by tapping the words from a photo.
- Connected: create products directly in the shop and keep a copy in the catalogue; also update existing
  products (name, category…).
- Pull also brings Product Plus fields (and their categories) and Alternatives.
- Sign in as the system user.
- Identify uses each shop's own AI Assistant settings, pulled from it.
- It's the user's own tool, on the user's own devices (never a shop's). **Android and Linux.** Reaches a shop over
  **Tailscale or the shop's Wi-Fi**.
- The app works out each shop's hourly system-user PIN itself from the support secret it holds *(my call:
  the app opens only with the user's own PIN / fingerprint, so a lost phone doesn't open every shop)*.
- Its changes stay hidden from the shop's Activity, like everything the system user does.
- *(My call, Wi-Fi:)* on a shop's Wi-Fi the sign-in travels unencrypted (Tailscale is encrypted); the app says
  so on the connect screen.

## 7. Sketch of the how (builder may change)

**App data (local):** catalogue products (identity, prices as suggestions, packs, extra barcodes, custom
field values keyed by field label, kind tags, source shops), catalogue suppliers, review queue, shop profiles
(address, install ID, kind, plugins, last pull), per-shop pending changes (offline queue).

**Matching:** by barcode (any of a product's codes), else by normalised name; a field by label,
case-insensitive. Duplicates in the catalogue: same rules plus a "look alike" list the user confirms.

**Talking to a shop:** the Suite's existing JSON API under `/api/v1`, as the system user with the session
cookie. Most of it exists: products (create with packs and barcodes, update, barcodes, packs, merge),
categories, units, suppliers, Product Plus (`/x/productplus/fields`, `/form`, `/grid`, `/values`),
Alternatives (`/x/alternatives…`). Likely Suite additions, to be confirmed by the builder:
- a "what is this shop" call: version, install ID, enabled plugins, kind;
- old stock with **several batches, each with expiry and price**, on create (intake's opening count is one
  batch today) — added as a seam that books stock but no money;
- a paged "everything" pull of products with their packs, barcodes, lots summary and plugin extras, so a
  pull isn't thousands of calls (plugin data through a generic hook, never core knowing a plugin).

**AI per shop (user):** a pull also brings the shop's AI Assistant settings (naming style, custom
instructions, Settings → Identify field ticks), so Identify for that shop follows its rules. The shop never
sends its API key out (by design), so *(my call)*: **connected**, the app asks the shop's own
`/x/aicatalog/identify` (the shop's settings and key do the work); **offline**, copy-the-prompt built from the
pulled settings.

**AI:** copy-the-prompt (paste mode) in the app, mirroring the Suite's Identify prompts, including the field
list; the reply read back the same way. *(My call: paste mode first; an API key later if wanted.)*

**OCR:** on the device (ML Kit, as the old app). One shared "scan into this field" sheet: photo → recognised
lines/words → tap to build the value → editable → into the field. Number fields keep only digits and the
decimal point; a date field reads common date forms.

**Prices in the catalogue** *(my call)*: a product keeps the last price seen per kind of shop as a
suggestion; a shop's own prices never change the catalogue's identity fields.

## 8. Left to the builder

Screen layout (following the old app's flow and the Suite's look), the local schema, paging and batching,
how the review list groups things, the CSV column mapping (the Suite's Sheets format, extras included).

## 9. Open questions

None left for the user. The builder confirms the Suite additions in §7 against the code before planning.
