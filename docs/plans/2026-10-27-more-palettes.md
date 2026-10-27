# More Built-in Palettes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add 17 built-in Dock palettes, each with light and dark modes, its own feel and a signature touch in its
extra CSS, without changing the existing five.

**Architecture:** Each palette is one CSS file in `web/src/theme/palettes/dock/`, written in the existing format.
The extra part comes after the token blocks and is the same layer as the editor's Custom CSS. Each palette is
registered in the `PALETTES` list, in an `@import` in `app.css`, and in Go's `settings.Skins`. One data-driven Vitest
suite checks every new palette: it parses, sets every token in both modes, meets readability, survives Duplicate,
and keeps its motion safe. Batches then add palettes, each followed by a shot review.

**Tech Stack:** CSS (Tailwind v4 tokens), TypeScript, Vitest, Go, Playwright.

**Spec:** `docs/specs/2026-10-27-more-palettes-design.md` · mockups `docs/design/mockups/palettes-famous.html`,
`docs/design/mockups/palettes-colourful.html`

## Global Constraints

- **Keep the existing five unchanged:** `studio`, `night`, `bazaar`, `lagoon`, `catppuccin`. Byte-for-byte, no
  edits.
- **New ids, in grid order after the five:** `gruvbox, nord, dracula, tokyonight, rosepine, solarized, everforest,
  kanagawa, synthwave, monokai, shadesofpurple, cobalt2, ayu, mango, pop, aurora, neumorph`.
- **Names:** Gruvbox, Nord, Dracula, Tokyo Night, Rosé Pine, Solarized, Everforest, Kanagawa, Synthwave '84,
  Monokai, Shades of Purple, Cobalt2, Ayu, Mango, Pop, Aurora, Neumorph. `monokai` is the original free Monokai, not Monokai
  Pro.
- **Light variants:** use the official ones where they exist (Dracula → Alucard, Tokyo Night → Day, Rosé Pine →
  Dawn, Kanagawa → Lotus, Ayu → Light, and Gruvbox, Solarized and Everforest light). Make a matching light version for
  Nord, Synthwave '84, Monokai, Shades of Purple and Cobalt2.
- **Colours:** famous palettes use their published hex values. Keep each palette's accent and pass readability by
  choosing the text colour on it (dark text on a bright brand).
- **Readability:** every pair in `contrast.ts` `PAIRS` passes in both modes, with see-through layers composited
  over `--background` first. That includes button text at 4.5. Status colours stay success = green family,
  danger = red family, warning = amber family.
- **Signature touches** (spec table) live in the file's extra part only. They never sit on prices, the cart, tables
  or PIN entry. Synthwave's glow goes on headings and the hero total only.
- **Per-mode signature values** go in custom `--vars` inside each mode's token block. The extra part must have no
  `@media (prefers-color-scheme…)` block, because `parsePalette` drops it.
- **Motion:** `--dur*` ≤ 350 ms. An infinite animation is allowed only on background art, only with a loop of
  ≥ 20 s, only animating `transform` / `opacity`, and it needs a `@media (prefers-reduced-motion: reduce)` rule that
  stops it.
- **The `background` shorthand gotcha:** never fold a token into the `background` shorthand. Use the longhand
  `background-*` properties.
- **Files:** each file ≤ 500 lines. Bundled fonts only. Components keep using tokens, and `guard.test.ts` stays green.
  If a touch needs a hook, add a generic `data-*` hook in core. Never write palette-specific code in components.
- **Process:** no Claude attribution in commits. `make check-core` stays green. The till and dialogs work at desktop
  size; admin works at 360 px in both modes.

## Review Focus

1. **A palette chosen at the till PC with Windows' "reduce motion" on:** Aurora's drift (and any other animation)
   must stop. Task 1's `motion` test checks that every `@keyframes` palette has a reduced-motion rule.
2. **Duplicate a new palette, then edit it in the palette editor:** the copy keeps the signature CSS (still under
   Custom CSS) and keeps both modes. Task 1's `duplicate` test round-trips `buildPalette(builtinDoc(id))` →
   `parsePalette` and compares `extra`.
3. **A see-through card (Nord, Aurora, any glass palette) under the cart totals:** text must stay readable once the
   card is composited on the page. Task 1's `contrast` test composites first, which `failing()` already does.
4. **A shop saved on a new palette, then the till reloads or goes to the login page (never signed in on that
   device):** the palette must show before sign-in, because login-info carries `settings.skin`. Task 6's e2e covers
   this.
