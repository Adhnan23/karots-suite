// The Demo Room's chrome over the real app: the watermark and its buttons (Demo
// moves, Prints, Pause, Switch shop, Exit), the demo logins on the sign-in
// screens, the paper strip the printers feed, and the moves menu (also on
// Ctrl+Shift+D, or five quick taps on the watermark).
import './overlay.css'
import { call, el, state, type Login, type Print, type State } from './api'

const root = el('div', { class: 'ksd', 'data-ksd': '' })
let st: State | null = null

function sideOfLogins(): HTMLElement | null {
  if (!st?.shop || !/^\/(login|lock)/.test(location.pathname)) return null
  const rows = st.shop.logins.map((l) => {
    const b = el('button', { type: 'button', title: `Sign in as ${l.name}` }, el('b', {}, l.name), el('span', {}, `${roleName(l.role)}, ${l.phone}, PIN ${l.pin}`))
    b.addEventListener('click', () => void signIn(l))
    return el('li', {}, b)
  })
  return el('aside', { class: 'ksd-logins', 'aria-label': 'Demo logins' }, el('h2', {}, 'Demo logins — tap to sign in'), el('ul', {}, ...rows))
}

const roleName = (r: string) => ({ admin: 'Owner', manager: 'Manager', cashier: 'Cashier' })[r] ?? r

/** The mark that can't be dismissed: a copy left on a customer's PC still points home. */
function watermark(): HTMLElement {
  const owner = st?.owner
  const mark = el('button', { class: 'ksd-mark', type: 'button', title: 'Karots Demo Room' },
    el('b', {}, 'Demo shop'), el('span', {}, `Karots Business Suite — ${owner?.name ?? 'Karots'}, ${owner?.phone ?? ''}`))
  let taps: number[] = []
  mark.addEventListener('click', () => {
    const now = Date.now()
    taps = [...taps.filter((t) => now - t < 3000), now]
    if (taps.length >= 5) {
      taps = []
      openCheats()
    }
  })
  const pause = el('button', { class: 'ksd-btn', type: 'button' }, st?.paused ? 'Resume' : 'Pause')
  pause.addEventListener('click', async () => {
    const on = pause.textContent === 'Pause'
    await call('/__demo/pause', { on }).catch(() => {})
    pause.textContent = on ? 'Resume' : 'Pause'
  })
  const sw = el('button', { class: 'ksd-btn', type: 'button' }, 'Switch shop')
  sw.addEventListener('click', async () => {
    if (!confirm('Switch shop? What you did in this one won’t be kept.')) return
    await call('/__demo/switch', {}).catch(() => {})
    location.replace('/')
  })
  const moves = el('button', { class: 'ksd-btn', type: 'button' }, 'Demo moves & logins')
  moves.addEventListener('click', openCheats)
  const prints = el('button', { class: 'ksd-btn', type: 'button' }, 'Prints')
  prints.addEventListener('click', () => void openPrints())
  const exit = el('button', { class: 'ksd-btn', type: 'button' }, 'Exit demo')
  exit.addEventListener('click', () => void exitDemo())
  const bar = el('div', { class: 'ksd-bar' }, mark, el('div', { class: 'ksd-fly' }, moves, prints, pause, sw, exit))
  mark.addEventListener('click', () => bar.classList.toggle('open')) // touch screens: a tap shows the buttons
  return bar
}

// ---- paper strip ----
const seen = { receipt: 0, label: 0 }
const tray = el('div', { class: 'ksd-tray', 'aria-live': 'polite' })

export function paper(p: Print): HTMLElement {
  const close = el('button', { class: 'ksd-x', type: 'button', 'aria-label': 'Put it away' }, '×')
  const body = p.text?.length
    ? el('pre', {}, p.text.join('\n'))
    : el('img', { src: `/__demo/print/${p.kind}/${p.id}`, alt: p.kind === 'label' ? 'Printed label' : 'Printed receipt' })
  const slip = el('figure', { class: `ksd-paper ${p.kind}` }, close, body)
  if (p.kick) slip.append(el('figcaption', {}, 'Cash drawer opened'))
  if (p.copies > 1) slip.append(el('figcaption', {}, `× ${p.copies} copies`))
  close.addEventListener('click', (e) => {
    e.stopPropagation()
    slip.remove()
  })
  slip.addEventListener('click', () => {
    const zoom = !slip.classList.contains('zoom')
    slip.classList.toggle('zoom', zoom)
    slip.classList.toggle('tucked', !zoom)
  })
  setTimeout(() => slip.isConnected && !slip.closest('.ksd-shelf') && !slip.classList.contains('zoom') && slip.classList.add('tucked'), 12000)
  return slip
}

let polling = false
let primed = false // the first answer only notes what printed before this page loaded

export async function pollPrints() {
  if (polling) return // a slow answer must not show a slip twice
  polling = true
  try {
    await poll()
  } finally {
    polling = false
  }
}

