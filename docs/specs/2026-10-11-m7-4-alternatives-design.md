# M7.4 — Alternatives (design)

The user approved the rethink on 2026-10-11: **A–G all yes**.

## 1. What karots-pos does (kept)

- **Group → tiers → products.** A group is a set of interchangeable products ("USB 32GB"); tiers are quality
  levels inside it (Genuine / Normal / Cheap). A product is in **one group only** (`alt_members.product_id`
  is the key). Groups and tiers can be turned off.
- **Tier reorder level:** a tier is **low** when the summed stock of its active members is at or under its
  level (0 = not tracked). A low product whose tier is above its level is **covered**.
- **Till search** finds the whole group when the search matches a group name, a tier name or any member's
  name (`ProductSearchContributor`, already in the suite).
- **Audit** on every change (`alt_group`, `alt_tier`, `alt_member`).

## 2. What changes (user, 2026-10-11)

- **A. Tiers optional.**
  - A new group gets one tier with a blank name. Until a tier is named or a second one is added, the group
    page shows just its products and "Reorder when the group has ≤ N" (the blank tier's level).
  - Tiers are ordered **best first** with ↑ / ↓ (`sort_order`). That order ranks what the till offers.
  - An empty tier can be deleted; a group can be deleted (its memberships go, products stay).
- **B. Swap at the till.**
  - A plain cart line (not a group line, not a service) whose qty is more than its stock shows a chip under
    its name: **⇄ SanDisk 32GB · Genuine · 14 left · Rs 1,800 · Swap**, up to 3 in-stock alternatives, best tier
    first, then most stock. Price is the sale type's price (wholesale on a wholesale sale).
  - Swap replaces the line's product in place and keeps its qty; discount, lot, serials and found-at-till
    reset. If the cart already has a plain line of that product, the qty merges into it.
  - **Core seam (generic):** `TillApi.swapLine(key, productId)` → reducer action `swap` (`lines.ts
    swapProduct`). Core names no plugin.
- **C. ⓘ popup.** The till's product info lists the product's alternatives with stock and price
  ("Alternatives: SanDisk 32GB — 14 left · Rs 1,800"), via the existing `ProductDetailContributor`.
- **D. Search and pin.** Search unchanged. The till tile pin is **⇄ Genuine**, or **⇄** when the group has no
  named tiers.
- **E. One reorder surface.**
  - **Core seam (generic):** core's Reorder page calls `ProductReorderAnnotator` (already defined, never
    wired). With any annotator: every row shows its note, and a **Hide covered items** toggle
    (`?hideCovered=1`) hides covered rows and pulls in `LowMembers` (via `ListQuery.AlsoIDs`: "low, or one of
    these"). Filtering happens **before paging** (karots-pos filtered the page only). `ReorderList.annotated`
    tells the page to show the toggle. Core-only: unchanged.
  - The standalone overview page is gone. The group list shows each tier's total vs its level with a
    **Low** chip, a **Low only** filter and **CSV** (group, tier, total, level, status).
- **F. Product form.** An **Alternatives** section on the product form and intake New item: group, then tier
  (only when the group has named tiers), or None. A duplicated product starts in its source's group. Via
  `productFormSections` + `ProductExtras` (Validate: the tier exists; Save: add / move / remove).
- **G. Health.** Disabled or deleted members show greyed on the group page with Remove; they never count in
  totals, search, pins or swaps. The admin product picker says "in USB 32GB" for a product already grouped;
  adding it asks **Move here?**.

## 3. Data

- karots-pos `0001_alternatives.sql` **byte for byte**; no new migration. A tier-less group is a tier named
  `''`. An old backup restores with **no mapping** (`restore_db_test.go`).
- Stock totals are decimal strings (karots-pos cast them to int).

## 4. API (`/api/v1/x/alternatives`)

| Route | Who | What |
|---|---|---|
| `GET /` | admin | groups with tiers (total, level, low), member counts; `?disabled=1` |
| `GET /:id` | admin | one group: tiers in order, each with members (name, stock, active) |
| `POST /` · `PUT /:id` · `POST /:id/active` · `DELETE /:id` | admin | group create (+ blank tier) / rename / on-off / delete |
| `POST /:id/tiers` · `PUT /tiers/:tid` · `POST /tiers/:tid/active` · `POST /tiers/:tid/move` · `DELETE /tiers/:tid` | admin | tier add / edit (name, level) / on-off / ↑↓ / delete when empty |
| `POST /tiers/:tid/members` (`{productId, move}`) · `DELETE /members/:pid` | admin | add (409 `IN_OTHER_GROUP` with the group name unless `move`) / remove |
| `GET /membership?ids=` | admin | product id → group name (picker hints) |
| `GET /product/:pid` | admin | the product's `{groupId, tierId}` (form section) |
| `GET /till/:pid` | cashier | in-stock alternatives, ranked (swap chip) |

## 5. Out of scope

A product in several groups, unit conversion in tier totals, a substitution report.
