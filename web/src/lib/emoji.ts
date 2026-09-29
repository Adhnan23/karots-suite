/** Shop-relevant emoji for till cards, by aisle, each with search words.
 * ponytail: curated list (~200), not the full Unicode set — add rows here when a shop needs one. */
export const EMOJI: [aisle: string, items: [emoji: string, words: string][]][] = [
  ['Food', [
    ['🍞', 'bread loaf bakery'], ['🥖', 'baguette bread bun'], ['🥐', 'croissant pastry bakery'], ['🧁', 'cupcake cake'], ['🎂', 'cake birthday'],
    ['🍰', 'cake slice dessert'], ['🍪', 'biscuit cookie snack'], ['🍫', 'chocolate sweet bar'], ['🍬', 'sweet candy toffee'], ['🍭', 'lollipop sweet candy'],
    ['🍩', 'donut doughnut'], ['🥨', 'pretzel snack'], ['🍿', 'popcorn snack'], ['🥜', 'nuts peanut cashew'], ['🌰', 'chestnut nut'],
    ['🍚', 'rice cooked'], ['🌾', 'rice paddy grain wheat flour'], ['🍜', 'noodles soup'], ['🍝', 'pasta spaghetti'], ['🥫', 'tin can canned'],
    ['🥚', 'egg eggs'], ['🧀', 'cheese dairy'], ['🧈', 'butter margarine'], ['🥓', 'bacon meat'], ['🍗', 'chicken meat'],
    ['🥩', 'meat beef steak'], ['🐟', 'fish seafood'], ['🦐', 'prawn shrimp'], ['🍤', 'fried prawn'], ['🍔', 'burger'],
    ['🌭', 'hot dog sausage'], ['🥪', 'sandwich'], ['🍕', 'pizza'], ['🌮', 'taco wrap'], ['🥟', 'dumpling samosa'],
    ['🍛', 'curry rice'], ['🍲', 'pot stew soup'], ['🥗', 'salad'], ['🍦', 'ice cream'], ['🍨', 'ice cream cup'],
    ['🧂', 'salt'], ['🌶️', 'chilli pepper spice'], ['🧄', 'garlic'], ['🧅', 'onion'], ['🫚', 'ginger'],
    ['🍯', 'honey jam'], ['🥣', 'cereal bowl'], ['🫘', 'beans dhal lentils'], ['🥥', 'coconut'], ['🍌', 'banana'],
    ['🍎', 'apple fruit'], ['🍊', 'orange fruit'], ['🍋', 'lemon lime'], ['🍇', 'grapes fruit'], ['🍉', 'watermelon'],
    ['🍍', 'pineapple'], ['🥭', 'mango'], ['🍓', 'strawberry'], ['🍐', 'pear'], ['🥑', 'avocado'],
    ['🥕', 'carrot vegetable'], ['🥔', 'potato vegetable'], ['🍅', 'tomato'], ['🥒', 'cucumber'], ['🥬', 'leaves greens cabbage'],
    ['🥦', 'broccoli vegetable'], ['🌽', 'corn maize'], ['🍆', 'brinjal eggplant'], ['🫑', 'capsicum pepper'], ['🍄', 'mushroom'],
  ]],
  ['Drinks', [
    ['🥛', 'milk dairy'], ['🍼', 'baby milk bottle feeding'], ['☕', 'coffee tea hot'], ['🍵', 'tea green'], ['🧋', 'bubble tea milk tea'],
    ['🥤', 'soda soft drink cola cup'], ['🧃', 'juice box drink'], ['🧉', 'drink mate'], ['💧', 'water'], ['🚰', 'water tap'],
    ['🍶', 'bottle yoghurt curd'], ['🍺', 'beer'], ['🍷', 'wine'], ['🥃', 'whisky arrack liquor'], ['🧊', 'ice cube'],
  ]],
  ['Home', [
    ['🧼', 'soap bar wash'], ['🧽', 'sponge scrub'], ['🧴', 'lotion shampoo bottle'], ['🧻', 'tissue toilet roll paper'], ['🪥', 'toothbrush'],
    ['🧹', 'broom sweep'], ['🧺', 'basket laundry'], ['🪣', 'bucket'], ['🧯', 'extinguisher'], ['🕯️', 'candle'],
    ['💡', 'bulb light'], ['🔦', 'torch flashlight'], ['🔋', 'battery'], ['🔌', 'plug adapter'], ['🪫', 'battery low'],
    ['🧷', 'safety pin'], ['🪡', 'needle sewing'], ['🧵', 'thread'], ['🧶', 'yarn wool'], ['🔑', 'key'],
    ['🔒', 'lock padlock'], ['🔨', 'hammer tool'], ['🪛', 'screwdriver'], ['🔧', 'spanner wrench'], ['🪚', 'saw'],
    ['🪜', 'ladder'], ['🧰', 'toolbox'], ['🪝', 'hook'], ['🧲', 'magnet'], ['🪤', 'trap mouse'],
    ['🍽️', 'plate cutlery'], ['🥄', 'spoon'], ['🍴', 'fork knife'], ['🔪', 'knife'], ['🫖', 'teapot kettle'],
    ['🍳', 'pan frying'], ['🥢', 'chopsticks'], ['🪴', 'plant pot'], ['🌸', 'flower'], ['🎁', 'gift present'],
    ['🛍️', 'bag shopping'], ['📦', 'box parcel package'], ['🗑️', 'bin trash'], ['🪟', 'window'], ['🛏️', 'bed'],
  ]],
  ['Care', [
    ['💊', 'pill tablet medicine'], ['💉', 'syringe injection'], ['🩹', 'plaster bandage'], ['🩺', 'stethoscope'], ['🌡️', 'thermometer'],
    ['🧪', 'test tube lab'], ['😷', 'mask face'], ['🧤', 'gloves'], ['💄', 'lipstick makeup'], ['💅', 'nail polish'],
    ['🪒', 'razor shave'], ['🪮', 'comb hair'], ['🧖', 'spa'], ['👶', 'baby'], ['🧷', 'nappy diaper pin'],
  ]],
  ['School & office', [
    ['✏️', 'pencil'], ['🖊️', 'pen'], ['🖋️', 'fountain pen ink'], ['🖍️', 'crayon'], ['🖌️', 'paint brush'],
    ['📏', 'ruler'], ['📐', 'set square'], ['✂️', 'scissors'], ['📎', 'paper clip'], ['📌', 'pin'],
    ['📓', 'notebook book'], ['📒', 'ledger book'], ['📔', 'exercise book'], ['📚', 'books'], ['📄', 'paper page sheet'],
    ['📃', 'document'], ['🗂️', 'file folder'], ['📁', 'folder'], ['✉️', 'envelope letter'], ['🏷️', 'label tag price'],
    ['🖨️', 'printer photocopy print'], ['📠', 'fax copy'], ['🎒', 'school bag backpack'], ['🧮', 'abacus calculator'], ['🗒️', 'notepad'],
  ]],
  ['Tech', [
    ['📱', 'phone mobile'], ['📲', 'reload top up phone'], ['☎️', 'telephone'], ['💻', 'laptop computer'], ['🖥️', 'desktop computer monitor'],
    ['⌨️', 'keyboard'], ['🖱️', 'mouse'], ['🎧', 'headphones earphones'], ['🔊', 'speaker'], ['📷', 'camera'],
    ['📺', 'tv television'], ['📻', 'radio'], ['💾', 'disk'], ['💿', 'cd dvd'], ['🕹️', 'game'],
    ['⌚', 'watch'], ['⏰', 'alarm clock'], ['📡', 'dish antenna'], ['🔗', 'cable link'], ['🛜', 'wifi router'],
  ]],
  ['Clothes & fun', [
    ['👕', 'tshirt shirt'], ['👖', 'jeans trousers'], ['👗', 'dress frock'], ['🧦', 'socks'], ['🩴', 'slippers flip flop'],
    ['👟', 'shoes sneakers'], ['👞', 'shoes'], ['🧢', 'cap hat'], ['👜', 'handbag'], ['🕶️', 'sunglasses'],
    ['☂️', 'umbrella'], ['🧸', 'toy teddy'], ['⚽', 'football ball'], ['🏏', 'cricket bat'], ['🎈', 'balloon party'],
    ['🎉', 'party'], ['🎀', 'ribbon'], ['🃏', 'cards'], ['🎨', 'art paint'], ['🪁', 'kite'],
  ]],
  ['Other', [
    ['⭐', 'star favourite'], ['🔥', 'hot fire deal'], ['✨', 'new sparkle'], ['💯', 'hundred'], ['💰', 'money bag'],
    ['💵', 'cash money note'], ['💳', 'card'], ['🧾', 'receipt bill'], ['🚚', 'delivery truck'], ['🛵', 'bike scooter'],
    ['⛽', 'fuel petrol gas'], ['🚗', 'car'], ['🐶', 'dog pet'], ['🐱', 'cat pet'], ['🐔', 'chicken hen'],
    ['❤️', 'heart'], ['✅', 'check tick'], ['🟢', 'green'], ['🔴', 'red'], ['🔵', 'blue'],
  ]],
]

/** Emoji whose search words (or the emoji itself) match every typed word. */
export function searchEmoji(q: string): string[] {
  const words = q.toLowerCase().split(/\s+/).filter(Boolean)
  const out: string[] = []
  for (const [aisle, items] of EMOJI)
    for (const [e, w] of items) {
      const hay = `${w} ${aisle.toLowerCase()}`
      if (!out.includes(e) && words.every((x) => e === x || hay.split(' ').some((t) => t.startsWith(x)))) out.push(e)
    }
  return out
}
