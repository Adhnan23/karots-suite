// The time machine: a shop's past being built, played back as a fast-forward —
// a sky that turns once a day, the date and the till's totals rolling on, and a
// calendar that lights up each day by how busy it was. Shared by the Demo Room's
// building screen (web/demo) and the Shop Builder (cmd/bootstrap); plain JS so
// both can load it. Colours come from the host: --tm-a, --tm-b (accents),
// --tm-cell (an empty day), --tm-fg, --tm-muted.
//
// Days arrive in batches (a week at a time); playback runs smoothly at the rate
// they come in, so the date never jumps.

const CSS = `
.tm { display: grid; gap: 12px; color: var(--tm-fg, inherit); }
.tm-sky { position: relative; height: var(--tm-sky, 54px); border-radius: 12px; overflow: hidden;
  background: linear-gradient(90deg, #0b1030 0%, #3b2a6b 14%, #f08a5d 22%, #7cc6f2 32%, #9fd8ff 50%, #7cc6f2 68%, #f0a35d 78%, #3b2a6b 86%, #0b1030 100%);
  background-size: 400% 100%; }
.tm-sun { position: absolute; top: 0; left: 0; font-size: calc(var(--tm-sky, 54px) * .42); line-height: 1; translate: -50% 0; }
.tm-head { display: flex; justify-content: space-between; align-items: baseline; gap: 12px; flex-wrap: wrap; }
.tm-date { font: 700 var(--tm-date, 26px)/1.1 ui-monospace, SFMono-Regular, Menlo, monospace; letter-spacing: .02em; font-variant-numeric: tabular-nums; }
.tm-ff { font-size: 12px; color: var(--tm-muted, inherit); font-variant-numeric: tabular-nums; white-space: nowrap; }
.tm-stats { display: flex; gap: 8px 22px; flex-wrap: wrap; font-size: 14px; color: var(--tm-muted, inherit); }
.tm-stats b { color: var(--tm-fg, inherit); font-variant-numeric: tabular-nums; font-size: 16px; margin-right: 4px; }
.tm-cal, .tm-months { display: grid; grid-template-columns: repeat(var(--cols), minmax(0, var(--tm-cell-size, 12px))); column-gap: 2px; }
.tm-cal { grid-template-rows: repeat(7, auto); grid-auto-flow: column; row-gap: 2px; }
.tm-cal i { aspect-ratio: 1; border-radius: 3px; background: var(--tm-cell, rgb(127 127 127 / .15)); transition: background-color .3s; }
.tm-cal i.pad { visibility: hidden; }
.tm-months span { font-size: 10px; color: var(--tm-muted, inherit); white-space: nowrap; overflow: visible; }
.tm-done .tm-ff { color: var(--tm-fg, inherit); }
`

const DOW = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat']
const MON = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec']
const still = () => matchMedia('(prefers-reduced-motion: reduce)').matches

function h(tag, cls, text) {
  const e = document.createElement(tag)
  if (cls) e.className = cls
  if (text) e.textContent = text
  return e
}

function parse(iso) {
  const [y, m, d] = iso.split('-').map(Number)
  return new Date(y, m - 1, d)
}

