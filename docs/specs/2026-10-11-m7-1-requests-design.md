# M7.1 — Customer requests (design)

Rethink approved by the user on 2026-10-11: **A–E all yes**. Special / advance orders (bakery cakes,
print jobs with a deposit and a due date) are **parked for after M7.10**. They need money and a job
lifecycle, which this plugin doesn't have.

## 1. What karots-pos does (kept)

- **Till:** "📝 Note request". The note is required; customer name and phone are optional. Any
  signed-in cashier can use it.
- **Admin:** a list of requests, with edit, remove and **Make product**. Make product asks for name
  (from the note), category, unit and an optional price. The product starts at cost 0 with no stock,
  and the request is marked `converted` with its `product_id`. **Clear converted** removes the done
  ones.
- **Data group:** "Customer requests / wanted".

## 2. What changes (user, 2026-10-11)

- **A. One wanted item, many asks.** While the cashier types, matching requests appear. **Asked
  again** adds an ask (with its own optional name and phone) instead of a duplicate. Admin rows read
  "Asked 3× · last 2 days ago", sorted by asks. **Merge** folds one open request into another, and
  every ask moves with it.
- **B. Three ways to close a request:**
  - **Make product:** as before, plus optional cost and supplier.
  - **Link to an existing product:** for an item that came in another way.
  - **Won't stock:** needs a reason. The request is kept, and the till shows "Asked before — won't
    stock: <reason>" on a matching note.
  - **Reopen** undoes any of the three.
- **C. Waiting list.** A request with a product and at least one ask that left a phone number and
  hasn't been called:
  - shows **Waiting for stock** while the product has none;
  - moves to **To call** once the product has stock, with a `tel:` link and **Called ✓** per ask.
- **D. Screen.** Apps → Requests, with tabs **Open · To call · Done**, search, paging and 360px
  cards. On the till, a "Note request" chip in the action row (`slots.tillActions`).
- **E. Till no-match (new generic seam).** When a till search finds nothing, the Quick item dialog
  also lists the till actions that declare `noMatch`. For this plugin that's "Not stocked? Note a
  request", with the search text prefilled.
- **Kept as karots-pos:** the selling price on Make product stays optional. (Open question; the user
  didn't pick. A Rs 0 product can be sold by mistake, so flip it to required if asked.)

## 3. Stages (computed, never stored)

| stage | rule | tab |
|---|---|---|
| `open` | status `open` | Open |
| `waiting` | `converted`, someone to call, product has no stock | Open (chip "Waiting for stock") |
| `call` | `converted`, someone to call, product has stock (`stock.quantity > 0`) | To call |
| `done` | `converted` with nobody to call, or `declined` | Done |

"Someone to call" means an ask with a phone number and `called_at` empty.

**Clear done** deletes `converted` rows in Done only. Won't-stock rows stay, because they are the
memory the till shows; each one can be removed on its own.

## 4. Data and old backups

- `0001_requests.sql` is kept **byte for byte** from karots-pos.
- New `0002_asks.sql`:
  - `requests` gains `reason TEXT NOT NULL DEFAULT ''` and `called_at TIMESTAMPTZ`.
  - Status gains `declined`; `converted` now means "has a product", whether made or linked.
  - New table `request_asks` (`id`, `request_id` → `requests` ON DELETE CASCADE, `customer_name`,
    `customer_phone`, `created_by`, `created_at`, `called_at`).
- **The first ask is the request row itself**, and repeat asks live in `request_asks`. A karots-pos
  row is therefore exactly one ask, and **a backup restores with zero mapping**. The store reads the
  row and the table as one list (`UNION ALL`, where the first ask has id 0).
- Data group `requests`: tables `requests` and `request_asks`.

## 5. API (`/api/v1/x/requests`)

**Cashier** (signed in):

| call | does |
|---|---|
| `GET /requests/similar?q=` | up to 5 requests in stages open / waiting / declined; every word must match the note |
| `POST /requests` `{note,name,phone}` | a new request |
| `POST /requests/:id/asks` `{name,phone}` | asked again (refused on a `call` / `done` converted one) |

**Admin + manager:**

| call | does |
|---|---|
| `GET /requests?tab=open\|call\|done&q=&page=` | rows with their asks, plus a count per tab |
| `PUT /requests/:id` `{note}` | edit the note |
| `PUT /requests/:id/asks/:ask` `{name,phone}` | edit an ask (ask 0 = the row) |
| `DELETE /requests/:id/asks/:ask` | remove an ask (ask > 0 only) |
| `POST /requests/:id/asks/:ask/called` | mark an ask called |
| `DELETE /requests/:id` | remove a request |
| `POST /requests/:id/product` `{name,categoryId,unitId,sellingPrice,costPrice,supplierId}` | make product |
| `POST /requests/:id/link` `{productId}` | link to an existing product |
| `POST /requests/:id/decline` `{reason}` | won't stock |
| `POST /requests/:id/reopen` | back to open |
| `POST /requests/:id/merge` `{into}` | merge (both must be open) |
| `POST /requests/clear-done` | delete the converted rows in Done |

Every admin write is audited through `core.Audit`.

## 6. Core seam (E)

In `web/src/plugins/slots.ts`:

- `TillAction` gains `noMatch?: string`, the label shown in Quick item.
- Its Panel gains `text?: string`, the prefill.

In the till:

- `TillUi`'s `action` dialog gains `text?`.
- `QuickItem` renders one link per action with `noMatch`, which calls
  `ui.open({ kind: 'action', key, text: name })`.

It's generic and names no plugin. A core-only build shows nothing new.

## 7. Testing

- **Go:**
  - Store DB tests: stages, similar matching, ask again, merge moving the first ask, clear done
    sparing declined ones.
  - API tests: 403 for a cashier on admin routes, decline without a reason → 400.
  - Restore test: a karots-pos-shaped `requests` archive.
- **Web:** unit test for the ask-count / "last asked" label.
- **e2e** (`plugin-requests.spec.ts`):
  1. The till chip notes a request, then asks again on a match.
  2. A no-match search → Quick item → "Note a request" with the text prefilled.
  3. Admin: Make product, then stock that product through Stock → Intake, then To call → Called ✓,
     then Done.
  4. Won't stock shows on the till.
  5. Merge.
  6. 360px with no side scroll.

## 8. Out of scope

- Special / advance orders (after M7.10).
- Telling customers by SMS.
- Requests in Reorder or Activity.
