# M7.6c — AI Assistant (aicatalog) (design)

Built **after** M7.6a (extra barcodes, Merge products — `2026-10-13-m7-6a-barcodes-merge-design.md`) and M7.6b
(lots in stock take / intake). The duplicate finder (H) opens M7.6a's merge.

The user approved the rethink on 2026-10-13: **A–C and E–H as recommended** ("do whatever you recommended"); **D (Insights) dropped** — "remove that as well", the plugin is catalogue help for whoever sets the shop up, not a shop-owner feature.
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

Rule carried over: **the AI suggests, a person confirms.** Nothing the AI says is written without a click.

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
- **C. Catalogue clean-up** (user, 2026-10-13: "optimize needs more attention"). Apps → AI Assistant →
  **Clean up**, four ticked lists from one AI pass (one prompt; chunks of 150 products when the shop is big):
  1. **Categories** — merges, re-parents, renames (karots-pos plan JSON), and product moves to a better
     existing category.
  2. **Uncategorized** — a suggested category (existing path, or a new one created on apply) for every product
     in Uncategorized / no category.
  3. **Names** — product renames that follow the Naming style setting (only sent when one is set).
  4. **Duplicates** — see H.
  - Each change ticked by default with its reason; untick to skip. **Apply ticked** → one transaction, invalid
    ops skipped (unknown ids, cycles, self-merge — karots-pos `ApplyPlan` rules), undo ops stored (renames
    store the old name).
  - **Undo last clean-up** replays them (re-creates merged categories with their old ids, restores placements
    and names). One level; a chunked session reverts as one (karots-pos behaviour). Product merges (H) are not
    part of it — they're core merges, confirmed one by one.
  - **Delete empty categories** button (no AI): categories with no products and no children, listed, one
    confirm.
- **D. No Insights.** The AI Insights page and its Reports card are not carried over.
- **E. Paste mode first (user, 2026-10-13: "i used aicatalog most of the time with prompt copy paste mode").**
  - **Paste is the default** for a new shop; the API is the optional extra. Every AI action's paste flow is
    the main path, not a fallback screen: one **Copy & open Gemini / ChatGPT / Claude** button (copies the
    prompt, opens or reuses one `ai-chatbot` tab), then a reply box that **reads the reply as soon as it is
    pasted** (no extra click; a bad paste says what's wrong and keeps the text).
  - Prompts carry an "answer with JSON only" line and the parser tolerates the chatbot's prose and code fences
    around it (karots-pos parsers), so a plain copy of the whole answer works.
  - Inside the product form the paste step stays in the same small panel under Name — no page change, the form
    keeps what was typed.
  - **Reads the clipboard on return** (user chose, 2026-10-13): while a reply is awaited, coming back to the tab
    reads the clipboard (`navigator.clipboard.readText`, asked once) and fills the reply box when it holds a
    parsable answer — so the round trip is Copy → paste + Enter in Gemini → Gemini's copy button → back. Needs a
    secure page (`localhost` or https); over plain LAN http the browser refuses and the person pastes (Ctrl+V).
  - **Short follow-ups:** the first prompt of a session carries the full rules, categories and naming style;
    later identify prompts are short ("Next item: 6201 ZZ — same rules, JSON only") since the chat remembers.
    **Full prompt** button for a fresh chat. Session = the browser tab's lifetime.
  - Settings page tip: **Chrome's Gemini side panel** keeps the chatbot beside the page (no tab switching).
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
- **H. Duplicates.** The clean-up also returns look-alike product pairs ("Coke 1L" / "Coca Cola 1 Ltr") with
  a reason and which one to keep (the one with more sales). Each row's **Merge…** opens the core merge confirm
  (M7.6a) with both filled — the person checks the counts and confirms, one pair at a time. Never automatic.

Not on the till: the plugin has no till surface and can't stop a sale.

## 3. API (`/api/v1/x/aicatalog`, admin)

| Method | Path | What |
|---|---|---|
| GET / PUT | `/settings` | settings (`hasKey`, never the key) |
| POST | `/settings/models` | live model list for the typed provider / key |
| POST | `/settings/test` | one tiny request |
| POST | `/identify` | `{query, barcode, hint, reply?}` → `{result}` or `{prompt, error?}` |
| POST | `/category` | `{path}` → `{id}` (find or create) |
| POST | `/cleanup` | `{offset, reply?}` → `{plan, chunk, chunks}` or `{prompt, error?}` (plan = categories, uncategorized, names, duplicates) |
| POST | `/cleanup/apply` | `{plan (ticked only, no duplicates), runId}` → `{summary, runId}` |
| GET / POST | `/cleanup/empty-categories` | list / delete empty categories |
| GET / POST | `/cleanup/last`, `/cleanup/undo` | last un-reverted run; revert it |

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
- Optimize (preview, manual, apply, revert) → Clean up (C).
- Insights (page, sales, reorder, manual) → **dropped by design (M7.6, user: shop owners won't use AI)**.
- Settings, models, test-key → Settings (E). GET `/ai-catalog` → Apps → AI Assistant.

## 6. Checks

- Go: prompt builders, parsers (fenced, prose-wrapped, bad JSON), `ApplyPlan` + undo round trip (ids
  restored), secret column blanked in a dump, the AI client against an `httptest` fake (OpenAI-compat and
  Gemini shapes, error messages surfaced).
- Web unit: the ask → prompt → paste state machine.
- Playwright (`plugin-aicatalog.spec.ts`), **paste mode for every flow** (the main path): identify in the
  product form (paste a fenced reply with prose) fills name + new category and the ⓘ row shows after save;
  identify from Items to review and from Intake; a bad paste keeps the text and says why; clean-up with one change
  unticked, then undo. **One API-mode flow** against a fake provider the test server points at
  (no real key), plus API failure → the same panel offers the prompt. duplicate row opens the merge confirm; clipboard read on return (Playwright grants
  `clipboard-read`). 360 px for settings and clean-up.
