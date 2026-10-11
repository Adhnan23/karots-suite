# M7.2 — Product Plus (design)

The user approved the rethink on 2026-10-11 with **A–E all yes**. For E, export / import with custom fields
is a **toggle**: when it's off, a sheet carries only the core product columns, exactly as today.

## 1. What karots-pos does (kept)

- **Field manager:**
  - Types: text, long text, number, yes/no, dropdown and date.
  - Each field has a label, a key slugged from the label (unique, never changes), a default, help text, and
    switches for required, searchable, till ⓘ, label and stock intake.
  - ▲▼ order, disable / enable (with a Show disabled filter), and every change audited (`product_field`).
- **Values are sparse.** A product with no `pp_values` row reads the field's default. Saving a value equal
  to the default deletes the row.
- **Product form and intake "New item".**
  - The fields appear in both. Intake shows only the fields marked for intake.
  - The server checks number, dropdown option and `YYYY-MM-DD` date.
  - A dropdown value whose option was removed stays selected as "X (removed)".
- **Search.** A searchable field matches on any part of its value. A yes/no field is found by its
  **label** when the answer is Yes.
- **Admin list line** ("Model No: XJ-9 · Grade B · Waterproof"). Stored values only; yes/no shows its
  label when Yes; long text is cut at 40 characters.
- **Till ⓘ rows and label top / bottom lines.** The default shows too; blanks and No are skipped; values
  are cut at 120 / 40 characters.
- **Duplicate product** is already in suite core, so nothing is left for the plugin here.

## 2. What changes (user, 2026-10-11)

- **A. Fields per category.**
  - A field applies to **all products** (the default) or to chosen categories, **including their
    subcategories**.
  - Every surface shows a field only for products in its scope: form, intake, search, list line, ⓘ,
    label, Fill in and sheets.
  - The product form re-checks the scope when the category changes.
  - A product moved out of scope **keeps** its stored values. They're hidden, and they come back if it
    moves back.
- **B. Fill in.** A grid on the plugin page:
  - Pick one or more fields (at 360px, one at a time), a category (optional, subcategories included),
    **Missing only** and a search, with paging.
  - Rows are products, and cells are typed inputs, the same controls as the form.
  - Edited cells are marked, and **Save** writes them all in one call. A bad cell is reported on that
    cell and the good ones still save.
- **C. Required becomes real.**
  - The "a required field needs a default" rule is gone.
  - **Missing** means: an active product in the field's scope, with no stored value and a blank default.
    A yes/no field is never missing.
  - The field list shows "14 products missing", which opens Fill in on that field with Missing only.
  - The product form and intake still refuse a blank required field that they show.
  - Quick-add, Stock Capture and sheet import never block. Their products simply show up as missing.
- **D. Safer edits.**
  - Changing a default shows, before saving, "Applies to N products with no value of their own".
  - Once a field has stored values, its **type is locked**, except between text and long text.
  - Removing a dropdown option keeps products that have it. They show "X (removed)" as today, and Fill
    in can find them.
- **E. Sheets toggle.**
  - Products → Sheets ▾ gets **Include custom fields** (shown only when a plugin offers columns). It
    applies to export, the template and import.
  - When it's on, there is one extra column per active field, headed by the field label. Out-of-scope
    cells are blank on export and ignored on import.
  - **When it's off, nothing changes:** core columns only, and custom columns in an imported sheet are
    ignored.
  - On import:
    - A blank cell leaves the value as it is (the core "only the columns your sheet fills" rule).
    - A bad cell (not a number / not an option / not a date) skips **only that value** and adds a note on
      that line.
    - Yes/no reads yes / no / true / false / 1 / 0.
- **F. Core: Skip disabled (user, 2026-10-11).** Products → Sheets ▾ export gets a **Skip disabled**
  toggle, **on by default**, which is today's behaviour (`ListAll` is active-only).
  - When it's off, disabled products come too, with an extra `active` column (yes / no).
  - Import reads `active` only when the sheet has that column: "no" disables a matched or new product,
    "yes" re-enables it. Without the column, nothing changes.
  - It's core, not the plugin, and applies to products only (customers / suppliers keep their current
    export).
- **Merged.** The three "show" switches become one **Where it shows** chip row: Till ⓘ · Label · Stock
  intake. The list line is always on, as in karots-pos.

## 3. Core seams (generic; no plugin named)