/** timeMachine builds one; add() feeds it each finished day, done() ends it. */
export function timeMachine() {
  if (!document.getElementById('tm-css')) {
    const s = h('style'); s.id = 'tm-css'; s.textContent = CSS; document.head.append(s)
  }
  const el = h('div', 'tm')
  const sky = h('div', 'tm-sky'), sun = h('span', 'tm-sun', '☀')
  sky.append(sun)
  const date = h('div', 'tm-date', '…'), ff = h('div', 'tm-ff', '▶▶ warming up')
  const head = h('div', 'tm-head'); head.append(date, ff)
  const sales = h('b', '', '0'), takings = h('b', '', 'Rs 0')
  const stats = h('div', 'tm-stats')
  const s1 = h('span'); s1.append(sales, 'sales')
  const s2 = h('span'); s2.append(takings, 'takings')
  stats.append(s1, s2)
  const months = h('div', 'tm-months'), cal = h('div', 'tm-cal')
  el.append(sky, head, stats, months, cal)

  const days = [], cumSales = [], cumTake = [], arrived = []
  let cells = [], pad = 0, shown = -1, lit = -1, max = 1, finished = '', last = 0, raf = 0

  function grid(first) {
    const start = parse(first.date)
    pad = (start.getDay() + 6) % 7 // weeks start on Monday
    const cols = Math.ceil((pad + first.of) / 7)
    el.style.setProperty('--cols', cols)
    for (let i = 0; i < pad; i++) cal.append(h('i', 'pad'))
    cells = []
    for (let i = 0; i < first.of; i++) cells.push(cal.appendChild(h('i')))
    for (let i = 0; i < first.of; i++) { // a month's name over the week its 1st falls in
      const d = new Date(start); d.setDate(d.getDate() + i)
      if (d.getDate() !== 1 && i !== 0) continue
      const col = Math.floor((pad + i) / 7)
      if (i === 0 && d.getDate() > 20) continue // too little of it to name
      const m = h('span', '', MON[d.getMonth()]); m.style.gridColumn = `${col + 1} / span 4`
      months.append(m)
    }
  }

  function paint(i) {
    const r = Math.min(1, days[i].sales / max) // busier = brighter, and leaning to --tm-a
    cells[i].style.backgroundColor = `color-mix(in srgb, var(--tm-a, #8b7bff) ${Math.round(40 + 60 * r)}%, var(--tm-b, #22d3ee))`
    cells[i].style.opacity = (0.25 + 0.75 * r).toFixed(2)
  }

  function frame(t) {
    raf = 0
    const dt = last ? Math.min(t - last, 1000) : 16
    last = t
    const target = days.length - 1
    if (shown < target) {
      // the rate days come in, or faster when behind (and when the build is done)
      const n = arrived.length, rate = n > 1 ? (n - 1) / (arrived[n - 1] - arrived[0]) : 7 / 3000
      const backlog = target - shown
      const speed = Math.max(rate, backlog / (finished ? 600 : 2500))
      shown = Math.min(target, shown + speed * dt)
      ff.textContent = `▶▶ ×${big(speed * 86_400_000)} speed`
    }
    const i = Math.floor(shown)
    if (i >= 0) {
      const d = parse(days[i].date)
      date.textContent = `${DOW[d.getDay()]} ${d.getDate()} ${MON[d.getMonth()]} ${d.getFullYear()}`
      sales.textContent = cumSales[i].toLocaleString('en-LK')
      takings.textContent = 'Rs ' + Math.round(cumTake[i]).toLocaleString('en-LK')
      for (; lit < i; ) paint(++lit)
    }
    const f = finished && shown >= target ? 0.5 : shown - Math.floor(shown) // the hour of the day shown
    if (!still()) {
      sky.style.backgroundPosition = `${f * 100}% 0`
      const up = f > 0.22 && f < 0.78
      sun.textContent = up ? '☀' : '☾'
      const x = up ? (f - 0.22) / 0.56 : ((f + 0.22) % 1) / 0.44
      sun.style.left = `${8 + 84 * x}%`
      sun.style.top = `${62 - 50 * Math.sin(Math.PI * x)}%`
    }
    if (finished && shown >= target) {
      ff.textContent = finished
      el.classList.add('tm-done')
      return
    }
    if (shown < target || !finished) raf = requestAnimationFrame(frame)
  }
  const kick = () => { if (!raf) raf = requestAnimationFrame(frame) }

  return {
    el,
    /** add takes the next finished day: { date, sales, takings, n, of }. */
    add(d) {
      if (!days.length) grid(d)
      days.push(d)
      arrived.push(performance.now())
      const k = days.length - 1
      cumSales[k] = (cumSales[k - 1] ?? 0) + d.sales
      cumTake[k] = (cumTake[k - 1] ?? 0) + Number(d.takings)
      if (d.sales > max) { max = d.sales; for (let i = 0; i <= lit; i++) paint(i) }
      kick()
    },
    /** done plays out the rest quickly, then shows text. */
    done(text) { finished = text || '✓ Done'; kick() },
    stop() { if (raf) cancelAnimationFrame(raf); raf = 0 },
  }
}

function big(n) {
  if (n >= 1e6) return (n / 1e6).toFixed(1) + 'M'
  if (n >= 1e3) return Math.round(n / 1e3) + 'k'
  return Math.round(n)
}
