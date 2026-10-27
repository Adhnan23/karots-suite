# Theme and palettes (approved 2026-09-27, revised same day)

**Approved mockups:**
- Shell, login and palettes: https://claude.ai/artifact/MANYgogqsxC4evmSADNkia. Its Palette and Mode switches show all four combinations, and it lists the tokens at the bottom.
- Till look and feature density: https://claude.ai/artifact/GkJjnypEes4GmHiHMDzFT5. Use its "Night Counter" and "Studio" directions only.

**Both mockups are look demos, not feature specs.** The real screens implement every
karots-pos feature (`docs/parity.md`) in this style. Before building a screen, check
karots-pos for what it has; don't copy the mockups' fake data or missing features.

## 1. Palettes and modes

These are two separate controls:

| Control | Who sets it | Values | Stored in |
|---|---|---|---|
| **Palette** | The system user only (hidden system menu → Appearance) | built-ins `studio`, `night`, `bazaar`, `lagoon`, `catppuccin`; custom palettes `custom-<id>` | `settings.skin` (existing column); custom palettes in the `palettes` table |
| **Mode** | Each device, with the ModeToggle | System (default), Light, Dark | `localStorage` key `ks-mode` |

Rules:

- **Each palette is one colour family with a light AND a dark version.** Switching the mode
  keeps the palette's colours: Studio stays saffron in both modes, and Night stays
  violet/cyan glass in both. Mode never changes the palette.
- **Every palette defines every token** in both modes (§2), and components use only
  those tokens. Components never check which palette is active: glass, glow and
  selected colours are tokens too.
- **Themes and palettes (2026-10-21, `docs/specs/2026-10-21-palettes-design.md`):** a *theme* is a whole
  layout / component set; there is one, **Dock** (`<html data-ui="dock">`). A *palette* restyles a theme's components
  (colours, material, shape, depth, motion) and belongs to that theme. Built-ins: **Studio** (solid, calm), **Night**
  (frosted, calm), **Bazaar** (solid, lively), **Lagoon** (liquid glass, calm), **Catppuccin** (Latte light / Mocha
  dark, pastel, frosted bars, lively; made by the user in the editor, 2026-10-21).
- **One file per built-in palette:** `web/src/theme/palettes/dock/<name>.css`, listed in `palettes/index.ts`.
  - Selectors are `:root[data-ui="dock"][data-palette="<name>"]` for light, `…[data-theme="dark"]` for dark, and `@media (prefers-color-scheme: dark) { …:not([data-theme="light"]) }`.
  - Adding a palette means adding one CSS file plus one entry in the palette list. Nothing else changes. `palettes/tokens.test.ts` checks every file defines every token in `palettes/tokens.ts`.
