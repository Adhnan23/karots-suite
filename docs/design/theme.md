# Theme — "Counter" (approved 2026-09-27)

The one theme for M0–M8. It has two modes that are the same design in two lights:

- **Light mode = "Studio"**: white cards on cool grey, saffron brand, calm and premium.
- **Dark mode = "Night Counter"**: deep navy with a violet→cyan glow and glass panels.

Reference mockup (clickable, approved): https://claude.ai/artifact/GkJjnypEes4GmHiHMDzFT5.
Switch it between "Night Counter" and "Studio" to see both modes.

**The mockup is a look demo, not a feature spec.** It shows the style of the till and
dashboard with fake data. The real screens implement every karots-pos feature
(`docs/parity.md`) in this style, milestone by milestone. Don't copy its data,
its missing features, or its plain-JS structure.

The third direction, "Bazaar" (per-category colours and icons, cart and pay stacked),
is **not** built now. It and other looks become theme packs later (spec §6.4). Keep
every colour in tokens so a pack can replace them.

## 1. Light and dark mode

- Every screen supports both modes: login, lock, admin (desktop and phone), till,
  and system pages. There is no screen that only works in one mode.
- **Each device chooses its own mode:** System (the default, follows the OS),
  Light, or Dark. This is a per-viewer convenience, not a shop setting. It's stored in
  `localStorage` under `ks-mode`; wrap every read and write in try/catch, and fall back
  to System.
- A `ModeToggle` (sun / moon / monitor) appears on the login page (top right), in the
  admin header user menu, and in the till top bar.
- The mode is applied as `data-theme="light"|"dark"` on `<html>`. It's absent for System,
  and then `prefers-color-scheme` decides. An inline script in `index.html` sets it
  **before first paint** so there's no flash.
- The brand colour, radius, density and receipt style stay **system-user only**
  (hidden system menu → Appearance), as the spec says. A system brand colour overrides
  the brand in **both** modes.

## 2. Palette tokens

These go in `web/src/theme/tokens.css`. Names follow shadcn so its components work
unchanged, plus a few of our own. **Warning:** shadcn's `--accent` means a hover/neutral
background, NOT the brand. The brand is `--brand` (and `--primary` points at it).

| Token | Light (Studio) | Dark (Night) | Use |
|---|---|---|---|
| `--background` | `#EFF0F3` | `#0B1020` | page |
| `--bg-art` | `none` | `radial-gradient(900px 500px at 12% -10%, rgb(124 92 255 / .35), transparent 60%), radial-gradient(700px 500px at 110% 110%, rgb(34 211 238 / .22), transparent 60%)` | page glow, on `body` |
| `--foreground` | `#15161A` | `#EAF0FF` | main text |
| `--card` / `--card-foreground` | `#FFFFFF` / fg | `rgb(255 255 255 / .055)` / fg | panels, cards |
| `--popover` / `--popover-foreground` | `#FFFFFF` / fg | `#161D33` / fg | menus and dialogs, always **solid** |
| `--muted` / `--secondary` / `--accent` | `#F6F7F9` | `rgb(255 255 255 / .085)` | inputs, secondary buttons, hover |
| `--muted-foreground` | `#5A5E69` | `#A3AECB` | secondary text |
| `--faint` | `#6E727D` | `#7A84A6` | axis labels, counts, hints (≥4.5:1 on card) |
| `--border` / `--input` | `#E4E6EB` | `rgb(255 255 255 / .10)` | lines |
| `--brand` | `#F2A900` | `#8B7BFF` | primary buttons, selected state, chart series 1 |
| `--brand-foreground` | `#15161A` | `#0B1020` | text on brand |
| `--brand-soft` | `#FFF4D6` | `rgb(139 123 255 / .18)` | selected backgrounds, flashes |
| `--brand-2` | `#15161A` | `#2FD5E6` | second accent (phone tab active, glow) |
| `--primary` / `--primary-foreground` / `--ring` | `var(--brand)` / `var(--brand-foreground)` / `var(--brand)` | same | shadcn wiring |
| `--success` (money IN) | `#12955A` | `#3DDC97` | paid, change, deltas up, ↓ into drawer |
| `--danger` = `--destructive` (money OUT) | `#D23F3F` | `#FF7C7C` | remove, deltas down, ↑ out of drawer |
| `--warning` | `#B66A00` | `#FFC857` | still to pay, expiry, attention |
| `--shadow` | `0 1px 2px rgb(21 22 26 / .06), 0 10px 28px rgb(21 22 26 / .07)` | `inset 0 1px 0 rgb(255 255 255 / .06), 0 10px 30px rgb(0 0 0 / .35)` | cards |
| `--glass` | `none` | `blur(18px) saturate(140%)` | `backdrop-filter` on panels |

Rules:

- **Never put brand-coloured text on a light background.** Saffron fails contrast there.
  In light mode, brand is a fill with `--brand-foreground` on top. Use `--foreground`
  for text that must stand out.
- Soft status backgrounds use `color-mix(in srgb, var(--success) 16%, transparent)`,
  and likewise for warning and danger. Don't add separate tokens for them.
- Keep karots' money-direction cue: green means money in, red means money out.
- **Glass costs GPU time.** Apply `--glass` to big panels (cart, pay, cards, sidebar)
  only, and never to each product tile or table row, because counter PCs are weak. Under
  `@media (prefers-reduced-transparency: reduce)`, set `--glass: none` and make
  `--card: #121932`.

