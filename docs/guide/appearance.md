# Appearance

The whole shop shares one **palette**. Each screen picks light or dark for itself.

## Light and dark

The sun / screen / moon buttons in the top bar switch between light, the device's own setting and
dark. The choice is kept on that device only, so the counter PC can stay light while the owner's
phone is dark.

<p>
  <img src="../images/admin/dashboard.jpg" width="49%" alt="Light">
  <img src="../images/admin/dashboard-dark.jpg" width="49%" alt="Dark">
</p>

## Palettes

![Appearance](../images/appearance/appearance.jpg)

22 built-in palettes, each with a light and a dark side:

- **Ours:** Studio, Night, Bazaar, Lagoon, Mango, Pop, Aurora, Neumorph.
- **Well-known editor themes:** Catppuccin, Gruvbox, Nord, Dracula, Tokyo Night, Rosé Pine,
  Solarized, Everforest, Kanagawa, Synthwave '84, Monokai, Shades of Purple, Cobalt2, Ayu.

A palette sets the colours, the material (solid, frosted or glass), the corner shape and the motion.
**Duplicate** a palette to make your own and change any of it with a live preview. Your own
palettes can be exported and imported between shops.

The palette is locked for the whole shop and chosen when the suite is set up. Ask your Karots
technician to change it.

| | Light | Dark |
|---|---|---|
| Studio | ![](../images/appearance/palette-studio-light.jpg) | ![](../images/appearance/palette-studio-dark.jpg) |
| Night | ![](../images/appearance/palette-night-light.jpg) | ![](../images/appearance/palette-night-dark.jpg) |
| Bazaar | ![](../images/appearance/palette-bazaar-light.jpg) | ![](../images/appearance/palette-bazaar-dark.jpg) |
| Lagoon | ![](../images/appearance/palette-lagoon-light.jpg) | ![](../images/appearance/palette-lagoon-dark.jpg) |
| Mango | ![](../images/appearance/palette-mango-light.jpg) | ![](../images/appearance/palette-mango-dark.jpg) |
| Catppuccin | ![](../images/appearance/palette-catppuccin-light.jpg) | ![](../images/appearance/palette-catppuccin-dark.jpg) |
| Nord | ![](../images/appearance/palette-nord-light.jpg) | ![](../images/appearance/palette-nord-dark.jpg) |
| Dracula | ![](../images/appearance/palette-dracula-light.jpg) | ![](../images/appearance/palette-dracula-dark.jpg) |
| Gruvbox | ![](../images/appearance/palette-gruvbox-light.jpg) | ![](../images/appearance/palette-gruvbox-dark.jpg) |
| Synthwave '84 | ![](../images/appearance/palette-synthwave-light.jpg) | ![](../images/appearance/palette-synthwave-dark.jpg) |

## Sell page layout

![Sell page layout](../images/appearance/sell-layout.jpg)

The Sell page can be laid out differently for each screen size the shop's tills have. A shop that
watches the cart more than the product cards (keyboard and scanner cashiers) can have a narrow
products column and a wide cart; an almost square 1280 × 1024 screen can stack products over
payment and give the cart the full height.

- **Screen sizes are found, not typed.** Every till that opens the Sell page reports the room it
  really has (Windows scaling counts, so a 1920 × 1080 laptop at 125% is 1536 × 864), and the list
  shows who uses each size. Tills of the same size share one layout. A dot marks a size that has
  its own layout; a size left alone looks as it always has.
- **Arrangement:** where each section goes: the three side by side in any order, or any two stacked
  in one column with the third full height beside it, on either side (18 in all). Sizes are set by
  dragging, not by the arrangement. An arrangement too wide for a screen is greyed out.
- **Search only:** no products column. The search bar runs across the top and what's typed drops
  a list below it (**↑ ↓**, **Enter**); a scan adds straight away. The plugin menus (Combos, Print &
  Copy, Reload & Bills…) sit on the button row and open in a pop-up; **Browse** (**F1**) opens
  the groups and frequently sold in the same pop-up, and F1 or Esc closes it.
- **Start from:** Scanner counter (search only, a wide cart, larger text), Touch counter (as it
  always was) or Square screen (products over pay, cart full height) sets the whole size in one
  tap; drag from there.
- **Widths:** drag the bars between sections on the preview, drawn at the screen's real shape.
  Each section shows its width (px and %) as you drag. Payment never goes below 330 px, the cart
  below 340 px or products below 300 px; a stacked part that gets too short is marked.
- **Product cards:** Large (as before), Medium (half the height), Small (name, price and stock) or a
  **List** of rows. Frozen, low stock, "2 prices" and plugin marks stay on every size; ⓘ shows the rest.
- **Text size:** Smaller, Normal or Larger moves all the Sell page's text together, plugin panels too.
- **Try here** opens the Sell page with the unsaved layout and a banner back to Appearance, where
  the draft is waiting. It is the real Sell page: a sale made there counts. With no till open it is
  look only.
- **Reset this size** puts it back as it was; **Save layouts** saves every size at once. A till picks up
  a new layout the next time it opens the Sell page.

![A till with products as a list](../images/till/sell-list.jpg)

Like the palette, only the Karots technician can change it: ask them if your counter wants a
different shape.

Receipt and label styles are on the same page: see [Printing](printing.md#receipt-styles).