- **Custom palettes:** the system user duplicates any palette on Appearance and edits it (colours per mode, material, background, depth, shape, motion, Custom CSS; the page previews live). It's saved as the same file shape (with a `/* karots-palette theme=… name=… base=… */` header) in the `palettes` table and served at the public `GET /api/v1/appearance/palette.css`, linked from `index.html` so it paints with the first frame (login included). Custom palettes export / import as `.css`; built-ins don't export. `features/appearance/paletteCss.ts` reads and writes the files.
- **Plugins** declare their own look tokens through `slots.paletteTokens` (defaults built from core tokens in the plugin's CSS); the editor lists them under Plugins.
- **Legacy brand / radius** (`settings.skin_custom` / `skin_radius`, the old Appearance controls): still applied as inline overrides while the palette is a built-in, cleared when another palette is chosen.
- **Every screen works with every palette in both modes.** `theme/guard.test.ts` fails on raw colour classes, raw durations / easings, pixel corners and literal Motion timings.

## 2. Palette tokens

These are the same names in every palette. Names follow shadcn, so its components work unchanged.

**Warning:** shadcn's `--accent` means a hover/neutral background, NOT the brand. The brand is
`--brand`, and `--primary` points at it.

| Token | Studio light | Studio dark | Night light | Night dark |
|---|---|---|---|---|
| `--background` | `#EFF0F3` | `#111215` | `#EEF0FA` | `#0B1020` |
| `--bg-art` | `none` | `none` | violet `rgb(124 92 255/.22)` at 12% −10% + cyan `rgb(34 211 238/.20)` at 110% 110% | the same gradients at `.35` / `.22` |
| `--foreground` | `#15161A` | `#EDEEF1` | `#161A33` | `#EAF0FF` |
| `--card` | `#FFFFFF` | `#1B1C21` | `rgb(255 255 255/.62)` | `rgb(255 255 255/.055)` |
| `--popover` (always solid) | `#FFFFFF` | `#1F2026` | `#FFFFFF` | `#161D33` |
| `--muted` = `--secondary` = `--accent` | `#F6F7F9` | `#24262C` | `rgb(110 91 255/.07)` | `rgb(255 255 255/.085)` |
| `--muted-foreground` | `#5A5E69` | `#A6A9B3` | `#50577A` | `#A3AECB` |
| `--faint` | `#6E727D` | `#8A8E99` | `#62698B` | `#7A84A6` |
| `--border` = `--input` | `#E4E6EB` | `#2E3038` | `rgb(60 50 140/.12)` | `rgb(255 255 255/.10)` |
| `--brand` | `#F2A900` | `#F5B820` | `#6E5BFF` | `#8B7BFF` |
| `--brand-foreground` | `#15161A` | `#15161A` | `#FFFFFF` | `#0B1020` |
| `--brand-soft` | `#FFF4D6` | `rgb(245 184 32/.16)` | `rgb(110 91 255/.13)` | `rgb(139 123 255/.18)` |
| `--brand-2` / `--brand-2-foreground` | `#15161A` / `#FFFFFF` | `#EDEEF1` / `#111215` | `#0A9FB4` / `#FFFFFF` | `#2FD5E6` / `#0B1020` |
| `--selected` / `--selected-foreground` (pressed tabs, segments, toggles) | `#15161A` / `#FFFFFF` | `#EDEEF1` / `#111215` | `#6E5BFF` / `#FFFFFF` | `#8B7BFF` / `#0B1020` |
| `--success` (money IN) | `#12955A` | `#3CCB84` | `#0F9160` | `#3DDC97` |
| `--danger` = `--destructive` (money OUT) | `#D23F3F` | `#F27272` | `#D6404E` | `#FF7C7C` |
| `--warning` | `#B66A00` | `#F0A23B` | `#A86500` | `#FFC857` |
| `--hero` / `--hero-foreground` / `--hero-muted` | `#15161A` / `#FFFFFF` / `#B9BCC6` | `#08090B` / `#EDEEF1` / `#A6A9B3` | `#151A3A` / `#EEF0FF` / `#B5BCDA` | `#070B18` / `#EAF0FF` / `#A3AECB` |
| `--hero-art` | saffron glow (see the mockup) | same | violet + cyan glow | same, stronger |
| `--glass` | `none` | `none` | `blur(18px) saturate(160%)` | `blur(18px) saturate(140%)` |
| `--glow` | `none` | `none` | `0 0 14px rgb(110 91 255/.45)` | `0 0 14px rgb(139 123 255/.7)` |
| `--bar` (floating dock and tab bar) | `rgb(255 255 255/.9)` | `rgb(27 28 33/.92)` | `rgb(255 255 255/.66)` | `rgb(22 29 51/.72)` |
| `--shadow`, `--shadow-float` | as in the mockup | as in the mockup | as in the mockup | as in the mockup |
| `--chart-1..6` | `#EDA100 #1BAF7A #2A78D6 #E87BA4 #D23F3F #0E9AA7` | `#C98500 #199E70 #3987E5 #D55181 #F27272 #2EC4CF` | `#4A3AA7 #1BAF7A #2A78D6 #E87BA4 #D6404E #C026D3` | `#9085E9 #199E70 #3987E5 #D55181 #FF7C7C #E879F9` |

The exact values are in the mockup's CSS: search for `data-pal="studio"` and `data-pal="night"`.

Rules:
- **Never put `--brand` text on a light background in Studio,** because saffron fails contrast. Use brand as a fill with `--brand-foreground` on it.
- Soft status backgrounds use `color-mix(in srgb, var(--success) 16%, transparent)`, not extra tokens.
- Keep karots' money cue: green means money in, red means money out.
- **Glass costs GPU time.** Use `--glass` on large surfaces only: cards, the dock, shelves and the tab bar. Never use it on each product tile or table row.
- Under `prefers-reduced-transparency: reduce`, set `--glass: none` and make `--card` solid.
- **Charts** (ECharts) use `--chart-1..4` as the categorical order (validated with the dataviz palette checks, 2026-10-05: adjacent pairs incl. a donut's wrap, both modes, each mode's own surface; slots 5–6 are unvalidated spares). They read `--chart-*`, `--border`, `--faint` and `--foreground` at render time, and re-render on palette or mode change.

## 3. Type

- **Figtree** (400–800) is used for all UI text. **Sora** (600–800) is used for display: page titles, the login clock, the till total, KPI values and "Give Rs …".
- Self-host both fonts with `@fontsource/figtree` and `@fontsource/sora`.
- Use `font-variant-numeric: tabular-nums` wherever money is shown.

Sizes: 12 / 13.5 / 15 (body) / 17 / 24 / 32 / 40. The login clock is 132px on desktop and 64px on a phone.

## 4. Shape and density

- **Shape is a palette value:** `--radius` / `--radius-sm` (Studio 16/11px; Tailwind's `rounded-md…4xl` scale from `--radius`), `--pill` (`rounded-pill`: chips, pill buttons, segmented controls; 9999px or the soft `var(--radius-sm)`). True circles (avatars, dots, spinners, round icon buttons) stay `rounded-full`.
- **Density:** comfortable is `--control-h` 46px, `--gap` 14px and `--pad` 16px. Compact is 38px, 10px and 12px.

## 5. Login and lock

The reference is the mockup's login frame.

- **Desktop / kiosk:** two columns.
  - **Left:** a `--hero` panel with `--hero-art`, containing:
    - the brand mark and shop name
    - a large live clock (updates every 15s)
    - the date
    - status pills: server online, counter, and last closed
    - a small tilted thermal-receipt card showing yesterday's close (`login-info` may add yesterday's totals; omit the receipt if the API doesn't give them)
  - **Right:** heading "Sign in to the counter", then the phone field, a PIN field with dots, the 3×4 keypad (Clear, 0, Delete), the Sign in button and ModeToggle (top right).
- **Phone:** the hero becomes a header strip (clock at 64px); the fields and keypad follow.
- **PIN entry:**
  - Put a real `<input>` over the dots, so a physical keyboard types into it. Enter submits.
  - The keypad writes into whichever field was last focused. Use `mousedown` `preventDefault` so focus doesn't leave the field.
  - On touch devices (`pointer: coarse`), set the PIN input's `inputMode="none"` so the keypad replaces the OS keyboard.
  - A wrong PIN shakes the box and shows a plain error, then clears the PIN.
- **No browser memory (shared counter), same as karots-pos:**
  - Set `autocomplete="off"` on the form and on both inputs, and give them non-standard `name`s.
  - The PIN is `type="text"` with `-webkit-text-security: disc`, **not** `type="password"`, so password managers never offer to save it.
  - Don't add `autocorrect` or `spellcheck`.
- **Install-ID Konami reveal:** keep it (a small line at the bottom of the form).
- **Lock** uses the same layout. The right side says "Locked by <name>", with the same PIN field, keypad, and a Sign out link.

## 6. Admin shell: the dock (no sidebar)

The reference is the mockup's desktop and phone frames.

**Desktop and tablet (768px and up):**
- **Top bar** (not sticky chrome, just the page head):
  - brand mark and shop name
  - breadcrumb (Area / Page)
  - search pill "Search pages, products, bills" with **Ctrl K**, which opens the command palette (karots-pos has one; it lists every page, plugin pages and "Open the till")
  - ModeToggle and the avatar menu (Change PIN, Lock, Sign out)
- **Page tabs:** inside an area, that area's pages show as pill tabs under the top bar.
- **Dock:** a floating `--bar` pill fixed at bottom centre, with `--glass` and `--shadow-float`.
  - **Items in order:** Home, then the areas Sell, Inventory, Purchasing, Money, Reports and Setup (karots-pos's sections, `templates/layouts/admin.templ`), a divider, **Apps**, and **Open the till** (brand fill, glow) at the end.
  - **Behaviour:** icons grow on hover and show a name tip. The current area has a dot under it. Badges (such as "Items to review: 3") come from the badge hook.
- **Shelf:** clicking an area raises a panel above the dock.
  - **Left:** a `--hero` "lead" card with the area's key figure (such as "Stock value Rs 1.84M, 12 running low").
  - **Right:** a 3-column grid of the area's pages, each with an icon, name and one-line description (karots-pos's link descriptions), plus counts.
  - **As built (M6.5, user: "bland, full of text"):** the grid is icon chip + name only, in fixed-height rows; the description of the page pointed at / focused shows in the lead card (fixed height, so the shelf never resizes), else the area's one-line blurb. Hover fills the chip with `--brand` and lifts it — no grey hover block; the current page's chip is `--selected`. The lead card's key figure is not built yet.
  - It closes on Escape or a click outside.
- **Apps:** a dock tile shaped as a 2×2 grid of plugin colours. It opens a shelf with every enabled plugin as an app card (colour, icon, name, description).
  - **Plugins never add dock icons themselves.** They register an app card, plus their pages under Apps and in the command palette, so the dock never overflows as plugins arrive (M7).
  - A plugin page shows as "Apps / Pharmacy / …" in the breadcrumb, with its own page tabs.
- **Content** gets the full width. Leave bottom padding (~110px) so the dock never covers it.

**Phone (below 768px):**
- **Top:** the brand mark, shop name, date and time, and the avatar menu (with ModeToggle).
- **Bottom:** a floating tab bar (`--bar`, `--glass`) with Home, Sell, **Till** (brand fill, centre), Inventory and More.
  - **More** raises a sheet with every other area and the plugin apps as icon tiles.
- **Home:** a hero sales card, 2×2 area tiles each with a live figure, then "Needs your attention".
- **Tables:** become card lists.

**Hidden system menu:** it's reached as in karots-pos, by tapping the user's name 8 times in the avatar menu. It's system user only and never visible to the shop.

## 7. Till

The till look follows the till frame of the first mockup, in whichever palette and mode is active. The layout is unchanged from the earlier approval:
- **Top bar:** 60px.
- **1366px and wider:** three columns, `1fr 392px 356px`.
- **Below 1200px:** two columns, `1fr 360px`, with cart and pay stacked.
- **Below 1024px:** kiosk auto-fit.
- **Top bar contents:** ModeToggle and Lock.

## 8. Motion

Use `motion/react` for:
- tile lift and press
- the cart-line flash
- a tender popping in
- the sale-complete zoom
- the dock hover grow
- shelves and sheets rising (220ms)
- the tab indicator
- KPI count-up

Keep durations ≤350ms, and turn everything off under `prefers-reduced-motion`.

**Motion is a palette value too:** `--dur-fast` (150ms, also Tailwind's default `transition`), `--dur`, `--dur-slow`,
`--ease`, `--ease-spring` and `--lift` (hover rise; 0 = calm, 2px = lively). CSS uses `duration-(--dur-fast)` /
`ease-(--ease)`; Motion uses `tx()` from `theme/motion.ts`. Only the cart-line flash and the login games keep literal
timings (marked "not a UI transition").

## 9. Not now

- The Bazaar palette and other packs. They're added later as extra palette files.
- Per-client component overrides.
- A shop-wide default mode.

## 10. Receipt styles (Image printing)

Ten looks for every slip when Settings → Printers → Print receipts as is **Image**: Classic (default), Bold, Minimal,
Ticket, Poster, Grid, Stamp, Elegant, Cards, Sections. Mockups: `docs/design/mockups/receipt-styles.html` (the
styles) and `docs/design/mockups/appearance-receipts.html` (the picker). Only the system user picks, on Appearance,
shop-wide — the preview there is the exact picture the printer gets. Paper is 1-bit: black or white only, no grey,
bundled fonts only. A style changes the look, never what a slip says. Each style is a kit in
`web/src/features/print/styles/<key>/` (header, rules, rows, total band, barcode, footer); a style may draw a whole
slip itself (`overrides`). Every style but Classic prints a Code 128 of the slip number, which the till's scan box
opens.

## 11. Label styles (Image printing)

When Settings → Printers & labels → Print labels as is **Image**, every label is a React template drawn at label dots
(8 per mm), sent as one TSPL `BITMAP`, and previewed as that same picture. Mockup:
`docs/design/mockups/label-styles.html`. A style per **label kind**, picked by the system user on Appearance → Labels:
**Price label** (core) Classic, Price Tag, Inverse, QR Tag, Dual, Ticket, Minimal, Shelf (≥ 70 mm wide, else Price
Tag); **Dose label** (Pharmacy) Classic, Clear, Boxed; **Repair ticket** (Repairs) Classic, Job. The shop's First /
Second text choices hold in every style; the price is the big one wherever it sits. Codes keep 2-dot modules and their
quiet zones, or the label draws Classic ("Too small for this style" on Appearance). Labels are paper: black and white
only (`INK`), bundled fonts, Sinhala / Tamil included. Classic labels (the printer's own fonts) stay the default.

