import './base.css'
import './pages.css'
import { call, el, state, type Shop, type State } from './api'
import { swatch } from './swatch'

const app = document.querySelector<HTMLElement>('#app')!
const PLUGINS = ['pharmacy', 'repairs', 'recharge', 'documents', 'combos', 'alternatives', 'productplus', 'requests', 'clearance', 'aicatalog']
const NAMES: Record<string, string> = {
  pharmacy: 'Pharmacy', repairs: 'Repairs', recharge: 'Reload & bills', documents: 'Print & copy', combos: 'Combos',
  alternatives: 'Alternatives', productplus: 'Product fields', requests: 'Requests', clearance: 'Clearance', aicatalog: 'AI assistant',
}
const PALETTES = ['studio', 'night', 'bazaar', 'lagoon', 'catppuccin', 'gruvbox', 'nord', 'dracula', 'tokyonight', 'rosepine', 'solarized',
  'everforest', 'kanagawa', 'synthwave', 'monokai', 'shadesofpurple', 'cobalt2', 'ayu', 'mango', 'pop', 'aurora', 'neumorph']
const STYLES = ['classic', 'bold', 'minimal', 'ticket', 'poster', 'grid', 'stamp', 'elegant', 'cards', 'sections']

/** A card wearing its shop's palette: the band is the look the customer gets. */
function card(s: Shop) {
  const w = swatch(s.palette)
  const band = el('div', { class: 'band' }, el('h2', {}, s.name))
  if (w) band.setAttribute('style', `background:${w.hero || w.brand};color:${w.background}`)
  const chips = el('ul', { class: 'chips' }, ...(s.plugins?.length ? s.plugins : ['Core only']).map((p) => el('li', {}, NAMES[p] ?? p)))
  const btn = el('button', { class: 'shop', type: 'button' }, band, el('p', {}, s.blurb), chips)
  if (w) {
    const dots = el('span', { class: 'dots', 'aria-hidden': 'true' })
    for (const c of [w.brand, w.brand2, w.card]) dots.append(el('i', { style: `background:${c}` }))
    band.append(dots)
  }
  btn.addEventListener('click', () => start({ shop: s.key }))
  return btn
}

function own() {
  const form = el('form', { class: 'own' })
  const boxes = el('fieldset', {}, el('legend', {}, 'Plugins'),
    ...PLUGINS.map((k) => el('label', {}, el('input', { type: 'checkbox', name: 'p', value: k }), NAMES[k])))
  const pal = el('select', { name: 'palette', 'aria-label': 'Palette' }, ...PALETTES.map((p) => el('option', { value: p }, p)))
  const sty = el('select', { name: 'style', 'aria-label': 'Receipt style' }, ...STYLES.map((p) => el('option', { value: p }, p)))
  form.append(el('h2', {}, 'Make your own'), el('p', { class: 'muted' }, 'Pick the plugins and looks. It takes a little longer: its past is built while you wait.'),
    boxes, el('div', { class: 'row' }, el('label', {}, 'Palette ', pal), el('label', {}, 'Receipt ', sty)), el('button', { class: 'btn', type: 'submit' }, 'Open this shop'))
  form.addEventListener('submit', (e) => {
    e.preventDefault()
    const plugins = [...form.querySelectorAll<HTMLInputElement>('input[name=p]:checked')].map((i) => i.value)
    start({ custom: { plugins, palette: pal.value, receiptStyle: sty.value } })
  })
  return form
}

async function start(body: unknown) {
  try {
    await call('/__demo/start', body)
    void poll()
  } catch (e) {
    alert((e as Error).message)
  }
}

function picker(st: State) {
  app.replaceChildren(
    el('header', {}, el('h1', {}, 'Which shop shall we open?'),
      el('p', { class: 'muted' }, 'Each one is a real Karots shop with a year behind it and staff at work. Nothing you do is kept.')),
    el('section', { class: 'grid' }, ...st.shops.map(card), own()),
  )
}

function building(st: State) {
  const w = st.shop ? swatch(st.shop.palette) : null
  const bar = el('div', { class: 'bar' }, el('i', { style: `width:${st.progress}%` }))
  const head = el('div', { class: 'band' }, el('h2', {}, st.shop?.name ?? 'Your shop'))
  if (w) head.setAttribute('style', `background:${w.hero || w.brand};color:${w.background}`)
  const failed = st.phase === 'failed'
  app.replaceChildren(el('section', { class: 'building', 'aria-live': 'polite' }, head,
    el('p', {}, failed ? `The shop didn't open: ${st.error}` : `${st.step || 'Opening the doors'}…`),
    failed ? el('button', { class: 'btn quiet', type: 'button', id: 'back' }, 'Pick another shop') : bar))
  app.querySelector('#back')?.addEventListener('click', () => call('/__demo/switch', {}).then(poll))
}

async function poll() {
  const st = await state()
  if (st.phase === 'ready') return location.replace('/login')
  if (st.phase === 'picker') return picker(st)
  building(st)
  if (st.phase === 'building') setTimeout(poll, 600)
}

void poll()