### Charts (ECharts)

Charts read tokens at render time (`getComputedStyle`) and re-render when the mode
changes. They never hard-code colours.

| Series | Light | Dark |
|---|---|---|
| This period | `#D98E00` line, `--brand` area at 14% | `--brand` line + area 14% |
| Previous period | `--faint`, dashed 4 4 | `--faint`, dashed 4 4 |
| Categorical 1–6 | `#D98E00 #2F6FEB #12955A #7C5CFF #D23F3F #0E9AA7` | `#8B7BFF #2FD5E6 #3DDC97 #FFC857 #FF7C7C #E879F9` |

Chart styling:

- Gridlines use `--border`; tooltips are `--foreground` on `--background` (inverted).
- There's a hover crosshair.
- The range is a segmented control: 7 / 14 / 30 / 90 days.

## 3. Type

- **Figtree** (400–800) is used for all UI text. **Sora** (600–800) is used for display:
  page titles, the till total, KPI values and the sale-complete change amount.
- Self-host both for offline use with `@fontsource/figtree` and `@fontsource/sora`.
  Remove the scaffold's Geist font. Set `--font-sans: "Figtree", system-ui, sans-serif`
  and `--font-display: "Sora", var(--font-sans)`.
- Use `font-variant-numeric: tabular-nums` everywhere money is shown.

| Step | Size / line-height | Weight | Where |
|---|---|---|---|
| xs | 12 / 16 | 600 | chips, badges, meta |
| sm | 13.5 / 20 | 500 | tables, secondary |
| base | 15 / 22 | 400–600 | body, buttons |
| lg | 17 / 24 | 700 | card titles, brand |
| xl | 24 / 30 | 800, display, −0.02em | page titles, KPI values |
| 2xl | 32 / 36 | 800, display, −0.02em | till total |
| 3xl | 40 / 44 | 800, display | "Give Rs …" on sale complete |

## 4. Radius (system-selectable)

| `data-radius` | `--radius` (cards) | `--radius-sm` (buttons, tiles, inputs) |
|---|---|---|
| `sharp` | 6px | 4px |
| `rounded` (default) | 16px | 11px |
| `round` | 22px | 14px |

Chips, pills, quantity steppers and segmented controls are always `999px`.

## 5. Density (system-selectable)

| `data-density` | `--control-h` | `--gap` | `--pad` (card) | till tile min-height |
|---|---|---|---|---|
| `comfortable` (default) | 46px | 14px | 16px | 98px |
| `compact` | 38px | 10px | 12px | 80px |

## 6. Layouts

### Till (desktop and kiosk only)

- **Top bar (60px):**
  - brand mark and shop name
  - cashier and counter
  - a drawer pill (green dot, "Till open · float Rs …")
  - hotkey hints
  - ModeToggle and Lock
- **At 1366×768 and wider:** three columns, `1fr 392px 356px`, with a gap of `--gap`.
  1. **Catalog:** search (F2) with groups as pills, a "Frequently sold" default and a
     tile grid (`minmax(160px,1fr)`).
  2. **Cart:** lines with a qty stepper, a per-line discount chip, an expiry badge and
     remove. Held sales sit at the bottom.
  3. **Pay:** customer picker (balance, advance and limit shown), method buttons, amount
     with Fill/Add, quick cash, tender list, summary with bill discount, then the
     due / change / keep banner, Complete, Hold and New sale.
- **Below 1200px (e.g. 1024×768):** two columns, `1fr 360px`. The cart and pay panels
  stack in the right column, and pay becomes collapsible above Complete.
- Under 1024 the kiosk auto-fit scale takes over (Task 12).

### Admin (one responsive admin, desktop AND phone)

- **1024px and wider:** a 232px sidebar (brand, nav, and "Open the till" pinned at the
  bottom) plus the main area. The main area has a page header (title, date, range
  control) and a card grid.
- **768–1023px:** the sidebar collapses to a 72px icon rail with tooltips.
- **Below 768px (the phone, designed at 360):**
  - a top header (shop name and avatar menu with ModeToggle)
  - a bottom tab bar: Home, Reports, Stock, Money, More, with a sliding indicator
  - KPI rows become a horizontal swipe (`scroll-snap`)
  - tables become card lists
- **Dashboard cards:**
  - KPIs with a delta vs yesterday and a sparkline
  - a trend chart (this period vs previous)
  - payment mix bars
  - top sellers
  - a "Needs your attention" list with severity chips (warn / crit / info)
- These are the look. M5 decides the real card set from karots-pos's dashboard.

### Login and lock

- A centred card on `--background` (dark mode shows the glow).
- The card holds the brand mark, shop name, phone + PIN, and the install-ID Konami reveal.
- ModeToggle sits top right. Lock uses the same card with the user's avatar.

## 7. Motion

Use **Motion** (`motion/react`) for:

- tile hover lift (2px)
- press scale (0.97)
- a new/changed cart line flashing `--brand-soft` (0.7s)
- a tender line popping in
- the sale-complete card zooming in (spring)
- the tab indicator sliding
- a number count-up on KPI values

Keep durations at or under 350ms. Everything is off under `prefers-reduced-motion`.

## 8. Not now

- Theme packs such as Bazaar, per-client component overrides, and a shop-wide default
  mode. Per-device mode covers it; add a system default if a shop asks.
