# Promo Videos Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Product search matches a product's local name, then 27 vertical promo videos (1 main + 8 shorts × Sinhala /
Tamil / English) rendered from data with Remotion.

**Architecture:** Search: `products.name_local` joins every product `LIKE`/`ILIKE` search (core first, then plugins).
Videos: a standalone `videos/` bun project. Content is data (`src/videos.ts` scene lists + `src/lines/<lang>.ts`
captions and voice lines); `edge-tts` turns each line into an mp3 and a `durations.json`; Playwright records short
screen clips from the Demo Room's Super Store; Remotion composes scene → video; one command renders all 27.

**Tech Stack:** Go + Postgres (search); bun, Remotion 4.x (`remotion`, `@remotion/cli`, `@remotion/media-utils`),
React, TypeScript strict, `@fontsource/noto-sans-sinhala` + `@fontsource/noto-sans-tamil`, `edge-tts` (via `uvx`),
Playwright (already in `web/`), ffmpeg/ffprobe.

**Spec:** `docs/specs/2026-10-28-promo-videos-design.md`

## Global Constraints

- Vertical 1080×1920, 30 fps, H.264 + AAC. Main promo 60–90 s, shorts 15–20 s (a few seconds over is fine).
- Never tech words on screen or in the voice ("plugin", "cloud server", "database", "AI", "Demo Room"). "Add-on" is fine.
- Every claim matches a real screen. Cloud backup copy and every add-on carry an "optional add-on" tag.
- End card on every video: logo `docs/images/logo.jpg`, "Karots Suite", phone mockup "Coming soon: Pocket Cashier" —
  *Power cut? PC broken? Keep selling from your phone.* — "WhatsApp us for a free demo · 076 962 6396" + WhatsApp mark.
- Short 1 promises watching "from anywhere" with the small caption "needs internet at the shop".
- No prices. Screens stay English in all languages.
- `videos/` never imported by `web/` or Go, never in the binary; `videos/out/`, `videos/node_modules/`,
  `videos/public/clips/`, `videos/public/voice/` git-ignored.
- Files ≤ 500 lines. No AI attribution in commits. Core stays plugin-inert.

## Review Focus

- A Sinhala / Tamil caption overflows or breaks mid-glyph at 1080 px wide → captions wrap inside a safe area
  (top 12 % and bottom 18 % kept clear for TikTok / Reels UI). Task 7 renders a still of the longest caption per language.
- A voice line is missing or its mp3 failed → render fails loudly, never a silent scene. Task 5 test.
- Local-name search with mixed scripts ("samba සහල්") → every token must match name OR local name (per token, not
  per field). Task 1 test.
- A product with no local name (NULL) still found by its English name. Task 1 test.
- A recording that lands on a loading state / empty list → clip trimmed to start after `networkidle`; Task 6 self-review.

---

### Task 1: Core product search matches the local name

**Files:**
- Modify: `internal/features/products/search.go` (`searchClause`)
- Modify: `internal/features/warranty/warranty.go:177`, `internal/features/conversions/conversions.go:87-88,134-135`
- Test: `internal/features/products/search_local_db_test.go` (create)
- Docs: the till / stock guide page that describes search (`docs/guide/`) — one line: "Search finds a product by its
  local name too (Sinhala / Tamil)."

**Interfaces:** Produces: no API change; `ListQuery{Search}` now matches `name_local`.

- [ ] **Step 1: Write the failing DB test** (gated on `DATABASE_URL` like `barcodes_db_test.go`; reuse its fixture style)

