# Palettes part 2: tokens everywhere, four built-ins, custom palettes (design, 2026-10-21)

## Intent

A **theme** is a whole rework of components and layout (sidebar or top-nav shells, other tills). We are not doing
themes now. What exists is **theme 1**, named **Dock** (key `dock`, after its floating dock shell; my call), the only
one.

**Palettes belong to a theme** (user, 2026-10-21): another theme moves and reshapes components, so its palettes
need their own values and maybe their own tokens. Every palette, built-in or custom, records its theme. Built-ins
live in `web/src/theme/palettes/dock/`, `<html>` carries `data-ui="dock"`, and every palette selector includes it
(`:root[data-ui="dock"][data-palette="studio"]`), so a future theme's "studio" never clashes. The Appearance
picker only lists the current theme's palettes.

A **palette** changes the look of theme 1's existing components and nothing else: colours, surface material,
shapes, depth (shadow / glow) and motion. Every palette uses the same set of tokens, and every component reads those
tokens the same way, so a new palette, built-in or custom, is just a new set of values. **Tokenising the whole UI is
half the point** (user): later refactors, theme packs and "make it look good again" passes then happen in one
place.

Who uses it: **the system user only**, on Appearance (the hidden page, unchanged). The user does this for each shop.

## Goals

1. **Every look-related value in the UI is a token.** No component hard-codes a corner size, shadow, motion timing or
   raw colour. A palette file can then restyle every screen without touching a component.
2. **Four built-in palettes** for theme 1, each with light and dark: Studio, Night and two new ones.
3. **Custom palettes:** the system user starts from any palette (built-in or custom), changes anything for light
   and dark, and saves it to the shop's DB. Create as many as they want.
4. **Export / Import** of custom palettes as one `.css` file, so a palette made for one shop goes to another.
   **Built-ins are not exported** (user): duplicate one and export the copy.

## Non-goals (now)

- Themes / theme packs (layout or component changes): `docs/specs/2026-09-27-karots-suite-design.md` §6.4, later.
- Fonts. `--font-sans` stays a token, but the editor doesn't offer it yet (only bundled fonts would work offline).
- Per-user or per-device palettes. The palette is shop-wide; **mode** (light / dark) stays per device (`ks-mode`).
- JSON palette formats. Palettes are CSS, hand-written or saved by the editor in the same shape (user).

## Anti-goals

- A component that checks which palette is active (already a hard rule, AGENTS.md).
- A palette that breaks a screen: unreadable text, a dialog with no backdrop, a till button with no edge. The four
  combinations of every built-in pass the usual screenshot gate.
- Looking like karots-pos.

## 1. The token set (theme 1)