5. **Signature CSS reaching an element the guard can't see** (for example a glow on `input` that hits the PIN pad):
   the shot review in each batch task covers the lock / PIN pad and the till with a cart, in the batch's palettes.

---

### Task 1: Palette checks + Gruvbox (the first new palette)

**Files:**
- Create: `web/src/theme/palettes/palettes.test.ts`, `web/src/theme/palettes/dock/gruvbox.css`
- Modify: `web/src/theme/palettes/index.ts` (append the entry), `web/src/app.css` (`@import` after `catppuccin.css`),
  `internal/features/settings/settings.go:90` (`Skins` += `gruvbox`), `internal/api/dto.go:55` (palette comment
  becomes `// a built-in id (settings.Skins) | "custom-<id>"` so it stops listing them),
  `internal/features/settings/appearance_test.go` (add a case)

**Interfaces:**
- Consumes: `PALETTES` (`theme/palettes/index.ts`), `TOKENS` (`theme/palettes/tokens.ts`), `builtinDoc`,
  `buildPalette`, `parsePalette` (`features/appearance/paletteCss.ts`), `PAIRS`, `failing` (`features/appearance/contrast.ts`).
- Produces: `NEW_PALETTES: string[]` exported from `palettes.test.ts`'s sibling `web/src/theme/palettes/index.ts`, the
  ids above in grid order. It is the test's subject list, and later tasks append to it. A test-side static resolver
  `resolveToken(tokens: Record<string,string>, name: string): string | null` follows `var(--x)` chains, and returns
  null for anything it can't read (an unreadable PAIRS token fails the test).

- [ ] **Step 1: Write the failing tests.** In `palettes.test.ts`, write one `describe` per id in `NEW_PALETTES`:
  - `registered`: the id is in `PALETTES` with a non-empty name and swatch, and `isBuiltin(id)`.
  - `every token, both modes`: `parsePalette(css)` gives `light` and `dark` maps that each hold every `TOKENS[].name`.
  - `contrast`: for light and dark, `failing((t) => resolveToken(map, t), PAIRS)` is `[]`. The light map is the
    light block. The dark map is the light block overlaid with the dark block.
  - `duplicate`: `parsePalette(buildPalette(builtinDoc(id), 'custom-1'))` keeps `extra` (trimmed equal) and both maps.
  - `extra is safe`: `doc.extra` doesn't match `/prefers-color-scheme/`. If the CSS has `@keyframes`, it also matches
    `/prefers-reduced-motion:\s*reduce/`.
  - `size`: the file is ≤ 500 lines.
  - `old five untouched`: a separate `it` checks that `studio,night,bazaar,lagoon,catppuccin` are the first five
    `PALETTES` ids, in that order.

  In Go, add the case `{"gruvbox is built in", ValidSkin("gruvbox"), "gruvbox"}`.
- [ ] **Step 2:** Run `cd web && bun run test src/theme/palettes` → FAIL (no `NEW_PALETTES` / no gruvbox file). Run
  `go test ./internal/features/settings -run Appearance` → FAIL ("studio" ≠ "gruvbox").
- [ ] **Step 3: Implement Gruvbox:**
  - **Palette:** Gruvbox light and dark (bg `#fbf1c7` / `#282828`, fg `#3c3836` / `#ebdbb2`, orange accent
    `#af3a03` / `#fe8019`, the published greens, reds and yellows for status).
  - **Feel:** solid surfaces, `--radius` 4 px, square pills, hard offset `--shadow` (`2px 2px 0`).
  - **Extra:** a faint paper-grain `background-image` on the page, an SVG noise data URI at low opacity, written as
    longhand.

  Also add the `NEW_PALETTES` export, the registry entry, the `@import`, the Go `Skins` entry and the dto comment.
- [ ] **Step 4:** Both test commands PASS. Also `bun run test src/theme` (guard) and `make check-core` stay green.
- [ ] **Step 5: Gate.** Shots go to `web/shots/pal2-1/`, Gruvbox, light and dark:
  - `SHOTS=pal2-1 SHOTS_PALETTES=gruvbox` for `till-shots` (`SHOTS_SCENE=sale` and `done`), `admin-shots`
    (`/admin`, `/admin/products`, `/admin/settings`) and `admin-shots SHOTS_SYSTEM=1 SHOTS_PATH=/admin/system/appearance
    SHOTS_PREP=appearance-palettes`.
  - Look at each shot: text readable, nothing clipped at card edges at 360 px, the grain doesn't touch the cart or
    tables.
