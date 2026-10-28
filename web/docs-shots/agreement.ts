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
// mark at the margin (2.1, A1, (a), a "defined term"); indented and other lines
// continue it.
const starts = /^(\d+\.\d*\s|[A-Z]\d\s|\([a-z]\)\s|"|Breaking |Copyright )/
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
    if (i === 0) return `<h2>${esc(b)}</h2>`
    if (b.startsWith('Copyright')) return `<p class="copy">${esc(b)}</p>`
    if (b.startsWith('DRAFT')) { draft = b; return '' }
    if (/^SCHEDULE /.test(b)) return `<h2 class="schedule">${esc(b)}</h2>`
    if (/^\d+\. [A-Z ,]+$/.test(b)) return `<h2>${esc(b)}</h2>`
    if (/^(\d+\.\d+|[A-Z]\d) /.test(b)) return `<p class="clause">${esc(b)}</p>`
    if (/^\([a-z]\) /.test(b)) return `<p class="item">${esc(b)}</p>`
    return `<p>${esc(b)}</p>`
  }).join('\n')
  return { html, draft }
}

const t = terms(readFileSync(join(root, 'LICENSE'), 'utf8'))
const fonts = 'file://' + join(root, 'web/node_modules')
const css = readFileSync(join(dir, 'agreement.css'), 'utf8').replaceAll('{{FONTS}}', fonts)
const fill = (file: string) => readFileSync(join(dir, file), 'utf8')
  .replace('{{LOGO}}', 'file://' + join(root, 'docs/images/logo.jpg'))
  .replace('{{DRAFT}}', t.draft ? `<div class="draft">${esc(t.draft)}</div>` : '')

const tmp = mkdtempSync(join(tmpdir(), 'ks-agreement-'))
const browser = await chromium.launch()
async function print(name: string, title: string, body: string) {
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
      <span>Initials: Karots ________ &nbsp; Shop ________</span>
      <span>Page <span class="pageNumber"></span> of <span class="totalPages"></span></span></div>`,
  })
  console.log(out)
}
await print('karots-suite-agreement', 'Order Form, Licence and Service Terms', `${fill('order-form.html')}<section class="terms">${t.html}</section>`)
await print('karots-suite-shop-rules', 'Shop Rules & Service Guide', fill('shop-rules.html'))
await browser.close()
