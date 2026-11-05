# AI Assistant jobs, searchable hints, Alternatives levels, packs on create

Status: approved by the user 2026-11-05. Delete with its plan when done (AGENTS.md).

## Why

The suite's main promise, in the user's words: **even someone who knows nothing about the stock can still use it.** A new
cashier asked for "a brake pad for a CT100" should find it even though the product's name is a part number and a brand.
The chatbot already sends those details (`specs`, `explanation`), but they are as good as lost once saved.

The user will run **every product** through AI Assistant, so one-at-a-time Identify is too slow, and organising the items
(groups, fields, packs, doses) should be AI suggestions too.

Reading the code also showed Alternatives and smart reorder barely work together, and that packs can't be set while
creating a product. Both are in this work.

## Goals

1. **Hints:** AI details are kept, shown, searchable and correctable, with no other plugin needed.
2. **Whole-list jobs:** work through many products in batches, with batches as small as one for quality control.
3. **Plugin-aware jobs:** extra AI jobs appear only for plugins in the shop's build. Those plugins don't know AI exists.
4. **Alternatives levels:** groups and tiers learn their reorder level, and low stock judges and shows them as a group.
5. **Packs and extra barcodes** can be set while creating a product.

## Non-goals

- Pocket Cashier release 2, and hints or these jobs on the phone. Pocket adapts from its own side in release 2.
- Creating products from a pasted list of names. Items get in by sheet import or by hand; jobs work on existing products.
- Suggesting quality tiers. The group maker makes single-tier groups; the user splits tiers by hand.
- Combo suggestions from sales history, and any AI-set combo price.
- Blocking Friday's install. Iman installs from the current Release; this reaches him as an update once tested.

## Anti-goals (a failure even if it works)

- **AI changes anything without a tick.** Every job: suggestion → tick → apply. No auto-apply, including in API mode.
- **A wrong strength becomes swappable at the till.** Groups match on ingredient, strength and form, with a visible reason.
- **AI replaces a value the shop set.** Jobs fill empty values; a redo shows old and new, and the old one stays unless ticked.
- **Another plugin's code mentions AI, or core names a plugin** (`make check-core`).
- **AI Assistant writes another plugin's tables.** It saves through that plugin's own code, so its rules hold.
- **A sale is ever stopped by any of this** (never-stop-a-sale rule).

## Part 1: Hints (AI Assistant only)

A hint is the existing `aicatalog_items.specs` + `user_explanation`. No Product Plus involved.

- **Better details:** the prompt asks for what buyers ask about: which vehicles and models a part fits, what kind of medicine
  it is and what it's for. Today it only asks for dimensions and ratings.
- **Searchable:** through `ProductSearchContributor`; typing "CT100" or "fever" at the till finds the product. This covers
  every search using `products.List`. Today AI Assistant contributes no search.
- **Visible:** a short line in search results (`ProductMetaProvider`), and the full specs and explanation in the till's ⓘ
  (`detailRows`, which drops the explanation today). Labelled **AI hint**, so it reads as a hint and not as medical advice.
- **Correctable:** shown and editable on the product page (`productFormSections`).
- **Existing saved details** start working without being asked again.
- Only products that went through AI Assistant have hints (user: fine, every product will).

## Part 2: Identify a whole list (always available)

- Lists products without hints; they drop off as they're done, so work carries on batch after batch.
- **Batch size is the user's choice, from 1 up** (user: single item = quality control). Starts small for medicines.
- **Redo:** pick any product that already has hints, and see old and new side by side.
- Returns name, category and hint. Each is ticked separately. Prices and stock are never touched.
- Same flow as Clean up: copy and paste with any chatbot, or the API mode that already exists.

## Part 3: Jobs that appear with a plugin

Every job works like Part 2 (batch size, tick, apply, fills empties only). Build order is the numbering.

**3a. Alternative group maker** (Alternatives in the build). Single-tier groups only. Each suggestion shows its reason, based
on ingredient, strength and form, and reads hints when they exist. Products already in a group aren't moved; an ungrouped
product may be suggested into an existing group. New groups start on Auto (Part 4).

