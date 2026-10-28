// Shared by the filled-in SAMPLE PDFs (sample.js, sample-notice.js): blue-ink
// handwriting, ticks, drawn signatures and stamps. Loaded before them.
window.K = (() => {
  const ink = (el, text) => { el.textContent = text; el.classList.add('ink') }
  const labels = (root, text) => [...root.querySelectorAll('label')].filter((l) => l.textContent.trim() === text)
  const field = (text, value, root = document, n = 0) => {
    const l = labels(root, text)[n]
    if (l) ink(l.nextElementSibling, value)
  }
  const row = (table, first, values) => {
    const tr = [...table.querySelectorAll('tr')].find((r) => r.cells[0]?.textContent.trim() === first)
    values.forEach((v, i) => { if (v != null) ink(tr.cells[i + 1], v) })
  }
  const tick = (text) => {
    for (const s of document.querySelectorAll('.boxes span, .agree td')) {
      if (s.textContent.includes('☐ ' + text) || (text === '*' && s.textContent.trim() === '☐')) {
        const w = document.createTreeWalker(s, NodeFilter.SHOW_TEXT)
        for (let t; (t = w.nextNode());) if (t.data.includes('☐')) { t.data = t.data.replace('☐', '☑'); break }
        s.classList.add('ticked')
      }
    }
  }
  const inline = (after, value) => {
    for (const s of document.querySelectorAll('.boxes span')) {
      if (s.firstChild?.textContent?.trim().startsWith(after)) { const l = s.querySelector('.line'); if (l) ink(l, value) }
    }
  }

  function signature(seed) {
    let x = 4, d = "M4 30"
    for (const c of seed) { const h = (c.charCodeAt(0) * 7) % 26; x += 14; d += ` q${6} ${-h} ${14} ${h / 2 - 4}` }
    return `<svg class="sig" viewBox="0 0 ${x + 20} 44"><path d="${d} q10 6 22 -8" fill="none" stroke="#1a3c9e" stroke-width="1.6" stroke-linecap="round"/></svg>`
  }
  function stamp(kind, date = "28.10.2026") {
    const [top, mid, col] = kind === 'karots'
      ? ['KAROTS KORP · KAROTS DEV · SAMPLE ·', 'KAROTS', '#5b2bb5']
      : ['SUNRISE MINI MART · KANDY · SAMPLE ·', 'SUNRISE', '#b3261e']
    return `<svg class="stamp" viewBox="0 0 120 120" style="color:${col}">
      <defs><path id="r-${kind}" d="M60 60 m-44 0 a44 44 0 1 1 88 0 a44 44 0 1 1 -88 0"/></defs>
      <circle cx="60" cy="60" r="56" fill="none" stroke="currentColor" stroke-width="3"/>
      <circle cx="60" cy="60" r="34" fill="none" stroke="currentColor" stroke-width="1.5"/>
      <text font-size="10.5" font-weight="700" fill="currentColor" letter-spacing="1"><textPath href="#r-${kind}">${top}</textPath></text>
      <text x="60" y="58" text-anchor="middle" font-size="13" font-weight="800" fill="currentColor">${mid}</text>
      <text x="60" y="73" text-anchor="middle" font-size="9" fill="currentColor">${date}</text></svg>`
  }
  return { ink, field, row, tick, inline, signature, stamp }
})()
