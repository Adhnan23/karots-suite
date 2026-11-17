# Sell page layout (design)

2026-11-17. Approved mockup: `.superpowers/brainstorm/42445-1794886387/content/sell-layout-v2.html` (git-ignored).

## Why

A pharmacy shown the suite (the one that asked for the keyboard-only till) wanted the products section narrower and
the cart wider. Keyboard cashiers (scan / search, ↑ ↓, Enter) watch the cart, not the product cards, and today's cards
are large (about 6 fit on a 1366 × 768 till). Shops' screens differ: 1280 × 1024 (almost square) up to 1920 × 1080,
and a laptop at 125% scaling gives the page far less room than its resolution says.

## Goals

- Per **screen size** the shop actually has: the arrangement of the three sections, their widths (and the height
  split where two are stacked), and the product card size.
- Set by the **system user only**, in Appearance, for the whole shop (user: "if we give the control to them they make
  stuff worse"; a shop that wants a change asks Karots).
- A shop that never sets anything looks exactly as today.

## Non-goals

- No till names, no per-cashier or per-monitor layouts: same-size screens share one layout (user: only different
  sizes need different layouts).
- No free drag-and-drop of sections: pick from tested arrangements.
- Pocket Cashier's phone screen is not touched (it has its own layout).
- No setting for the shop's admin, and no per-device override.

## Anti-goals

- A layout that hides a section, makes payment too narrow to use, or breaks a key (F-keys, ↑ ↓ in search, Ctrl K).
- Anything that slows or blocks selling: a bad or missing layout falls back to today's.

## What the user decided

- **Where:** system user → Appearance, a new card "Sell page layout". Saved shop-wide like the palette.
- **Arrangements** (pictures to pick): the three side by side in any order — Products | Cart | Pay (today), Cart |
  Products | Pay, Products | Pay | Cart, Cart | Pay | Products, Pay | Cart | Products — and the stacked ones: Products
  | Cart over Pay (today's narrow), its mirror, Products over Pay | Cart full height, its mirror, Products | Pay over Cart.
- **Widths:** drag the bars between columns (and between the two stacked parts) on a live preview at the size's real
  shape. Shown in px and %. Saved as shares, not pixels.
- **Product cards:** Large (today), Medium (same contents, half the height), Small (name on up to two lines, price,
  stock, a red dot for low stock), List (one row: name, stock, price; the ↑ ↓ highlight shows on the row). Small and
  List drop the pack line; it stays in ⓘ.
- **Sizes are detected, not typed:** each till reports the room the Sell page really has (browser viewport, CSS px,
  so Windows scaling is counted). Appearance lists the sizes seen at this shop with **who uses each** ("1920 × 1080 —
  used by Shehan, Nimal"), marks **this screen** when opened on a till, and a dot on sizes that are set.
- **Reset this size** returns it to today's layout.

## Approaches considered

1. **Pick from arrangements per detected size** (chosen): each arrangement is known to work with the keys, dialogs and
   every palette; the list stays short because sizes repeat.
2. **One layout for the whole shop:** simpler, but a 1280 × 1024 and a 1920 × 1080 want different shapes, and adding
   sizes later changes how it's saved. Rejected by the user.
3. **Free drag-and-drop of sections:** produces untested layouts and much more code for the same result.
4. **A fixed list of common resolutions:** wrong under Windows scaling (a 1920 × 1080 laptop at 125% is 1536 × 864).

## Builder's calls (mine, open to the user)

- Storage: `settings.sell_layouts` jsonb, keyed `"<w>x<h>"`, each `{arr, fr[], split, card}`; validated on save
  (known arrangement, shares that sum to 1 and respect the minimums). Migration `00036_`.
- Seen sizes: the till's presence beat carries its viewport; the server keeps a small table of sizes seen (size, user
  names, last seen) so the list survives a restart. Sizes not seen for 90 days drop off the list (their layout stays).
- Matching at the till: the saved size nearest to the viewport within about ±10% each way; none → today's layout.
  The Sell page re-checks on window resize.
- Minimums (real px): Payment 330, Cart 340, Products 300; a stacked part shows a warning in the preview when too short.
- Below 1200 px wide the same rules apply; today's two built-in layouts become the defaults for sizes above / below 1200.
- The sales person's page uses the same layout (it is the same Sell page).
- Docs: `docs/guide/appearance.md` section + a screenshot of the card; `docs/guide/till.md` mentions that layouts are set by
  Karots. Playwright: a layout saved for a size is used by a till at that size; List mode with ↑ ↓ + Enter adds the
  highlighted product.