**3b. Fill in details** (Product Plus in the build). Follows Product Plus exactly:
- Field for all products, or for chosen categories and their sub-categories.
- Types: text, long text, number, yes/no, dropdown (must pick one of the shop's options), date (fixed format).
- **A tick list on the page chooses which fields it may fill**, because a chatbot would make up something like a rack number.
- **Each product is asked only for the ticked fields that apply to its category**, so products in one batch may be asked for
  different fields.
- **The chatbot gets the values other products already use** for each ticked field and must reuse the same wording when one
  fits, so "GSK" doesn't also become "GlaxoSmithKline". A new value is allowed when none fits, and is marked new in the
  preview. The list is capped so prompts stay short (the builder picks the cap, most-used values first).
- Runs after categories are right, since a wrong category means wrong fields; the page says so.
- `SetValues` doesn't check scope, so AI Assistant must, so a bad reply can't save a field onto the wrong product.

**3c. Setup details** (always). Suggests has-expiry yes/no and pack (Card × 10). Prices stay blank for a pack; the user fills them.

**3d. Pharmacy marks** (Pharmacy in the build). Suggests Rx-only and controlled. The page says clearly that **no suggestion
does not mean cleared**, because the chatbot can miss one. The pharmacist still checks the list.

**3e. Usual dose** (Pharmacy in the build; the user asked for it after being told the risk). The chatbot gets the shop's dose
list and must reuse its exact wording, so doses stay consistent. A dose not on the list is marked new, with its meaning and
doses a day. **The pharmacist approves one by one: no tick-all.** Never replaces a dose already set.

**3f. Combo suggestions** (Combos in the build). From a button on the Combos page. Suggests products that go together, from
common sense. Opens the normal combo form with products filled in; **the user sets the price and saves.**

**Buttons on other plugins' pages:** a generic empty page-actions slot, which a plugin page renders without knowing what fills
it; AI Assistant registers its button there. Used on the Combos and Alternatives pages. Every job also lists on AI Assistant's
own page.

## Part 4: Alternatives levels and low stock (useful without AI)

Today a tier's level is typed by hand and starts at 0 = don't track, and a low tier's order splits evenly across brands. So
AI-made groups would do nothing for reorder, and a slow brand gets the same order as a fast one.

- **Auto level:** the sum of the members' reorder levels, worked out on read, so it follows the nightly smart levels. Typing
  a number overrides it, the same as for products. Existing typed levels stay; existing 0s become Auto, as `00009` did for
  products. Typing 0 still means don't track.
- **Split, per group or tier:**
  - **Automatic (default):** order follows each brand's recent sales; an even split if nothing has sold.
  - **Manual:** the user sets the split. If another brand starts selling faster it shows a suggestion, and nothing changes
    until it's tapped (the same idea as "looks off" on fixed levels).
- **Till Low stock tab:** today it ignores coverage, so a cashier can order a brand while its twin is full. It must match
  the Reorder page: note, hide covered by default (with a "show covered" option), and pull in brands of a low group. Check
  the printed low stock list tells the same story.
- **Group view on both pages** (the user approved this layout):

```
Vitamin C 100s                       (no group: same as today)
  3 left · reorder at 10              order 20

Paracetamol 500mg                    (group: all brands together)
  LOW: 12 left in the group · reorder at 50
    Panadol      8 left               order 70
    Brand B      4 left               order 20
    Brand C      0 left               order 10
```

  One block per tier. Core stays plugin-inert: `ReorderNote` gains generic group fields and core renders them.
- **Dashboard low-stock count:** appears not to use coverage (`admin_dashboard.go` uses `products.List`; only
  `admin_reorder.go` uses the annotators). Confirm, then fix through the generic hook.

## Part 5: Packs and barcodes on create (core)

`ProductForm.tsx:179-181` shows `BarcodesBlock` and `PacksBlock` only for a saved product. Let the user add both while
creating, saved along with the product. Every shop gets this, with or without plugins.

## Constraints

- **AI Assistant can't import the other plugins.** A Go import pulls that plugin into every build (`pharmacy.go` does it on
  purpose), and adding them to `requires` would too. So plugins expose what jobs need through **generic hooks in
  `internal/plugin`** that say nothing about AI (for example "this plugin can make product groups"). Prompts live in
  `plugins/aicatalog`. A job appears because its hook is registered. The same applies on the React side.
- The user accepts AI Assistant knowing other plugins' jobs exist; other plugins never know about AI Assistant.
- Plugin hard rules, tokens only, 360px for admin pages, files ≤ 500 lines (`apply.go` is 449, `cleanup.go` 353: new files).
- No Claude or AI attribution in commits. "AI" in product wording is fine.
- Docs move with the product: guide pages for AI Assistant, Alternatives, Combos, Pharmacy, stock and purchasing; retake
  screenshots. No price or legal change is expected.
- Hand testing happens on clean starter data; no test rows left behind.

## Build order

Part 5 (small, helps item loading) → Part 1 → Part 2 → 3a → Part 4 → 3b → 3c → 3d → 3e → 3f. Each part ships working, so
work can stop anywhere and what's done can be Released. **No Release without the user's say**; a new Release means copying
`dist/iman-pharmacy-kalpitiya/` to the USB stick again if it comes before install.

