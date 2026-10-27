# More built-in palettes — design

Date: 2026-10-27 · Theme: Dock · Mockups: `docs/design/mockups/palettes-famous.html`,
`docs/design/mockups/palettes-colourful.html` (brainstorm fragments: open them through the visual companion or read
their inline styles. They show colours and the signature idea, not exact token values).

## Why

When the user sets up a shop, they sit with the owner, click through every palette on Appearance and save the one the
owner likes. Only the system user can change it later. Five built-ins (Studio, Night, Bazaar, Lagoon, Catppuccin) are
too few to show, and the user wants:

- the famous palettes people know from editors and terminals, the way Catppuccin was added
- bright, colourful ones, not only dimmed / muted colours
- "more advanced": each palette is more than colours. It uses everything a palette file can set (material, corners,
  shadows, motion) **and the Custom CSS layer** for a signature touch.

## Goals

1. **Add 17 built-in Dock palettes.** Keep the existing five unchanged. That makes 22 built-ins.
   - **Calm / famous:** Gruvbox, Nord, Dracula, Tokyo Night, Rosé Pine, Solarized, Everforest, Kanagawa.
   - **Colourful:**
     - Famous: Synthwave '84, Monokai, Shades of Purple, Cobalt2, Ayu.
     - Our own originals: **Mango** (mango orange, leaf green, hibiscus pink), **Pop** (comic-book red, blue and
       yellow with thick black outlines), **Aurora** (teal-to-violet gradients, glass cards) and **Neumorph**
       (neumorphism, below). Added at the user's request, 2026-10-27.
2. **Light and dark for each, both good.** Use the theme's official light variant where it has one: Dracula →
   Alucard, Tokyo Night → Day, Rosé Pine → Dawn, Kanagawa → Lotus, Ayu → Light, plus Gruvbox, Solarized and
   Everforest's own light versions. Where there's none (Nord light, Synthwave '84, Monokai, Shades of Purple,
   Cobalt2), make a matching light version. (My call.)
3. **Faithful colours.** Famous palettes use their published hex values for the base, surfaces, text and accents,
   mapped onto our token roles. A palette should be recognisable to someone who uses it.
4. **A full personality per palette.** Material (solid / frosted / liquid glass), corners, pills, shadows, glow,
   background art and motion (feel + length) chosen to fit it.
5. **One signature touch per palette** in the file's extra-CSS part, the same layer as the editor's "Custom CSS".
   It's the mockups' signature line:

   | Palette | Signature touch |
   |---|---|
   | Gruvbox | Square corners, hard offset shadows, faint paper grain |
   | Nord | Frosted glass, a faint aurora glow in the background |
   | Dracula | Neon glow on the main buttons and focus |
   | Tokyo Night | Soft neon-sign glow, night-sky backdrop |
   | Rosé Pine | Extra-round corners, slow gentle motion |
   | Solarized | Flat: no shadows, crisp lines |
   | Everforest | Soft leafy background art, gentle shadows |
   | Kanagawa | A faint wave pattern behind the page |
   | Synthwave '84 | Glowing text and buttons, a sunset-grid sky |
   | Monokai | Each role in its own bright colour |
   | Shades of Purple | Yellow totals and main button pop |
   | Cobalt2 | Crisp outlines, high contrast |
   | Ayu | Warm, cheerful, very clean |
   | Mango | Warm gradient wash, big round corners |
   | Pop | Thick borders, hard shadows |
   | Aurora | Gradient main buttons, glass cards, slowly drifting glow |
| Neumorph | Neumorphism: cards the same colour as the page, raised by a soft light-and-dark double shadow; pressed tabs, segments and inputs look pushed in (inset shadow). Light is soft grey-blue; dark is charcoal. The brand colour stays solid and clearly visible on main buttons, so actions never get lost in the softness |

6. **The picker shows them as one flat grid of all 22**, in this order: the existing five, the eight calm ones, then
   the eight colourful ones. No groups, filters or search (user: "just spread all of these"). Clicking one previews it
   live on the whole page, as today.
7. **Duplicate works on every new one.** The copy carries its colours, feel and signature CSS into a custom palette
   the system user can edit, the same as Duplicate does for the existing built-ins.

## Non-goals

- New themes. The shell stays Dock; these are palettes only.
- Changing Studio, Night, Bazaar, Lagoon or Catppuccin.
- Grouping, tags, search or favourites in the picker.
- Letting non-system users pick palettes (mode light / dark stays per device, as now).
- Anything on paper. Receipts, labels and the A4 sheets stay black and white.

## Anti-goals (a failure even if it "works")

- **A signature touch that hurts the work.** Glow, grain, patterns or gradients must never make prices, totals, the
  cart, PIN entry or tables hard to read, and must never slow the till. Effects sit on backgrounds, cards, the main
  button and the dock, not on text blocks (except a small glow on a hero total, where it stays readable).
