// Prints what a shop signs, as A4 PDFs in docs/legal/: the agreement (the Order
// Form with blanks to fill in by hand, then the terms read from LICENSE) and the
// Shop Rules & Service Guide it brings in.   make agreement
import { chromium } from '@playwright/test'
import { readFileSync, writeFileSync, mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

const root = resolve(import.meta.dir, '../..')
const dir = join(root, 'docs/legal/agreement')
const esc = (s: string) => s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')

// LICENSE is plain text: a paragraph starts after a blank line or at a clause
// mark at the margin (2.1, 3., (a), a "defined term"); indented and other lines
// continue it.
const starts = /^(\d+\.\d*\s|\([a-z]\)\s|"|Breaking |Copyright |[A-Z]+ SCHEDULE )/
function terms(text: string) {
  const blocks: string[] = []
  for (const line of text.split('\n')) {
    const last = blocks.length - 1
    if (!line.trim()) blocks.push('')
    else if (last >= 0 && blocks[last] && (/^\s/.test(line) || !starts.test(line))) blocks[last] += ' ' + line.trim()
    else blocks.push(line.trim())
  }
  let draft = ''
  const html = blocks.filter(Boolean).map((b, i) => {
    if (i === 0 || b.startsWith('These terms are the body')) return '' // the agreement's opening says both
    if (b.startsWith('Copyright')) return `<p class="copy">${esc(b)}</p>`
    if (b.startsWith('DRAFT')) { draft = b; return '' }
    if (/^[A-Z]+ SCHEDULE /.test(b)) return `<h2 class="schedule">${esc(b)}</h2>`
    if (/^\d+\. [A-Z ,]+$/.test(b)) return `<h2>${esc(b)}</h2>`
    if (/^\d+\.\d+ /.test(b)) return `<p class="clause">${esc(b)}</p>`
    if (/^\d+\. /.test(b)) return `<p class="clause">${esc(b)}</p>` // a schedule's items
    if (/^\([a-z]\) /.test(b)) return `<p class="item">${esc(b)}</p>`
    return `<p>${esc(b)}</p>`
  })
  // The terms run up to the Second Schedule; the Schedules follow the attestation.
  const cut = html.findIndex((h) => h.includes('class="schedule"'))
  return { body: html.slice(0, cut).join('\n'), schedules: html.slice(cut).join('\n'), draft }
}

const t = terms(readFileSync(join(root, 'LICENSE'), 'utf8'))
const fonts = 'file://' + join(root, 'web/node_modules')
const css = readFileSync(join(dir, 'agreement.css'), 'utf8').replaceAll('{{FONTS}}', fonts)
const fill = (file: string) => readFileSync(join(dir, file), 'utf8')
  .replace('{{LOGO}}', 'file://' + join(root, 'docs/images/logo.jpg'))
  .replace('{{DRAFT}}', t.draft ? `<div class="draft">${esc(t.draft)}</div>` : '')

const inked = (s: string) => `<span style="color:#1a3c9e;font-style:italic;font-weight:600">&nbsp;${s}&nbsp;</span>`
const initialsText = (i: boolean | [string, string]) =>
  Array.isArray(i) ? `Initials: Karots ${inked(i[0])} &nbsp; Shop ${inked(i[1])}` : i ? 'Initials: Karots ________ &nbsp; Shop ________' : ''
const sample = (script: string) => `<script>${readFileSync(join(dir, 'sample-kit.js'), 'utf8')}</script><script>${readFileSync(join(dir, script), 'utf8')}</script>`

const tmp = mkdtempSync(join(tmpdir(), 'ks-agreement-'))
const browser = await chromium.launch()
// initials: blanks to fill in by hand, the sample's own (in ink), or none (letters, forms).
async function print(name: string, title: string, body: string, initials: boolean | [string, string] = true) {
  const html = join(tmp, name + '.html')
  writeFileSync(html, `<!doctype html><html><head><meta charset="utf-8"><title>${title}</title><style>${css}</style></head><body>${body}</body></html>`)
  const p = await browser.newPage()
  await p.goto('file://' + html)
  await p.evaluate(() => document.fonts.ready)
  const out = join(root, 'docs/legal', name + '.pdf')
  await p.pdf({
    path: out, format: 'A4', printBackground: true, preferCSSPageSize: true, displayHeaderFooter: true,
    headerTemplate: '<span></span>',
    footerTemplate: `<div style="width:100%;margin:0 16mm;font:7.5pt sans-serif;color:#555;display:flex;justify-content:space-between">
      <span>Karots Suite — ${title}</span>
      <span>${initialsText(initials)}</span>
      <span>Page <span class="pageNumber"></span> of <span class="totalPages"></span></span></div>`,
  })
  console.log(out)
}
// The agreement in the usual Sri Lankan form: opening (parties, recitals), terms, attestation, Schedules.
const agreement = () => `${fill('preamble.html')}<section class="terms">${t.body}</section>${fill('attestation.html')}` +
  `${fill('order-form.html')}<section class="terms schedules">${t.schedules}</section>`
await print('karots-suite-agreement', 'Licence and Service Agreement', agreement())
await print('karots-suite-shop-rules', 'Shop Rules & Service Guide', fill('shop-rules.html'))
await print('karots-suite-balance-notice', 'Notice to pay the balance', fill('balance-notice.html'), false)

// The lawyer review: one box per point in lawyer-items.json.
type Item = { title: string; where: string; now: string; why: string; ask: string }
const items = (JSON.parse(readFileSync(join(dir, 'lawyer-items.json'), 'utf8')) as Item[]).map((x, i) => `
  <div class="item">
    <h2>Point ${i + 1} · ${esc(x.title)} <span class="where">${esc(x.where)}</span></h2>
    <p><b>What it says now:</b> ${esc(x.now)}</p>
    <p><b>Why:</b> ${esc(x.why)}</p>
    <p class="ask"><b>Question:</b> ${esc(x.ask)}</p>
    <div class="grid2">
      <div><label>Lawyer's opinion</label><div class="box"></div></div>
      <div><label>Suggested change / new wording</label><div class="box"></div></div>
    </div>
    <div class="boxes"><label class="lead">Decision</label><span>☐ Keep as it is</span><span>☐ Change as suggested</span><span>☐ Remove</span><span>☐ Needs more advice</span><span>Initials <span class="line inline"></span></span></div>
  </div>`).join('')
await print('karots-suite-lawyer-review', 'Legal review', fill('lawyer-review.html').replace('{{ITEMS}}', `<section class="items">${items}</section>`), false)

// A filled-in example of the agreement, marked SAMPLE on every page.
await print('karots-suite-agreement-sample', 'Licence and Service Agreement (SAMPLE)',
  `<div class="sample-mark">SAMPLE</div>${agreement()}${sample('sample.js')}`,
  ['K.K.', 'S.S.'])
await print('karots-suite-balance-notice-sample', 'Notice to pay the balance (SAMPLE)',
  `<div class="sample-mark">SAMPLE</div>${fill('balance-notice.html')}${sample('sample-notice.js')}`, false)
await browser.close()