## Done means (user, 2026-11-05)

Implement all ten, test them fully, review them, fix every flaw and bug found in or around this work, and harden it. Only
then, if time is left, start Pocket Cashier release 2 as a separate piece of work; it isn't required for this.

## Settled after the builder check (Claude's calls; the user can overrule them)

- **When a product is done for a job:** a per-job, per-product record (asked, applied, skipped, no answer). A product leaves
  that job's list once answered, even if unticked or given no suggestion, and can be brought back from a "Skipped / no answer"
  filter or by Redo. So a list of thousands keeps moving.
- **Yes/no values:** `has_expiry` and `rx_only` can't show "not looked at", so the record decides that. A suggestion that differs
  from the current value is shown as a change, starts unticked, and needs a tick; nothing flips silently. In 3d, products with
  no answer stay listed for the pharmacist.
- **Group maker batches:** one category at a time, sending every ungrouped name and hint in it, so twins sit side by side. A big
  category is split alphabetically. Batch size counts products to place; a batch of 1 can only match against existing groups
  and the rest of its category.
- **Group maker on other goods:** works for any shop. Must match on the same item type and the same size, rating or fit;
  medicines on ingredient, strength and form. A product with no strength, size or hint in its name or details isn't suggested.
- **Group order total:** `2 × level − stock`, today's formula. The mockup's numbers were illustrative. A tracked group's split
  **replaces** each brand's own suggestion rather than being raised back by core's `max`, through a generic `ReorderNote` field.
  Quantities stay editable before ordering.
- **Joining a group with several tiers:** the user picks the tier in the preview, with no default; the chatbot never chooses.

**Whole-list ticks (proposed to the user; no objection, can be changed):** hint ticked; category ticked only if the product has
none, and a new category is marked new; a differing category and every name start unticked, with a "tick all names" button.
An old name stays searchable after a rename, so a part number staff know still finds the product.

## Prompts and replies (tested with real Gemini and ChatGPT replies, 2026-11-05)

**Prompts work for any shop (user):** no job's prompt assumes a kind of shop, except jobs that only exist with a plugin (Pharmacy
jobs may speak of medicines). Today's "retail/spare-part products" and the bearings examples in `prompts.go` go. The hint asks
what a customer would ask to know it's the right item, with parts and medicines as neutral examples at most. The country stays
Sri Lanka (the suite's market). Settings' custom instructions already reach every prompt through `withCustom`, and that is where
a shop describes itself. Every new job must keep using it.

**Replies must survive these (each one becomes a test, using the real reply):**
- **Unescaped inch marks** (`"specs":"Reach: 12.7mm (1/2"); Hex 16mm (5/8"); …"`, Gemini as pasted). `json.Unmarshal` rejects the whole
  reply today. Add a repair pass for stray quotes in strings before giving up; if repair fails, keep the error message.
- **`confident:false` with invented values** (ChatGPT: "Unknown Product (XYZQ 77)", "Unclassified") or blank ones (Gemini): both count
  as no answer. They never rename, file or fill anything.
- **Whole-list replies** skip blank or unknown items, and ids not in the batch are dropped, never applied.
- **Empty values and null yes/no** mean no suggestion.
- **Doses:** the literal `"new"` (ChatGPT) or a blank dose with `new:true` (Gemini) is never stored. The prompt asks for a real short form
  (`1-0-0`, `every 6 hours`); without one the row shows the meaning and the pharmacist types the form or leaves it. A dose claiming to be
  on the list but not on it counts as new.
- **As-needed doses are `per_day: 0`** (the dose list's meaning, `0009_doses.sql`). Both chatbots answered 3 or 4, which would make Days… work
  out a wrong quantity, so the prompt says so and the preview shows it.
- **Category separators:** `normPath` already handles " / "; keep the shallow-category rule in every job.

**Wording:** don't put "cashier's hint" in prompts; it was copied into the answers. "Confident" doesn't prove accuracy: both chatbots were
confident with entirely different vehicle lists for one spark plug. So hints keep the AI label and a "confirm before selling" line. Both
answered every Rx and controlled mark without using null and without a way to verify, so the warning stays. Amoxicillin's dose split them
(`1-1-1` against refusing), which is why doses are approved one by one.

## The builder's calls

Names and shape of the generic hooks and the slot; the batch screen's layout, reusing `PastePanel` and Clean up's preview; how
long "recent sales" is; the threshold for "selling faster"; whether job runs share Clean up's undo (`aicatalog_optimize_runs`)
or have their own, as long as the last apply can be undone.