- [ ] **Step 6: Commit** `feat(palettes): palette checks + Gruvbox`.

### Task 2: Nord, Dracula, Tokyo Night

**Files:** Create `dock/{nord,dracula,tokyonight}.css`. Modify `palettes/index.ts` (entries + `NEW_PALETTES`),
`app.css`, `settings.go` `Skins`, and `appearance_test.go` (one case per id).

**Interfaces:** Consumes Task 1's `NEW_PALETTES` and checks. They run unchanged on the new ids.

- [ ] **Step 1:** Append the three ids to `NEW_PALETTES` and add the Go cases.
- [ ] **Step 2:** `bun run test src/theme/palettes` → FAIL (missing files). Go → FAIL.
- [ ] **Step 3: Implement**, each with its spec signature:
  - **Nord:** frosted `--glass`, a see-through `--card`, and a faint aurora gradient in `--bg-art`.
  - **Dracula:** dark is Dracula (`#282a36`, purple `#bd93f9`, pink `#ff79c6`), light is Alucard. The extra CSS puts
    a neon `--glow` on main buttons and focus rings. Glow colour comes from a per-mode `--sig-glow` var.
  - **Tokyo Night:** dark is Night (`#1a1b26`, blue `#7aa2f7`), light is Day. Night-sky `--bg-art`, plus a soft glow on
    the dock and main button.
- [ ] **Step 4:** PASS, guard green.
- [ ] **Step 5: Gate:** Task 1's shot set to `web/shots/pal2-2/` for these three, plus `till-shots SHOTS_SCENE=closed`
  to see the lock overlay with the glow.
- [ ] **Step 6: Commit** `feat(palettes): Nord, Dracula, Tokyo Night`.

### Task 3: Rosé Pine, Solarized, Everforest, Kanagawa

**Files and Interfaces:** same pattern as Task 2, for `rosepine, solarized, everforest, kanagawa`.

- [ ] **Step 1:** Append the ids and add the Go cases. **Step 2:** FAIL.
- [ ] **Step 3: Implement:**
  - **Rosé Pine:** Main / Dawn. Extra-round corners (`--radius` ≥ 16 px), a calm motion feel, `--dur` at the slow end
    (≤ 350 ms).
  - **Solarized:** dark / light. Flat: `--shadow` none, `--glow` none, `--lift` 0, hairline borders.
  - **Everforest:** dark / light medium. Leafy soft `--bg-art` (gradient blobs), gentle shadows.
  - **Kanagawa:** Wave / Lotus. A faint repeating wave pattern on the page background only (longhand). It sits under
    `--card`, so it never shows through text.
