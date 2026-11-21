# AI Assistant (AIC)

Code: `plugins/aicatalog` (jobs*.go: identify, groups, fields, setup, marks, dose; apply.go: the cleanup plan and its
revert; hints.go; combo_suggest.go; parse.go; merge.go). Tables: `aicatalog_items` (what it learnt per product),
`aicatalog_job_items` (per job, per product), `aicatalog_optimize_runs` (applied plans, for undo),
`aicatalog_settings` (the API key is a secret).

**It works through other plugins, never into their tables:** custom fields through Product Plus's attribute set
(Identify's ticked fields, the Fields job), medicine marks and doses through Pharmacy's attribute set, groups through
every plugin's product grouper (Alternatives), category merges through every plugin's category hook (Product Plus
scopes, …), and combo ideas handed to Combos' own form. A job whose plugin isn't compiled in isn't offered. So its
battle tests compile in Product Plus, Alternatives and Pharmacy, and **their** auditor checks must pass after every
apply and undo too (PPF-1, ALT-1, ALT-5, Pharmacy's own once group 2 writes them).

## The words the rules use

- **Job** — a batch of suggestions for many products (names, categories, hints, custom fields, groups, expiry, packs,
  doses). **Apply** writes the ticked ones; **undo** puts them back.
- **Cleanup plan** — renames, merges and moves of categories, with one undo.
- **Stale** — the product changed since the suggestion was made.

## The grid

Columns: **Apply** · **Undo** · **Core** (money, stock — PLG-1) · **Merge** · **Faults**.

| Feature | Apply | Undo | Core | Merge | Faults |
|---|---|---|---|---|---|
| Job: names, categories, hints, fields, groups | AIC-2 | AIC-3 | AIC-1 | AIC-5 | PLG-4,5 |
| Job: expiry, packs, doses | AIC-2 | AIC-3 | AIC-1 | AIC-5 | PLG-4,5 |
| Another plugin's data (fields, marks, groups, scopes) | AIC-9 | AIC-9 | AIC-1 | — a | AIC-9 |
| Cleanup plan (categories) | AIC-4 | AIC-4 | AIC-1 | — a | PLG-4 |
| The AI's answer | AIC-6 | — a | — a | — a | AIC-6 |
| Combo ideas | AIC-7 | — b | — b | — a | — b |
| Settings, the key | AIC-8 | — a | — a | — a | PLG-5 |

a — not touched. b — nothing is saved.

## Rules

```
AIC-1  The AI Assistant never changes a price, stock or money: apply and undo touch names, categories,
       hints, custom fields, groups, expiry ticks, price-less packs and doses only.
       Checked by: example (snapshot), ledgercheck.

AIC-2  Apply writes exactly the suggestions ticked; a product changed since its suggestion is skipped and
       said so.
       Checked by: example.

AIC-3  Undo puts back exactly what apply changed and skips (and says) whatever was changed again since —
       never over a later edit.
       Checked by: example.

AIC-4  A cleanup plan leaves no product in a removed category; its category merges go through every
       plugin's category hook, and undo hands the notes back so the plugins put their rows back too.
       Checked by: example.

AIC-5  A product merge keeps the kept product's learnt item (old names joined) and moves job items
       (PLG-2).
       Checked by: example, ledgercheck.

AIC-6  The AI's answer is untrusted: a malformed answer changes nothing, and anything naming a product,
       category or value not asked about is dropped.
       Checked by: example.

AIC-7  Suggest combos sends the 200 products that sold most in the last 90 days, topped up with the newest
       when fewer sold; ideas need two or more listed products and never a prescription medicine; nothing
       is saved (user, 2026-11-21).
       Checked by: example.

AIC-8  The API key is never sent back to a screen; a backup marks it secret.
       Checked by: example.

AIC-9  What a job writes into another plugin goes through that plugin's own hook (attribute set, grouper,
       category hook), so that plugin's rules hold after apply and after undo (Product Plus values fit their
       fields, PPF-1; a product sits in one tier and hand splits add up, ALT-1, ALT-5). A job for a plugin that
       isn't compiled in isn't offered, and a run without it changes nothing of it.
       Checked by: example (with those plugins compiled in), their ledgercheck.
```