async function poll() {
  const r = await fetch(`/__demo/prints?receipt=${seen.receipt}&label=${seen.label}`).catch(() => null)
  if (!r?.ok) return
  const got = (await r.json()) as Record<'receipt' | 'label', Print[] | null>
  for (const kind of ['receipt', 'label'] as const) {
    for (const p of got[kind] ?? []) {
      if (p.id <= seen[kind]) continue // already on the strip
      seen[kind] = p.id
      if (!primed) continue
      tray.prepend(paper(p))
    }
  }
  primed = true
  while (tray.children.length > 5) tray.lastElementChild!.remove()
}

// ---- every print so far, newest first ----
export async function openPrints() {
  if (document.querySelector('.ksd-cheats')) return
  const r = await fetch('/__demo/prints?receipt=0&label=0').catch(() => null)
  const got = (r?.ok ? await r.json() : {}) as Record<'receipt' | 'label', Print[] | null>
  const all = [...(got.receipt ?? []), ...(got.label ?? [])].sort((a, b) => b.at.localeCompare(a.at))
  const shelf = el('div', { class: 'ksd-shelf' })
  for (const p of all) {
    const s = paper(p)
    s.classList.remove('tucked')
    const time = new Date(p.at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    s.prepend(el('figcaption', { class: 'ksd-when' }, `${p.kind === 'label' ? 'Label' : 'Receipt'} · ${time}`))
    s.querySelector('.ksd-x')?.remove()
    shelf.append(s)
  }
  const dialog = el('div', { class: 'ksd-cheats', role: 'dialog', 'aria-label': 'Prints' },
    el('div', { class: 'ksd-card wide' }, el('h2', {}, 'Prints'),
      all.length ? shelf : el('p', { class: 'ksd-note' }, 'Nothing printed yet — ring up a sale at the till.')))
  dialog.addEventListener('click', (e) => e.target === dialog && dialog.remove())
  dialog.addEventListener('keydown', (e) => e.key === 'Escape' && dialog.remove())
  root.append(dialog)
}

async function exitDemo() {
  if (!confirm('Close the Demo Room? Everything in it is thrown away.')) return
  await call('/__demo/exit', {}).catch(() => {})
  document.body.replaceChildren(el('main', { class: 'ksd-closed' }, el('h1', {}, 'The Demo Room is closed'),
    el('p', {}, 'Nothing was kept. You can close this tab.')))
}

// ---- cheat menu ----
export function openCheats() {
  if (!st || document.querySelector('.ksd-cheats')) return
  const list = el('div', { class: 'ksd-list' })
  const groups = new Map<string, HTMLElement>()
  const group = (name: string) => {
    if (!groups.has(name)) {
      const g = el('section', {}, el('h3', {}, name))
      groups.set(name, g)
      list.append(g)
    }
    return groups.get(name)!
  }
  const note = el('p', { class: 'ksd-note', role: 'status' })
  const dialog = el('div', { class: 'ksd-cheats', role: 'dialog', 'aria-label': 'Demo moves' },
    el('div', { class: 'ksd-card' }, el('h2', {}, 'Demo moves'), list, note))
  const shut = () => dialog.remove()
  dialog.addEventListener('click', (e) => e.target === dialog && shut())
  dialog.addEventListener('keydown', (e) => e.key === 'Escape' && shut())
  for (const l of st.shop?.logins ?? []) {
    const b = el('button', { type: 'button' }, `Sign in as ${l.name} (${roleName(l.role)})`)
    b.addEventListener('click', () => void signIn(l))
    group('Sign in').append(b)
  }
  for (const c of st.cheats ?? []) {
    const b = el('button', { type: 'button' }, c.label)
    b.addEventListener('click', async () => {
      note.textContent = `${c.label}…`
      try {
        await call(`/__demo/cheat/${encodeURIComponent(c.name)}`, {})
        note.textContent = `Done: ${c.label}`
        if (c.name.startsWith('cycle-') || c.name === 'reset') setTimeout(() => location.reload(), 400)
      } catch (e) {
        note.textContent = (e as Error).message
      }
    })
    group(c.group).append(b)
  }
  root.append(dialog)
  dialog.querySelector('button')?.focus()
}

async function signIn(l: Login) {
  const device = localStorage.getItem('ks-device') ?? ''
  await fetch('/api/v1/auth/logout', { method: 'POST', headers: { 'X-Ks-Device': device } }).catch(() => {})
  const r = await fetch('/api/v1/auth/login', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Ks-Device': device },
    body: JSON.stringify({ phone: l.phone, pin: l.pin }) })
  if (r.ok) location.replace(l.role === 'cashier' ? '/till' : '/admin')
}

// ---- mount ----
function render() {
  root.replaceChildren(watermark(), tray, ...[sideOfLogins()].filter((x): x is HTMLElement => !!x))
}

export async function mount() {
  st = await state().catch(() => null)
  if (!st) return
  document.body.append(root)
  render()
  let path = location.pathname
  setInterval(() => {
    if (location.pathname !== path) { // the SPA navigated: the logins panel follows
      path = location.pathname
      render()
    }
    void pollPrints()
  }, 1000)
  window.addEventListener('keydown', (e) => {
    if (e.ctrlKey && e.shiftKey && e.key.toLowerCase() === 'd') {
      e.preventDefault()
      e.stopImmediatePropagation()
      openCheats()
    }
  }, true)
}

if (!import.meta.env?.VITEST) void mount()