- [ ] **Step 4:** PASS. **Step 5: Gate** to `web/shots/pal2-3/` (Task 1's set). **Step 6: Commit**
  `feat(palettes): Rosé Pine, Solarized, Everforest, Kanagawa`.

### Task 4: Synthwave '84, Monokai, Shades of Purple, Cobalt2, Ayu

**Files and Interfaces:** same pattern, for `synthwave, monokai, shadesofpurple, cobalt2, ayu`.

- [ ] **Step 1:** Append the ids and add the Go cases. **Step 2:** FAIL.
- [ ] **Step 3: Implement:**
  - **Synthwave '84:** neon pink `#ff7edb`, cyan `#36f9f6` and yellow `#fede5d` on `#241b2f`, with a made-up light
    version. A sunset-grid `--hero-art` / `--bg-art`. A `text-shadow` glow on headings (`h1`, `h2`) and the hero total
    only, never body text or tables.
  - **Monokai** (the classic free one): `#272822`, pink `#f92672`, green `#a6e22e`, yellow `#e6db74`, with a made-up
    light version. Every role gets its own bright colour: brand pink, brand-2 green, success green, warning yellow.
  - **Shades of Purple:** `#2d2b55`, with yellow `#fad000` as brand (dark text on it) and orange `#ff9d00`.
  - **Cobalt2:** `#193549`, gold `#ffc600`, with crisp 1 px outlines on cards (extra CSS through the `--shadow` inset
    rim). High contrast.
  - **Ayu:** Mirage dark / Light, orange `#ffcc66` / `#fa8d3e` (dark text where needed) and sky blue.
- [ ] **Step 4:** PASS. **Step 5: Gate** to `web/shots/pal2-4/` (Task 1's set + `SHOTS_SCENE=closed`). **Step 6:
  Commit** `feat(palettes): Synthwave '84, Monokai, Shades of Purple, Cobalt2, Ayu`.

### Task 5: Mango, Pop, Aurora, Neumorph (our own)

**Files and Interfaces:** same pattern, for `mango, pop, aurora, neumorph`.

- [ ] **Step 1:** Append the ids and add the Go cases. **Step 2:** FAIL.
- [ ] **Step 3: Implement**, following the colourful mockup:
  - **Mango:** mango orange `#ff6b00`, leaf green `#00a676`, hibiscus pink `#ff2e63`. A warm gradient `--bg-art` wash
    and big corners (`--radius` 16 px).
  - **Pop:** primary red, blue and yellow on cream / black. Thick 2 px ink borders and a hard `3px 3px 0` shadow on
    cards, buttons and the dock, done through `--shadow` / `--shadow-float` rims plus extra CSS borders on generic
    hooks.
  - **Aurora:** teal `#00c2a8` to violet `#6a5cff` gradient on the main button (a `background-image` longhand through
    a `--sig-btn` var), liquid glass cards and a slowly drifting glow in `--bg-art`. The drift is a pseudo-element
    `@keyframes` on transform / opacity, ≥ 20 s a loop, with a reduced-motion stop.
  - **Neumorph** (neumorphism):
    - **Colours:** light page and card `#e0e5ec`, text `#2d3748`; dark page and card `#2a2d32`, text `#e6e8eb`.
      `--card` equals `--background`.
    - **Raised surfaces:** a double `--shadow` (a light highlight top-left, a dark shade bottom-right), with values
      per mode.
    - **Pressed look:** the extra CSS gives pressed / selected controls (`[aria-pressed="true"]`,
      `[aria-checked="true"]`, `[aria-selected="true"]`) and inputs an inset shadow, through a per-mode
      `--sig-inset` var.
    - **Brand:** a solid, saturated colour (for example `#5a67d8`) on main buttons, so actions stand out.
    - **Readability:** borders nearly invisible, but `--faint` and `--muted-foreground` still pass PAIRS.
    - **Gate:** check the PIN pad, the cart and every pressed control in light and dark.
- [ ] **Step 4:** PASS, including the `extra is safe` reduced-motion check for Aurora. **Step 5: Gate** to
  `web/shots/pal2-5/` (Task 1's set). On a mid PC, check that the till stays smooth with Aurora: no long frames when
  adding cart lines (Chrome performance panel or a Playwright trace).
- [ ] **Step 6: Commit** `feat(palettes): Mango, Pop, Aurora, Neumorph`.

### Task 6: Picker e2e, plugin screens, docs

**Files:**
- Modify: `web/e2e/palettes.spec.ts` (new test), `docs/design/theme.md` (§1 list of built-ins), `AGENTS.md` (Hard
  rules' palette list: five → the 22, or "built-ins (theme.md §1)"), `docs/status.md`

- [ ] **Step 1: Failing e2e:** `test('every built-in palette is in the picker; a new one saves and shows on the login
  page')`.
  - The system user sees 22 built-in swatches in grid order (the first is Studio, the last is Neumorph).
  - Pick Kanagawa, then Save. Reload: `document.documentElement.dataset.palette === 'kanagawa'`. Sign out: the login
    page has `data-palette="kanagawa"`.
  - `afterAll` resets the palette to `studio`.
- [ ] **Step 2:** Run it. It fails until the grid count matches, or passes straight away if Tasks 1–5 already produce
  22. If it passes first time, ledger that.
- [ ] **Step 3: Plugin screens.** Build with `make plugins P=all`. Take `admin-shots` of one plugin page (`SHOTS_PATH=
  /admin/x/repairs`) in `SHOTS_PALETTES=dracula,pop,aurora`, light and dark, to `web/shots/pal2-6/`. Plugin tokens
  must follow. Then `make core-only`.
- [ ] **Step 4: Full runs:** `make test`, `make test-web`, `make check-core`, and Playwright (free ports 3100 and 5173
  first).
- [ ] **Step 5: Docs:**
  - theme.md §1: the 22 built-ins and a line per new palette.
  - AGENTS hard rule: the palette list.
  - status.md: a "More palettes — BUILT" entry.
- [ ] **Step 6: Commit** `docs: 17 more built-in palettes — theme, AGENTS, status`.