`TestSearchFindsLocalName`: create product `Name: "TEST-L Samba Rice 1kg"`, `NameLocal: "සම්බා සහල්"`, and
`"TEST-L Plain"` with no local name. Assert via `NewRepository(conn).List(ctx, ListQuery{Search: q, Limit: 10, Page: 1})`:
- `"සම්බා"` → contains Samba
- `"test-l සහල්"` (mixed: one token per field) → contains Samba
- `"test-l plain"` → contains Plain (NULL local name doesn't drop it)
- `"සම්බා plain"` → contains neither

- [ ] **Step 2:** `go test ./internal/features/products -run TestSearchFindsLocalName` → FAIL (no rows for `සම්බා`).

- [ ] **Step 3:** In `searchClause`, each token's `NOT (...)` gets `OR lower(coalesce(p.name_local, '')) LIKE '%' || tok || '%'`.
  No fuzzy (`word_similarity`) on the local name. Add a comment line:
  `// ponytail: local name is an unindexed LIKE; add a trigram index if a big catalogue gets slow.`
  Warranty and conversions: add `OR p.name_local ILIKE '%' || $1 || '%'` (`fp.`/`tp.` in conversions).

- [ ] **Step 4:** test passes; `make test` green.

- [ ] **Step 5:** guide line; commit `feat(search): products are found by their local name too`.

### Task 2: Plugin product searches match the local name

**Files:**
- Modify: `plugins/productplus/grid.go:35`, `plugins/alternatives/store_members.go:96`, `plugins/pharmacy/store_scripts.go:343`
- Test: add one case to `plugins/pharmacy/store_db_test.go` (the script product picker finds a product by `name_local`)

- [ ] **Step 1:** failing test `TestScriptPickerFindsLocalName` in pharmacy (a medicine with `name_local` "පැරසිටමෝල්", search "පැරසි" → found).
- [ ] **Step 2:** run → FAIL.
- [ ] **Step 3:** add `OR p.name_local ILIKE '%' || $N || '%'` beside each `p.name ILIKE` above.
- [ ] **Step 4:** `make plugins P=all && go test ./plugins/... && make core-only` → PASS.
- [ ] **Step 5:** commit `feat(plugins): product pickers find the local name too`.

### Task 3: `videos/` project scaffold

**Files:**
- Create: `videos/package.json`, `videos/tsconfig.json`, `videos/remotion.config.ts`, `videos/src/index.ts`
  (`registerRoot`), `videos/src/Root.tsx`, `videos/src/theme.ts`, `videos/src/fonts.ts`, `videos/README.md`
- Modify: `.gitignore` (the four `videos/` paths above), `AGENTS.md` Layout (one line: `videos/ promo videos
  (Remotion, own package.json; make videos) — never shipped`), `Makefile` (`videos:` → `cd videos && bun run render`)
- Before starting: check `remotion.dev/docs/ai/claude-code-plugin`; if the user installed the Remotion skill, load it.

**Interfaces:** Produces `theme.ts`: `export const t = { bg, surface, ink, muted, accent, accentInk, radius }` (values
copied from `web/src/theme/palettes/dock/studio.css` light), `fonts.ts`: `fontFor(lang: Lang): string` (Noto Sans
Sinhala / Noto Sans Tamil / the suite's sans) — fonts loaded from the `@fontsource` packages, never the network.

- [ ] **Step 1:** `bun init` in `videos/`, add `remotion @remotion/cli @remotion/media-utils react react-dom
  @fontsource/noto-sans-sinhala @fontsource/noto-sans-tamil` (exact versions pinned), a placeholder composition
  `Hello` 1080×1920 30 fps.
- [ ] **Step 2:** `cd videos && bunx remotion still Hello out/hello.png` → a PNG 1080×1920 exists.
- [ ] **Step 3:** commit `feat(videos): Remotion project`.

### Task 4: Content as data — scenes and lines in three languages

**Files:**
- Create: `videos/src/types.ts`, `videos/src/videos.ts`, `videos/src/lines/en.ts`, `si.ts`, `ta.ts`,
  `videos/src/content.test.ts`

**Interfaces:** Produces
```ts
export type Lang = 'en' | 'si' | 'ta'
export type Focus = { x: number; y: number; zoom: number }        // 0..1 centre, zoom ≥ 1
export type Visual =
  | { kind: 'clip'; src: string; focus?: Focus; from?: number }   // public/clips/<src>.webm, from = seconds
  | { kind: 'shot'; src: string; focus?: Focus }                  // a docs/images path, copied to public/shots
  | { kind: 'problem' }                                           // big caption on the palette, no screen
export type Scene = { id: string; visual: Visual; addon?: boolean; note?: 'internet' }
export type Video = { id: VideoId; scenes: Scene[] }              // the end card is added by the composition
export type VideoId = 'promo' | 'away' | 'buying' | 'credit' | 'staff' | 'suppliers' | 'profit' | 'safe' | 'addons'
export type Line = { caption: string; voice: string }             // caption ≤ 8 words; voice ≤ ~2 short sentences
export type Lines = Record<string, Line>                          // scene id → line; plus 'end' and 'end-soon'
```
`videos.ts` exports `VIDEOS: Video[]`. Shorts: `<id>-problem` (kind problem) → 1–2 proof scenes → end. Promo: hook,
the proof scenes of away / buying / credit / money / staff, then montage scenes `m-suppliers m-profit m-requests
m-similar m-search m-hold m-labels m-receipts m-offline m-backup m-addons` (~2 s each), then end. Short 8 shows
pharmacy, repairs + reload, print shop, clearance, combos (each `addon: true`). `m-backup`, `safe-cloud`, and all
add-on scenes: `addon: true`. Short `away`: `note: 'internet'`.

Copy rules: everyday words, the owner's problem first ("Still writing credit in ten books?"), then the relief. English
written first; Sinhala and Tamil are natural shop-talk drafts (not word-for-word), flagged in `videos/README.md` as
**"to be checked by a fluent speaker before posting"**.

- [ ] **Step 1:** `content.test.ts` (bun test): for every lang × every scene id in `VIDEOS` plus `end`, `end-soon`,
  `addon-tag`, `internet-note` — a non-empty caption and voice; caption ≤ 8 words in `en`; every `clip` / `shot` src
  listed in `CLIPS` (Task 6) or existing under `docs/images`.
- [ ] **Step 2:** `bun test` → FAIL (files empty).
- [ ] **Step 3:** write `videos.ts` + the three line files.
- [ ] **Step 4:** `bun test` → PASS.
- [ ] **Step 5:** commit `feat(videos): scenes and lines in Sinhala, Tamil and English`.

### Task 5: Voices — one mp3 per line + durations

**Files:**
- Create: `videos/scripts/voice.ts`, `videos/src/voices.ts`, `videos/scripts/voice.test.ts`
- Generated (ignored): `videos/public/voice/<lang>/<sceneId>.mp3`, `videos/src/durations.json` (committed — small, keeps
  renders reproducible without re-voicing)

**Interfaces:** `voices.ts`: `export const VOICE: Record<Lang, string> = { si: 'si-LK-ThiliniNeural', ta:
'ta-LK-SaranyaNeural', en: 'en-IN-NeerjaNeural' }` (user may swap; samples below). `durations.json`:
`Record<Lang, Record<sceneId, seconds>>` (ffprobe). `bun run voice [--lang si]` regenerates only lines whose text
changed (hash in `public/voice/<lang>/.hashes.json`).

- [ ] **Step 1:** `voice.test.ts`: after `bun run voice`, every lang × line id has an mp3 > 1 KB and a duration > 0.3 s.
- [ ] **Step 2:** run → FAIL.
- [ ] **Step 3:** `voice.ts` shells `uvx edge-tts --voice <v> --text <voice> --write-media <file>` per line, then
  `ffprobe -show_entries format=duration`. Exit non-zero on any failure.
- [ ] **Step 4:** PASS. Also write `videos/out/samples/<voice>.mp3` — one sample line per language in both voices
  (`si-LK-SameeraNeural/ThiliniNeural`, `ta-LK-KumarNeural/SaranyaNeural`, `en-IN-PrabhatNeural/NeerjaNeural`) and tell
  the user where they are; keep going with the defaults.
- [ ] **Step 5:** commit `feat(videos): computer voice per line`.

### Task 6: Screen clips from the Demo Room

**Files:**
- Create: `web/docs-shots/clips.spec.ts` (reuses the `unlock` / `session` / `hideChrome` helpers — move them to
  `web/docs-shots/demo.ts` and import from both specs), `videos/src/clips.ts` (`export const CLIPS: string[]`)
- Modify: `web/playwright.docs.config.ts` if needed so `clips.spec.ts` runs only with `DOCS_CLIPS=1`; `Makefile`
  `videos-clips:` (same `DOCS_CODE` as `docs-shots`)
- Output: `videos/public/clips/<name>.webm` (`recordVideo`, ignored)

Clips (≤ 8 s each, phone 390×844 for admin, desk 1440×900 for the till): `away-dashboard` (owner, phone: dashboard
→ Who's on now), `away-sales` (phone, sales list scroll), `till-close` (till close with the short warning),
`buying-lowstock` (till Low stock tab), `buying-reorder` (reorder by supplier), `credit-list` (customers with
balances), `credit-statement` (a statement sheet), `money-cashflow` (phone), `staff-activity` (Activity: returns /
discounts by user), `staff-lock` (till auto-lock screen), `suppliers-dues`, `profit-report`, `requests`,
`similar` (alternatives on an out-of-stock item), `search-local` (till search typing `සම්බා` → Samba Rice shows),
`hold-bill`, `labels`, `receipt`, `backup-data` (Setup → Data copies), plus add-on clips `pharmacy`, `repairs`,
`recharge`, `documents`, `clearance`, `combos`. A screen with an existing good still may use the `shot` instead.

- [ ] **Step 1:** start the Demo Room (`make demo` binary), `make videos-clips` → every name in `CLIPS` exists as a webm.
- [ ] **Step 2:** self-review: grab a frame of each (`ffmpeg -ss 1 -frames:v 1`) and look at every one — no loading
  spinners, no empty lists, no demo chrome, no install id / PIN visible. Re-record failures.
- [ ] **Step 3:** `make docs-shots` still works after the helper move (`DOCS_ONLY=dashboard`).
- [ ] **Step 4:** commit `feat(videos): Demo Room screen clips`.

### Task 7: Compositions — scene, end card, video

**Files:**
- Create: `videos/src/Scene.tsx` (visual with slow zoom/pan to `focus`, caption block, add-on tag, internet note),
  `videos/src/Problem.tsx`, `videos/src/EndCard.tsx` (logo, wordmark, drawn phone mockup with a fake till screen
  inside "Coming soon: Pocket Cashier", WhatsApp line; the WhatsApp mark as inline SVG), `videos/src/Video.tsx`
  (sequences scenes; scene length = voice duration + 0.4 s, min 1.8 s; music under it), `videos/src/Root.tsx`
  (27 compositions `<video>-<lang>`, duration via `calculateMetadata` from `durations.json`)
- Music: `videos/public/music.mp3` from Pixabay Music (upbeat, free licence) + `videos/MUSIC.md` (title, author, URL,
  licence). Volume 0.12 under voice, 0.35 on the end card. If the download is blocked, ask the user to drop a file there.

- [ ] **Step 1:** `bunx remotion still` on: `credit-si` frame 30, `promo-ta` a montage frame, `addons-en` a frame
  showing the add-on tag, `away-si` showing the internet note, every video's end card, and the longest caption per
  language → PNGs in `videos/out/stills/`.
- [ ] **Step 2:** self-review every still (Read the PNG): text inside the safe area, Sinhala / Tamil glyphs whole, Studio
  palette, readable from arm's length, end card complete. Fix and re-render until all pass.
- [ ] **Step 3:** `bunx remotion render credit-en out/test.mp4`; watch a few frames; `ffprobe` → 1080×1920, h264, aac, 15–25 s.
- [ ] **Step 4:** commit `feat(videos): scene, end card and video compositions`.

### Task 8: Render all 27 + handoff

**Files:**
- Create: `videos/scripts/render.ts` (`bun run render [--lang si] [--video credit]`): renders to
  `videos/out/<lang>/karots-suite-<video>-<lang>.mp4`, then ffprobes each and prints a table (file, seconds, size);
  exits non-zero if a short is > 25 s or the promo is outside 55–100 s or the size isn't 1080×1920.
- Modify: `videos/README.md` (how to change a line, re-voice, re-record, render; translations need a fluent check;
  music licence), `docs/status.md` (videos done, where the files are, what waits on the user), delete this plan.

- [ ] **Step 1:** `make videos` → 27 files, the table prints, exit 0.
- [ ] **Step 2:** self-review: extract 3 frames from each of `promo-si`, `promo-ta`, `promo-en`, `staff-si` and look at them.
- [ ] **Step 3:** commit `feat(videos): render all promo videos`; tell the user: where the MP4s are, the voice
  samples to pick from, and that Sinhala / Tamil text needs a fluent check before posting.