- **Unreadable text.** Body text must reach WCAG AA (4.5:1) on its surface, in both modes. That includes text on
  buttons (`--brand-foreground` on `--brand`, `--danger-foreground` on `--danger`): our button text is too small to
  count as large text. Muted and faint text may be softer, but never below 3:1. Keep a famous palette's published
  accent and pass the bar by picking the text colour (dark text on a bright brand such as Ayu orange or Cobalt2
  yellow), not by changing the accent.
- **Status colours that lie.** Success stays green-family, danger red-family and warning amber-family in every
  palette, whatever the theme's accent is.
- **Endless motion that eats a till PC's CPU.** Any animation (Aurora's drift, for example) is slow, uses only
  transform / opacity, and stops under `prefers-reduced-motion`.
- **A screen that breaks in one palette.** Every screen has to work in every palette and both modes, including the
  till, dialogs, the login hero and phone admin at 360 px.
- **Pixel-identical copies of an editor theme screen.** We take each palette's colours, not its UI.

## Constraints

- **Files.** One CSS file per palette in `web/src/theme/palettes/dock/<id>.css`, in the existing file format (light
  block, dark under `prefers-color-scheme`, `[data-theme]` overrides, then extra CSS). It must parse with
  `paletteCss.ts` so the editor and Duplicate can read it. One `PALETTES` entry per palette (with swatch), one
  `@import` in `app.css`, and the id in Go's `settings.Skins`.
- **Ids.** Lower-case ascii: `gruvbox`, `nord`, `dracula`, `tokyonight`, `rosepine`, `solarized`, `everforest`,
  `kanagawa`, `synthwave`, `monokai`, `shadesofpurple`, `cobalt2`, `ayu`, `mango`, `pop`, `aurora`, `neumorph`.
- **Tokens.** Every token in `palettes/tokens.ts` is set in both modes. Plugin tokens (`slots.paletteTokens`) follow
  each palette, just as they do for the five.
- **Guard rules hold.** Components still use tokens only and never branch on the active palette
  (`theme/guard.test.ts`). The extra CSS may target shell and kit elements by their existing stable hooks (classes,
  `data-*`, roles). If a signature touch needs a hook that doesn't exist, add a generic `data-*` hook in core,
  never palette-specific code.
- **The `background` shorthand gotcha (AGENTS.md).** Multi-layer `--bg-art` needs longhand `background-*`.
- **Motion caps.** `--dur*` ≤ 350 ms. Infinite animations are allowed only for background art, and only when slow
  (≥ 20 s a loop).
- **Size and fonts.** Each file stays under 500 lines. The added CSS must not noticeably slow the first paint of the
  login (it's render-blocking). Bundled fonts only; no new font downloads.
- **Names and licences.** Palette names are shown as the themes call themselves (Dracula, Nord, …). Mango, Pop and
  Aurora and Neumorph are ours. Only use free / open palettes: the colourful slot is the original free **Monokai** (id `monokai`),
  not the paid Monokai Pro.

## Decided with the user

- All 16, added, nothing replaced (2026-10-27). Then a 17th: a neumorphism palette (user, 2026-10-27).
- One flat grid, no grouping. The system user walks the owner through them at shop setup.
- "More advanced" means using the Custom CSS layer for each palette's signature, not new editor controls.

## My calls (the user can overrule)

- Made-up light versions where a theme has none.
- The grid order above.
- The signature touches listed above (from the mockups).
- AA contrast as the bar (4.5:1 for button text too), and status colours kept green / red / amber.
- The original free Monokai instead of the paid Monokai Pro, for the licence. It looks almost the same. (The user
  said shops can pick whatever they like, so the name isn't important to them.)
- No existing built-in uses the extra-CSS part yet, so these are the first. The parser drops an
  `@media (prefers-color-scheme: dark)` block in the extra part, so a signature touch that differs by mode reads custom
  `--vars` set inside each mode's token block.
- Synthwave's glowing text: only on headings and the hero total, never body text or tables (the anti-goal wins).

## Left to the builder

- Exact token values for each role. Map from the published palettes; tune for contrast and for our components.
- How each signature is built in CSS, within the rules above.
- Swatch colours for the picker.
- How the work is split into tasks (for example by batch: calm, then colourful, then the originals).

## How it's checked

- Vitest:
  - every new palette parses (`paletteCss`) with every token set in light and dark
  - `builtinDoc(id)` and Duplicate keep the extra CSS
  - a contrast test for the **17 new palettes** only (the existing five are untouched; Night light's
    danger-foreground on danger is 4.47:1, a known existing gap), each in both modes:
    - foreground on background
    - foreground on card, with a see-through `--card` composited over `--background` first
    - brand-foreground on brand and danger-foreground on danger
    - fails below the bar
- Go: the new ids are valid skins, and an unknown id still falls back.
- Playwright: the picker lists 22. Choosing a new palette and saving shows it after a reload and on the login page.
- Shot review (UI gate), for every new palette × light / dark:
  - the till with a cart, Pay open, a dialog, the dashboard, a list page, Settings and the login hero
  - desktop and 360 px (phone shots for the admin pages)
  - checked for readability, card edges and anything the signature touch breaks
