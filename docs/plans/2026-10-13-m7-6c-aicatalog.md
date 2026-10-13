# M7.6c AI Assistant (aicatalog) — implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: use superpowers:executing-plans. Steps use checkbox (`- [ ]`) syntax.
> Same shape as M7.6b. Opus executes and reviews, one commit per step, and UI gates are self-reviewed (4 combos, 1366 + 360).

**Goal:** the AI is a helper inside the screens the shop already uses — ✨ Identify under the Name field of the
product form and intake's new item, and a Clean up page (categories, uncategorized, names, duplicates → core merge)
— with paste mode (copy the prompt to a chatbot, paste the reply) as the main path and the API as an option.

**Architecture:** a new plugin `plugins/aicatalog` forks karots-pos's client, prompts, parsers and `ApplyPlan` /
`Revert`; every AI action has one shape (send → result, or `{prompt, error?}`; send again with `reply` → parsed). Two
generic core seams: `DataGroup.Secret` (blanked in every dump) and the React slot `productAssists` (under Name).
The duplicate rows open core's existing merge confirm (exported from `MergeInto.tsx`).

**Spec:** `docs/specs/2026-10-13-m7-6c-aicatalog-design.md` (read it — §2 A–H, §3 API, §4 data). karots-pos source:
`~/Projects/go/karots-pos/plugins/aicatalog/` (`ai.go`, `optimize.go`, `admin_optimize.go`, `store.go`, `migrations/`).

## Global constraints

- Core never names a plugin; `make check-core` after every task. Plugin lists committed core-only (`make core-only`).
- Plugin key `aicatalog`, goose table `goose_db_version_aicatalog`, display name **AI Assistant**.
- karots-pos `0001_init.sql`–`0007_naming_style.sql` copied **byte for byte**, same names; suite changes only in `0008_suite.sql`.
- The API key never appears in any JSON response (`hasKey` only) and is `''` in every dump. Blank key on PUT = keep.
- The AI suggests, a person confirms: nothing is written without a click. No till surface.
- Paste is the default mode; every AI action falls back to the prompt + reply box when the API call fails.
- AI HTTP timeout 45 s. Clean-up chunks of 150 products.
- Theme tokens only, admin works at 360px, 4 combos. Files ≤ 500 lines. No AI attribution in commits.

## Review focus

