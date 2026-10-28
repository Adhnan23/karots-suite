// Prints the agreement a shop signs: the Order Form (docs/legal/agreement/) with
// blanks to fill in by hand, then the terms read from LICENSE, as one A4 PDF.
//   make agreement   →   docs/legal/karots-suite-agreement.pdf
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
const form = readFileSync(join(dir, 'order-form.html'), 'utf8')
  .replace('{{LOGO}}', 'file://' + join(root, 'docs/images/logo.jpg'))
  .replace('{{DRAFT}}', t.draft ? `<div class="draft">${esc(t.draft)}</div>` : '')
const page = `<!doctype html><html><head><meta charset="utf-8"><title>Karots Suite agreement</title>
<style>${readFileSync(join(dir, 'agreement.css'), 'utf8').replaceAll('{{FONTS}}', fonts)}</style></head>
<body>${form}<section class="terms">${t.html}</section></body></html>`

const tmp = join(mkdtempSync(join(tmpdir(), 'ks-agreement-')), 'agreement.html')
writeFileSync(tmp, page)
const browser = await chromium.launch()
const p = await browser.newPage()
await p.goto('file://' + tmp)
await p.evaluate(() => document.fonts.ready)
const out = join(root, 'docs/legal/karots-suite-agreement.pdf')
await p.pdf({
  path: out, format: 'A4', printBackground: true, preferCSSPageSize: true, displayHeaderFooter: true,
  headerTemplate: '<span></span>',
  footerTemplate: `<div style="width:100%;margin:0 16mm;font:7.5pt sans-serif;color:#555;display:flex;justify-content:space-between">
    <span>Karots Suite — Order Form, Licence and Service Terms</span>
    <span>Initials: Karots ________ &nbsp; Shop ________</span>
    <span>Page <span class="pageNumber"></span> of <span class="totalPages"></span></span></div>`,
})
await browser.close()
console.log(out)