1. **Product extras.**
   - `ProductRequest.extra map[string]json.RawMessage` holds opaque data per plugin key (TS:
     `Record<string, unknown>`).
   - Go hook `plugin.ProductExtras{Key, Validate(ctx, ExtraCtx, raw) error, Save(ctx, ExtraCtx, raw) error}`,
     where `ExtraCtx{ProductID, CategoryID, IsNew}`.
   - Admin create / update and intake create call `Validate` for every key **before** the product is
     written, and `Save` **after** it (core's product writes aren't one tx either: Update + reprice).
     An unknown key → 400.
   - React slot `productFormSections: { key, Section }`. The Section receives
     `{ productId, sourceId, categoryId, where: 'form' | 'intake', value, onChange }`; `sourceId` is the
     product being duplicated, so a copy carries the values over.
   - `ProductForm` and intake `NewItem` render the sections and send `extra`.
2. **List line.** The existing `ProductMetaProvider` gets bridged (`products.MetaProvider`), and
   `AdminProductRow.meta` is shown under the name on the Products page (table and cards).
3. **Sheet columns.**
   - Go hook `plugin.ProductSheetColumns{Key, Label, Columns(ctx) ([]string, error),
     Export(ctx, ids) (map[int64][]string, error), Import(ctx, productID, categoryID, cells map[string]string) (note string, err error)}`.
   - `GET /admin/products/sheet-extras` returns `[{key, label}]`.
   - Export, template and import take `extras=<key,…>`.
   - `products.ImportOne` returns the product id.
   - `SpreadsheetMenu` shows the toggle per extra (remembered per device in `localStorage`), for
     products only.

4. **Skip disabled** (F) is a core feature, not a seam:
   - `products.ListAll(ctx, includeDisabled bool)`;
   - export takes `disabled=1` (adds the `active` column);
   - `ImportRow.Active *bool`.

Apart from F, a core-only build shows and sends nothing new.

## 4. Data and old backups

- karots-pos `0001`–`0004` are copied **byte for byte**.
- **New `0005_field_scope.sql`:**
  - `pp_field_categories (field_id → pp_fields ON DELETE CASCADE, category_id BIGINT, PK (field_id, category_id))`,
    with no FK to core `categories` (the plugin's schema versions on its own). **No rows = all products.**
  - A category deleted from core just drops out of the scope at read time.
- Data group `productplus`: `pp_fields`, `pp_values`, `pp_field_categories`.
- **A karots-pos backup restores with zero mapping.** Every old field has no scope rows, so it applies to
  every product, as before. Its stored values are untouched. A required field that has a default can't
  be "missing", so nothing old turns red.
- **Scope rule (one SQL helper):** a field applies to product `p` when it has no scope rows, or
  `p.category_id` is in the subtree of a scope category (recursive CTE over `categories.parent_id`).

## 5. API (`/api/v1/x/productplus`, admin + manager)

| call | does |
|---|---|
| `GET /fields?disabled=1` | fields with `scope: [{id, name}]`, `valueCount`, `missing` (required only), `usingDefault` |
| `POST /fields` / `PUT /fields/:id` | create / edit (type lock, options, scope); audited |
| `POST /fields/:id/active` `{active}` · `POST /fields/:id/move` `{dir}` | as karots-pos |
| `GET /form?productId=&categoryId=&where=form\|intake` | fields in scope with each one's value for the product (default when none, `removed` flag) |
| `GET /grid?fields=1,2&categoryId=&missing=1&q=&page=` | `{rows: [{id, name, categoryName, values: {fieldId: value \| null}}], total}`; `null` = reads the default |
| `PUT /values` `[{productId, fieldId, value}]` | Fill in save; answers `{saved, errors: [{productId, fieldId, message}]}` |

Every write is audited. A product's extras save goes through seam 1, not through this API.

## 6. Testing

- **Go:**
  - Store DB tests: the scope (a subcategory inherits, a moved product hides and keeps its values),
    missing (a default or a yes/no is never missing), the type lock, sparse save, search by value and by
    yes/no label within the scope, meta, ⓘ and label rows.
  - Core API tests with a fake extras hook (validate refusal → 400 and nothing written; unknown key →
    400) and a fake sheet column (export / import with and without `extras`).
  - Plugin API tests (403 for a cashier, the grid's per-cell errors).
  - Restore test with a karots-pos-shaped `pp_fields` / `pp_values` archive.
- **Web:** unit tests for value coercion (yes/no words, number / date checks shared by the form and
  Fill in).
- **e2e** (`plugin-productplus.spec.ts`):
  1. Add a scoped required dropdown; it shows on the form only in that category and blocks a blank save.
  2. The value is searchable at the till and shows in ⓘ and on the label options.
  3. Intake shows only intake fields.
  4. Fill in → Missing only → save two cells.
  5. Export with the toggle on and off (header columns), and import a value back. Export with Skip
     disabled off includes a disabled product with `active` = no, and a re-import keeps it disabled.
  6. The type lock message.
  7. 360px with no side scroll.

## 7. Out of scope

- Filtering the product list by a field value (search covers it).
- Custom fields on receipts (dropped in karots-pos on purpose).
- Fields on customers / suppliers.