All tokens live in the palette's two blocks (light, dark). The colour tokens stay exactly as in
`docs/design/theme.md` §2. New ones (names are the builder's call; these are the intended set):

| Group | Tokens | Notes |
|---|---|---|
| Shape | `--radius`, `--radius-sm`, `--radius-pill` (chips, pill buttons, the dock), `--border-w` | `--radius-pill` can be `9999px` (round) or a small value (square-ish pills). True circles (avatars, dots, spinners) stay `rounded-full`. |
| Material | `--card`, `--bar`, `--glass` (already) + `--sheen` (an optional top highlight layer), `--edge` (the inner border / rim light) | **solid**: opaque `--card`, `--glass: none`, no sheen. **frosted**: translucent card + blur (Night today). **liquid glass**: more transparent, stronger blur + saturate, a curved `--sheen` gradient and a bright `--edge`. |
| Depth | `--shadow`, `--shadow-float`, `--glow` (already) | |
| Background | `--background`, `--bg-art`, `--hero`, `--hero-art` (already) | |
| Motion | `--dur-fast`, `--dur`, `--dur-slow`, `--ease`, `--ease-out`, `--spring` (a stiffness/damping pair Motion reads), `--lift` (hover rise in px, 0 = none) | **Calm**: short, soft ease, no overshoot, no lift. **Lively**: a springy ease with slight overshoot, hover lift. Keep everything ≤ 350 ms; `prefers-reduced-motion` still turns it all off. |

The `data-radius` switch (`sharp | rounded | round`) and the brand-colour override go: both become palette values
(see §5).

### The tokenising sweep

Measured on 2026-10-21 in `web/src` (`.tsx`): `rounded-md` 32, `rounded-lg` 26, `rounded-xl` / `2xl` / `3xl` 12,
`rounded-sm` 4, `rounded-full` 183 (most are pills or circles; sort each), `shadow-sm/md/lg` 6, `text/bg/border-white`
and `bg-black` ~16, Tailwind `duration-*` 8, and 14 files using `motion/react` with literal timings. All of them move
onto tokens: either Tailwind theme mappings in `tokens.css` (`@theme` so `rounded-lg` itself means a token) or explicit
`var(--…)` classes. The builder picks whichever keeps the diff smallest and reads cleanly. Print routes keep
`--paper` / `--ink` (physical media). Add a `make check-core`-style guard (or a Vitest scan) that fails on a new raw
corner, shadow, colour or duration class, so the sweep stays done.

## 2. Built-in palettes

One CSS file each in `web/src/theme/palettes/dock/`, listed in `palettes/index.ts` with `theme: 'dock'`, name and swatch:

| Palette | Material | Motion | Look (builder designs the exact values) |
|---|---|---|---|
| **Studio** (have) | solid | calm | saffron, neutral greys, unchanged |
| **Night** (have) | frosted | calm | violet / cyan glass, unchanged |
| **Bazaar** (new, my call; named in theme.md as "later") | solid | lively | warm Sri Lankan market: terracotta / turmeric with a deep teal, warm paper background in light, deep brown-black in dark, rounder pills |
| **Lagoon** (new, my call) | liquid glass | calm | sea-glass aqua / coral over a soft gradient background, strong blur, sheen and bright edges |

Studio and Night must look exactly as they do today once the sweep is done: same tokens, same values. The two new
ones get a screenshot gate in all four combinations, desktop + 360, including the till, login and a dialog.

## 3. Custom palettes

- **Storage:** a new core table (migration `00011_`), e.g. `palettes(id, theme, name, base, css, created_at, updated_at)`.
  `base` = the palette it was started from (for the editor's "Reset to base"). `css` is the whole palette file. It's
  part of the shop's data, so it's in backups and restores. Old backups have none, which is fine.
- **Which palette is on:** `settings.skin` holds a built-in id (`studio`, `night`, `bazaar`, `lagoon`) or a custom
  one (`custom-<id>`). Today's `skin` values keep working.
- **Same file shape as a built-in:** `:root[data-ui="dock"][data-palette="<id>"] { … }`, `…[data-theme="dark"] { … }` and the
  `prefers-color-scheme` block. An exported custom palette could be dropped into `palettes/dock/` as a new built-in
  with only its id changed. The file starts with a comment header naming its theme (`/* karots-palette theme=dock
  name=… */`); Import refuses a file for another theme.
- **Loading:** the active custom palette must apply on the **login page** too (public, before anyone signs in) and
  with no flash. The SPA keeps today's `ks-palette` pre-paint script; the active custom palette's CSS is served at a
  public `GET /api/v1/appearance/palette.css` (empty for a built-in) and linked from `index.html`. Builder may pick a
  different route that meets the same need.
- **Limits:** name required and unique; CSS ≤ 64 KB; delete refused for the active palette; system user only for
  every write (same guard as Appearance today).
- **Trust:** the CSS comes from the system user, so it isn't sanitised beyond the size limit, but it is served as
  `text/css` (never inlined into HTML) so it can't break out of a stylesheet.

## 4. The editor (Appearance)

Appearance gets a **Palettes** section above Receipt styles:

- A picker with every built-in and custom palette (swatch cards like today), **Use** (sets `skin`), **Edit** (custom
  only), **Duplicate** (any → new custom, named "<name> copy"), **Export** (custom only → `<name>.css` download),
  **Import** (`.css` → a new custom palette; the file's palette id is rewritten to the new id; another theme's file
  is refused), **Delete** (custom,
  not the active one).
- **Edit** opens the editor for that palette, with a **Light | Dark** switch (flips the page's mode while editing)
  and grouped controls:
  - **Colours**: every colour token, grouped (background, surfaces, text, brand, selected, money in/out/warning,
    hero, charts). A colour field + hex/rgba text box each.
  - **Material**: Solid / Frosted / Liquid glass. Picking one fills the material tokens with sensible values for
    that mode; blur strength and card opacity sliders tune it.
  - **Background**: plain colour or the palette's art gradient (on/off, plus its two colours).
  - **Depth**: shadow strength, glow on/off + colour.
  - **Shape**: corner radius (a slider covering today's sharp → round), pill shape (round | soft), border width.
  - **Motion**: Calm / Lively, plus a speed slider.
  - **Custom CSS**: a text box for extra rules, saved at the end of the palette's CSS.
- **Live preview:** every change applies to the page at once (the editor writes a draft `<style>`), like the brand
  colour does today. Leaving without saving puts the saved palette back. A small preview strip (a card, buttons, a
  segmented control, a till total, a dialog) sits beside the controls so the system user sees the effect without
  leaving the page. At 360 the controls stack above the strip.
- **Save** writes the CSS. **Reset to base** reloads the base palette's values.
- The controls read and write the token lines inside the palette's light and dark blocks. A hand-written or imported
  palette whose CSS has more than tokens keeps that extra CSS in the Custom CSS box.

## 5. What goes away and how old data maps

- The **Brand colour** and **Radius** controls go from Appearance, `settings.skin_custom` / `skin_radius` stop being
  read, and `Appearance.brandColor` / `radius` leave the DTO.
- A shop (or a restored backup) with a brand colour set or a radius other than `rounded` gets a custom palette made
  for it once (at migration and on restore): "<Palette> (shop)", the base palette with that brand / radius, and set
  as active, so nothing looks different after the update.

## Left to the builder

Exact token names and values, the Tailwind mapping technique, the two new palettes' colours, the editor's control
widgets and layout (the usual look bar: not default-looking; build a quick mockup first if unsure), the CSS
read/write helper (keep it a small pure module with Vitest tests), and the route names.

## Calls I made (user to check)

- Theme 1 is named **Dock**; palettes are per theme, never shared across themes.
- New palettes **Bazaar** (solid, lively) and **Lagoon** (liquid glass, calm), so the four cover every material and
  both motions.
- "Animation" = motion **feel and speed** (calm vs lively, a speed slider), not new animations per palette.
- Fonts stay out of the editor for now.
- Brand colour and radius become palette values; existing shops with them set are converted to a custom palette so
  they don't change.
