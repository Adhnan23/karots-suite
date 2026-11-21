# Product Plus — custom product fields (PPF)

Code: `plugins/productplus` (store.go fields, values.go values + checks, scope.go, grid.go Fill in + sheets,
attributes.go, merge.go, hooks.go), `web/src/plugins/productplus`. Tables: `pp_fields`, `pp_values` (no row = the
field's default), `pp_field_categories` (no rows = every product).

## The words the rules use

- **Field** — text, long text, number, yes/no, dropdown or date; **required**, **searchable**, shown at the till, on
  labels, at intake; **active** or disabled.
- **Scope** — the categories a field applies to, with their subcategories; none = every product.
- **Fits** — a value its type accepts: a number parses, yes/no is `true` / `false`, a dropdown value is one of its
  options, a date is a real `YYYY-MM-DD`.

## The grid

Columns: **Save** (form, intake, sheet import, Fill in, Karots Shelf / AI through the attribute set) · **Show** (till ⓘ,
label, list line, search, sheet export) · **Field change** (type, options, scope, disable) · **Merge** (product,
category + undo) · **Faults**.

| Feature | Save | Show | Field change | Merge | Faults |
|---|---|---|---|---|---|
| A value | PPF-1, PPF-2 | PPF-3 | PPF-4 | PPF-6 | PLG-3,4,5 |
| A required field | PPF-2 | — a | PPF-4 | — a | PLG-5 |
| A scoped field | PPF-2, PPF-3 | PPF-3 | PPF-5 | PPF-7 | PLG-5 |
| A disabled field | PPF-8 | PPF-8 | PPF-8 | PPF-6 | — b |
| A field itself (label, key) | PPF-9 | — a | PPF-9 | — a | PLG-5 |

a — nothing more to say. b — as any value.

## Rules

```
PPF-1  Every stored value fits its field: whatever path saved it (form, intake, sheet, Fill in, the
       attribute set), a value that doesn't fit is refused and the product keeps what it had.
       Checked by: ledgercheck, example.

PPF-2  A product in a required field's scope can't be saved from the form without it; a field outside a
       product's scope is neither asked for nor saved.
       Checked by: example.

PPF-3  A field shows only on products in its scope (its categories and their subcategories, or every
       product); search finds only searchable, active fields.
       Checked by: example.

PPF-4  A type or dropdown change that would drop values is refused until confirmed; once confirmed, values
       that fit are converted, the rest dropped — PPF-1 holds after.
       Checked by: example.

PPF-5  A value set while a product was in scope is kept when it moves out (hidden) and shows again if it
       moves back.
       Checked by: example.

PPF-6  A product merge keeps the kept product's value for each field; the duplicate's value moves over only
       for fields the kept one hasn't set (PLG-2).
       Checked by: example, ledgercheck.

PPF-7  A category merge carries the merged category's scope onto the kept one; undo removes only the scope
       rows the merge added.
       Checked by: example.

PPF-8  A disabled field keeps its values and is hidden everywhere (form, till, label, search, sheet);
       enabling it brings them back.
       Checked by: example.

PPF-9  Field labels are unique and never a core products sheet column; keys are unique.
       Checked by: refusals, ledgercheck.
```
