# M7.6 — AI Assistant (aicatalog) (design)

The user approved the rethink on 2026-10-13: **A–G as recommended, plus H** ("do whatever you recommended").
Plugin key stays `aicatalog` (old backups carry `goose_db_version_aicatalog` and `aicatalog_*`); the display
name is **AI Assistant**. No further AI uses and no MCP (user, 2026-10-13: most shops have no AI API and buyers
won't use it — photo bills, supplier messages, ask-your-shop, Sinhala/Tamil names were all dropped, not parked).

## 1. Why it changes

karots-pos grew aicatalog in five phases: a seven-step add-product stepper (identify → name & category →
barcode → price → qty → labels → save), a comma/line bulk queue, a staging table with batch enrich and
commit/revert, a category Optimize pass, and AI Insights. Only **identify** was new — barcode, markup, qty and
labels are core Intake, which the suite already has. **The user does not use bulk entry** (2026-10-13: "it
also do the same thing so i have to verify everything"), so the queue and the staging table go. In the suite
the AI is a **helper inside the screens the shop already uses**, never a parallel way to add products.

Rule carried over: **the AI suggests, a person confirms.** Nothing the AI says is written without a click, and
insights only narrate figures core already computed.

## 2. What it does

- **A. ✨ Identify inside core screens.** A small **✨ Identify** button under the Name field of core
  `ProductForm` (so also **Items to review**, which opens that form) and Intake's new-item panel.
  - Sends what is typed (name or code; barcode if any) plus the shop's existing category paths, so the AI
    reuses one instead of inventing a near-duplicate.
  - Shows **one sure answer** or **up to 4 options**, each with its plain-English explanation and specs, plus
    "None of these — let me explain" (a hint box that asks again).
  - Picking one **fills the name and category** in the form. A category path the shop lacks is created on pick
    (`categories.FindOrCreateByPath`, `/` normalised to `>`). The person still reviews and saves the form.
  - **Core seam (generic):** slot `productAssists: { key, Assist }`, rendered under the Name field in
    `ProductForm` and the intake new panel. `Assist` gets `{ name, barcode, where: 'form' | 'intake',
    fill(f: { name?: string; categoryId?: number; extra?: unknown }) }`. `extra` is stored as
    `ProductRequest.extra[key]` (existing `ProductExtras` path). Core never knows what fills it.
  - The pick is learnt on save: the plugin's `ProductExtras.Save` writes `aicatalog_items` (resolved name,
    category, specs, explanation, source `ai_confident | user_picked | user_explained`, raw query).
- **B. No bulk entry.** No queue, no staging table. Suite migration drops `aicatalog_mock_products`; an old
  backup still restores (restore already skips tables this schema lacks). Products a karots-pos commit created
  are ordinary products already.
- **C. Optimize categories, change by change.** Apps → AI Assistant → **Tidy categories**.
  - Sends the category tree (id | name | parent | count) and the active products (id | name | category) —
    one request when it fits, else chunks of 150 (same as karots-pos).
  - Plan = merges, re-parents, renames, product moves (same JSON as karots-pos). Shown as a list, **each change
    ticked by default with its reason; untick to skip**. Apply ticked → one transaction, invalid ops skipped
    (unknown ids, cycles, self-merge — karots-pos `ApplyPlan` rules).
  - **Undo last tidy** replays the stored undo ops (re-creates merged categories with their old ids, restores
    product placements). One level, a chunked session reverts as one (karots-pos behaviour).
- **D. Insights as a Reports card.** Reports → **Explain these numbers**: period picker (same presets as core
  reports), a Sales & profit panel and a Reorder panel. Each shows the figures from core
  (`reports` P&L and the reorder list) and **Explain** sends only those figures with the "use only these,
  invent nothing" guardrail. Plain text answer.
- **E. Paste mode first (user, 2026-10-13: "i used aicatalog most of the time with prompt copy paste mode").**
  - **Paste is the default** for a new shop; the API is the optional extra. Every AI action's paste flow is
    the main path, not a fallback screen: one **Copy & open Gemini / ChatGPT / Claude** button (copies the
    prompt, opens or reuses one `ai-chatbot` tab), then a reply box that **reads the reply as soon as it is
    pasted** (no extra click; a bad paste says what's wrong and keeps the text).
  - Prompts carry an "answer with JSON only" line and the parser tolerates the chatbot's prose and code fences
    around it (karots-pos parsers), so a plain copy of the whole answer works.
  - Inside the product form the paste step stays in the same small panel under Name — no page change, the form
    keeps what was typed.
- **Settings (Apps → AI Assistant → Settings).**
  - **Paste mode / API** switch at the top; the API fields show only for API.
  - Provider preset (Gemini, Groq, OpenRouter, OpenAI, Ollama, LM Studio, llama.cpp) fills base URL + model;
    **Load models**; **Test AI**; key typed once (never sent back to the browser, blank = keep).
  - **Web search** (Gemini only), **Naming style** (identify only), **Custom instructions** (every prompt).
  - **Default markup is dropped** — core price fields do cost ⇄ markup ⇄ selling.
  - **Paste fallback everywhere:** every AI action answers with either the result, or (paste mode, or the API
    call failed) the prompt to copy + a box to paste the reply into. The same request, never a dead end.
- **F. The key never leaves the server.** Generic backup seam: `DataGroup.Secret []string` ("table.column");
  every dump (download and automatic) writes those as `''`. After a restore the key is re-entered. The key is
  also never in any JSON response (`hasKey: bool` only).
- **G. ⓘ row.** `ProductDetailContributor`: "AI identified · <name> — <specs>" (specs only when known).
- **H. Possible duplicates.** Tidy categories also returns look-alike product pairs ("Coke 1L" / "Coca Cola 1
  Ltr") with a reason. Listed under the plan, **suggestion only**: each opens the product form to rename or
  disable. Nothing is merged automatically (sales history stays on each product).

Not on the till: the plugin has no till surface and can't stop a sale.

## 3. API (`/api/v1/x/aicatalog`, admin)

| Method | Path | What |
|---|---|---|
| GET / PUT | `/settings` | settings (`hasKey`, never the key) |
| POST | `/settings/models` | live model list for the typed provider / key |
| POST | `/settings/test` | one tiny request |
| POST | `/identify` | `{query, barcode, hint, reply?}` → `{result}` or `{prompt, error?}` |
| POST | `/category` | `{path}` → `{id}` (find or create) |
| POST | `/tidy` | `{offset, reply?}` → `{plan, chunk, chunks}` or `{prompt, error?}` |
| POST | `/tidy/apply` | `{plan (ticked only), runId}` → `{summary, runId}` |
| GET / POST | `/tidy/last`, `/tidy/undo` | last un-reverted run; revert it |
| POST | `/insights/:kind` | `kind = sales | reorder`, `{from, to, reply?}` → `{facts, text}` or `{prompt, facts}` |

One shape for every AI action: send without `reply` → the server answers (API mode) or hands back the prompt
(paste mode, or the call failed with `error`); send again with `reply` → the server parses it. Parsers are
karots-pos's (`parseIdentify`, `parseOptimizePlan`) — tolerant of code fences and prose. HTTP timeout 45 s
(grounded Gemini replies run long).

## 4. Data

- karots-pos `0001_init.sql`–`0007_naming_style.sql` byte for byte, same file names (goose versions must
  match an old backup's `goose_db_version_aicatalog`).
- Suite **`0008_suite.sql`**: `DROP TABLE aicatalog_mock_products`; `ALTER TABLE aicatalog_settings DROP
  COLUMN default_markup`; `manual_mode` defaults to true and a settings row with no key is switched to paste
  (a fresh install starts in paste mode; a restored shop keeps its own choice, as restore replaces the row).
  Duplicate pairs (H) are not stored.
- Data group: `aicatalog_items`, `aicatalog_optimize_runs`, `aicatalog_settings`; `Secret:
  ["aicatalog_settings.api_key"]`.
- `restore_db_test.go`: a karots-pos shop at 0007 with drafts, a committed draft, an optimize run, a learnt
  item and a key restores — items, run and settings come across, the mock table is skipped, the key is blank.

## 5. Parity (28 rows)

- Identify / pick / category / manual identify → ✨ Identify (A).
- Barcode / price / qty / save → **core Intake** (already ported; A adds identify to it).
- Mock row × 3, enrich, apply, commit, revert, clear → **dropped by design (M7.6, user: bulk unused)**.
- Optimize (preview, manual, apply, revert) → Tidy categories (C).
- Insights (page, sales, reorder, manual) → Reports card (D).
- Settings, models, test-key → Settings (E). GET `/ai-catalog` → Apps → AI Assistant.

## 6. Checks

- Go: prompt builders, parsers (fenced, prose-wrapped, bad JSON), `ApplyPlan` + undo round trip (ids
  restored), secret column blanked in a dump, the AI client against an `httptest` fake (OpenAI-compat and
  Gemini shapes, error messages surfaced).
- Web unit: the ask → prompt → paste state machine.
- Playwright (`plugin-aicatalog.spec.ts`), **paste mode for every flow** (the main path): identify in the
  product form (paste a fenced reply with prose) fills name + new category and the ⓘ row shows after save;
  identify from Items to review and from Intake; a bad paste keeps the text and says why; tidy with one change
  unticked, then undo; explain sales. **One API-mode flow** against a fake provider the test server points at
  (no real key), plus API failure → the same panel offers the prompt. 360 px for settings, tidy and explain.