1. A reply wrapped in prose and a ```json fence parses; a reply with no JSON keeps the pasted text and says why — Tasks 3, 7.
2. A plan naming unknown ids, a self-merge, a reparent cycle or a product rename to an existing product's name is
   skipped, never fails the transaction; undo restores merged categories with their old ids — Task 5.
3. A restored karots-pos shop at 0007 keeps its own `manual_mode`, and its key comes in blank (dump AND restore) — Tasks 1, 2.
4. Picking an option whose category path the shop lacks creates it once (`/` → `>`), and picking again reuses it — Task 4.
5. Merging a product that has an `aicatalog_items` row into one that also has one doesn't violate the PK — Task 4.

---

### Task 1: core — `DataGroup.Secret` blanked in every dump
- [x] `internal/plugin/hooks.go`: `DataGroup.Secret []string` ("table.column"; doc: written as `''` in every dump).
- [x] `internal/backup/backup.go` `Dump`: after `dumpRows`, blank every column named by any `plugin.CollectDataGroups()`
  group's `Secret` for that table (a `blankColumns(cols, rows, names)` beside `zeroColumns`, value `""` not NULL). One
  place, so the download, the scheduler and the before-restore snapshot all blank it.
- [x] Test (`backup/secret_db_test.go`, DATABASE_URL-gated): a temp table + a registered group with `Secret` → the
  dump's row has `""`, other columns intact. Commit `feat(backup): a plugin can mark columns secret — every dump writes them blank`

### Task 2: plugin skeleton, migrations, store, restore
- [x] `plugins/aicatalog/plugin.json` (key, name "AI Assistant", import, version 2.0.0, description), `aicatalog.go`
  (`init` → `plugin.Register`, `Key()`, `Migrations()`), `migrations/` = karots-pos 0001–0007 copied + `embed.go`.
- [x] `0008_suite.sql` (Up/Down): `DROP TABLE aicatalog_mock_products`; `ALTER TABLE aicatalog_settings DROP COLUMN
  default_markup`; `ALTER COLUMN manual_mode SET DEFAULT true`; `UPDATE aicatalog_settings SET manual_mode = true WHERE api_key = ''`.
- [x] `store.go`: `Settings` (no markup), `GetSettings` / `SaveSettings` (blank key keeps the stored one), `LearnItem`,
  `GetItem`, `CategoryPaths` (forked, cap 200).
- [x] `Setup`: DataGroup `{Key:"aicatalog", Name:"AI Assistant", Tables: items, optimize_runs, settings, Secret:
  ["aicatalog_settings.api_key"]}`.
- [x] `restore_db_test.go` (clearance pattern): a karots-pos 0007 DB with a mock draft + committed draft, a run, an item
  and a key (`manual_mode=false`) → dump → restore into a suite DB at 0008: item + run + settings across,
  `manual_mode` still false, mock table in `SkippedTables`, `api_key = ''` (focus 3). Fresh migrate → `manual_mode` true.
- [x] `make plugins P=all` → `go test ./plugins/aicatalog/...` → `make core-only`. Commit
  `feat(aicatalog): plugin skeleton — karots-pos migrations byte for byte, paste mode by default, key kept out of backups`

### Task 3: AI client, prompts, parsers
- [x] `ai.go`: fork `Client` (timeout **45 s**), `call` / `callGemini` / `complete` / `ListModels` / `Ping`,
  `apiErrMessage`, `withCustom`, `withNaming`, `searchEnabled`, `isLocal`. Drop Insights / enrich / `Ask`.
- [x] `prompts.go`: `systemPrompt` (forked) + "Reply with JSON only."; `IdentifyPrompt(q, hint, cats, custom, naming)`;
  `IdentifyFollowUp(q, hint) = "Next item: <q> — same rules, JSON only."` (+ hint line). `cleanupSystemPrompt` = karots-pos
  optimize prompt extended with `"uncategorized":[{"product_id","category":"A > B","reason"}]`, `"names":[{"product_id","to","reason"}]`
  (only when a naming style is set — the prompt says so and carries it), `"duplicates":[{"a","b","reason"}]`;
  `CleanupPrompt(cats, prods, uncatIDs, naming, custom)`. PRODUCTS rows mark uncategorized ones (`|U`).
- [x] `parse.go`: `parseIdentify`, `parseCleanup` (outermost `{…}`, tolerant of fences / prose); error text for the UI:
  "Couldn't find an answer in that — paste the whole reply (it should contain JSON starting with {)." Normalise category
  paths (`/` → `>`, trim).
- [x] `ai_test.go`: parsers — fenced, prose-wrapped, bad JSON, empty; `httptest` fakes — OpenAI-compat answer, Gemini
  native answer (multi-part concatenated), `{"error":{"message"}}` and Gemini array error surfaced, no key + non-local → error.
  Commit `feat(aicatalog): AI client, identify and clean-up prompts, tolerant parsers`

### Task 4: API — settings, identify, category; learn on save; ⓘ row; merger
- [x] `api.go` on `reg.Admin()`: `GET/PUT /aicatalog/settings` (`hasKey`, never the key), `POST /aicatalog/settings/models`,
  `POST /aicatalog/settings/test` (models/test take the typed provider, base URL, model and key — blank key = stored),
  `POST /aicatalog/identify {query, barcode, hint, reply, short}` → `{result}` | `{prompt, error?}`: `reply` set → parse;
  paste mode or API error → prompt (`short` → `IdentifyFollowUp`); `POST /aicatalog/category {path}` → `{id}` via
  `categories.FindOrCreateByPath` on the normalised path.
- [x] `ProductExtras{Key:"aicatalog"}`: Validate = valid JSON `{name, category, specs, explanation, source, query}` with
  source ∈ `ai_confident|user_picked|user_explained`; Save → `LearnItem`.
- [x] `ProductDetailContributor`: "AI identified" · `<name> — <specs>` (specs only when non-empty).
- [x] `ProductMerger`: if `into` has a row delete `from`'s, else move it (focus 5) + `merge_db_test.go`.
- [x] `api_test.go` (DATABASE_URL-gated, plugintest): settings round trip never echoes the key; paste mode returns a
  prompt; reply parsed; API failure (fake 500) → prompt + error; category path created once then reused (focus 4); extras
  saved with a product → ⓘ row. Commit `feat(aicatalog): identify, settings and category routes; a pick is learnt on save and shown in ⓘ`

### Task 5: clean-up — plan, apply, undo, empty categories
- [ ] `cleanup.go` (store): `CleanupCategories`, `CleanupProducts(limit, offset)` (active, non-service, with `uncat` =
  no category or one named "Uncategorized"), `CountCleanupProducts`, `SalesCounts(ids)`.
- [ ] `apply.go`: fork `ApplyPlan` / `isDescendant` / `Revert` / `revertOp` / `LatestRun` and extend: uncategorized →
  `FindOrCreateByPath` inside the tx (undo `created_category` → delete when empty) + `move_product`; names →
  `rename_product` (skip when blank, same, or another active product already has that name — checked with a SELECT, never
  a failing UPDATE); duplicates never applied. Summary counts all six kinds. Chunks thread `runID`.
- [ ] Duplicates in the preview: server drops pairs with unknown / same / retired ids and orders each as `{keep, drop}`
  by sales count (ties: lower id kept).
- [ ] Routes: `POST /aicatalog/cleanup {offset, reply}` → `{plan, chunk, chunks, offset, next}` | `{prompt, error?}`;
  `POST /aicatalog/cleanup/apply {plan, runId}`; `GET /aicatalog/cleanup/last`, `POST /aicatalog/cleanup/undo`;
  `GET/POST /aicatalog/cleanup/empty-categories` (no products, no children; POST deletes the listed ids that are still empty).
- [ ] `apply_db_test.go`: each skip rule (focus 2), rename + merge + move + uncategorized-new-path + product rename →
  undo restores ids, names, parents and placements and deletes the created category; two chunks revert as one; empty
  categories list / delete leaves a non-empty one. Commit `feat(aicatalog): catalogue clean-up — categories, uncategorized, names, duplicates; one undo`

### Task 6: core web seams — `productAssists`, exported merge confirm
- [ ] `slots.ts`: `ProductAssist = { key; Assist: ComponentType<{ name: string; barcode: string; where: 'form'|'intake';
  fill: (f: { name?: string; categoryId?: number; extra?: unknown }) => void }> }`, `productAssists: []`.
- [ ] `ProductForm.tsx` and intake `NewItem`: render each Assist under the Name input; `fill` sets name / category and
  `extras[key] = extra`. Core-only build renders nothing.
- [ ] `MergeInto.tsx`: `export function MergeConfirm`. `bunx tsc -b`, `make test-web`, `make check-core`. Commit
  `feat(web): productAssists slot under the product name; merge confirm can be opened by a caller`

### Task 7: ✨ Identify (UI gate)
- [ ] `web/src/plugins/aicatalog/ask.ts`: pure reducer `idle → asking → result | awaiting(prompt) → result | error(text kept)`
  + `readReply` (calls the endpoint with `reply`) — `ask.test.ts` (Vitest): paste mode, bad paste keeps text, API fail → awaiting.
- [ ] `PastePanel.tsx` (shared): **Copy & open chatbot** (clipboard write + `window.open(url, 'ai-chatbot')`, chatbot from
  settings: Gemini / ChatGPT / Claude), reply box parses on paste (no button), **reads the clipboard when the tab regains
  focus** while awaiting (`navigator.clipboard.readText`, failures silent), **Full prompt** button.
- [ ] `Identify.tsx` (the Assist): ✨ Identify (disabled with no name / barcode); one sure answer or up to 4 options
  (name, category, specs, explanation) + "None of these — let me explain" (hint → ask again, source `user_explained`);
  pick → `POST /category` → `fill({name, categoryId, extra})`, invalidate the categories query. First prompt per tab full,
  later `short` (sessionStorage `ks-aicat-primed`).
- [ ] `index.tsx`: register `productAssists`. Shots: form + intake, paste state and options, 4 combos at 1366, form at 360.
  Commit `feat(aicatalog): ✨ Identify in the product form and intake — paste first, reads the reply as it lands`

### Task 8: Apps → AI Assistant — Settings and Clean up (UI gate)
- [ ] App tile + `adminPages` (`/admin/x/aicatalog`, tabs **Clean up** · **Settings**).
- [ ] `SettingsPage.tsx`: Paste mode / API switch first; chatbot choice (paste); API: provider preset (Gemini, Groq,
  OpenRouter, OpenAI, Ollama, LM Studio, llama.cpp → base URL + model), Load models, Test AI, key (placeholder
  "saved — leave blank to keep"), web search (Gemini only); Naming style; Custom instructions; Chrome Gemini side-panel tip.
- [ ] `CleanupPage.tsx`: Start → PastePanel or plan; four ticked lists (Categories, Uncategorized, Names, Duplicates)
  with reasons; Apply ticked (duplicates excluded) → summary, next chunk; **Undo last clean-up** (shows the summary);
  duplicate row **Merge…** → `MergeConfirm`; **Delete empty categories** (list + one confirm).
- [ ] Shots: both pages, 4 combos, 360px (card edges). Commit `feat(aicatalog): AI Assistant settings and clean-up pages`

### Task 9: e2e, parity, docs
- [ ] `web/e2e/plugin-aicatalog.spec.ts` (import `test` from `./fixtures`; paste mode): identify in the product form with
  a fenced + prose reply → name + new category filled, save, ⓘ row; identify from Items to review and intake; bad paste
  keeps text + message; clean-up with one change unticked → apply → undo; duplicate Merge… opens the confirm; clipboard
  read on return (`clipboard-read` granted). **API mode:** a Node `http` fake on a free port (`/chat/completions`, OpenAI
  shape; base URL localhost so no key) → identify answers directly; fake returns 500 → same panel offers the prompt.
  360px: settings + clean-up.
- [ ] `docs/parity.md`: 28 aicatalog rows ticked or marked dropped by design (spec §5). `docs/differences.md`: new rows.
- [ ] Full checks: `make verify`, `go test ./...` core-only and with every plugin (`-p 1`), `make test-web`, full
  Playwright, `make verify-portable`. `docs/status.md` + memory updated. Commit `docs: M7.6c done — AI Assistant`
