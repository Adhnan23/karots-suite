import { writeFileSync } from 'node:fs'
import { test, type Page } from '../e2e/fixtures'
import { cashier, code, only, out, owner, phone, session, startShop } from './demo'

// The website's phone tour (karots-suite-site /tour/<kind>/): per Demo Room shop, a sale as frames
// (01–06) and the screens an owner asks about (07–12). `make tour-shots` against a running Demo Room.
// Output: docs/images/tour/<kind>/<NN>-<name>.jpg — stable names, the site's tour lists them — and receipt.png,
// the printed slip as the printer got it (the site's home page shows one).
test.skip(!code, 'set DOCS_CODE to the Demo Room hourly code')
test.describe.configure({ mode: 'serial' })

const till = { width: 1366, height: 768 }
const search = 'Scan a barcode or search products'

// Each kind's headline screen at 08: batches and expiry for shops that sell dated goods,
// the plugin that sells the shop otherwise.
const kinds: { kind: string; shop: string; headline: string }[] = [
  { kind: 'pharmacy', shop: 'pharmacy', headline: '/admin/stock/batches' },
  { kind: 'grocery', shop: 'grocery', headline: '/admin/stock/batches' },
  { kind: 'phone', shop: 'phone', headline: '/admin/x/repairs' },
  { kind: 'hardware', shop: 'hardware', headline: '/admin/x/clearance' },
]
const screens: [string, string][] = [
  ['/admin/products', '07-products'],
  ['', '08-headline'],
  ['/admin/suppliers', '09-suppliers'],
  ['/admin/reorder', '10-reorder'],
  ['/admin/stock', '11-low-stock'],
  ['/admin', '12-dashboard'],
]

async function settle(page: Page) {
  await page.waitForLoadState('networkidle')
  await page.waitForTimeout(700) // charts and Motion settle
}

async function frame(page: Page, kind: string, name: string) {
  await settle(page)
  await page.screenshot({ path: `${out}/tour/${kind}/${name}.jpg`, type: 'jpeg', quality: 82 })
}

type Pick = { barcode: string; isService: boolean; trackSerial: boolean; stockFrozen?: boolean; stockQty: string; packs?: unknown[] }

/** Three products the till adds with one scan: no packs, serials or questions. */
async function threeProducts(page: Page): Promise<string[]> {
  const picks: Pick[] = await (await page.request.get('/api/v1/till/quick-picks')).json()
  return picks
    .filter((p) => p.barcode && !p.isService && !p.trackSerial && !p.stockFrozen && Number(p.stockQty) > 5 && !p.packs?.length)
    .map((p) => p.barcode)
}

/** Scans codes until `want` lines are in the cart; a code that opens a question is skipped. */
async function scanInto(page: Page, codes: string[], want: number) {
  const lines = page.getByRole('button', { name: 'Clear' })
  let added = 0
  while (added < want && codes.length) {
    const c = codes.shift()!
    await page.getByLabel(search).fill(c)
    await page.getByLabel(search).press('Enter')
    await settle(page)
    if (await page.getByRole('dialog').isVisible()) {
      await page.keyboard.press('Escape')
      continue
    }
    added++
  }
  if (added < want) throw new Error(`only ${added} products went in with one scan`)
  await lines.waitFor()
}

for (const k of kinds) {
  test(`tour: ${k.kind}`, async ({ browser }) => {
    if (only && !k.kind.includes(only)) return
    const ctl = await session(browser, null)
    await startShop(ctl, k.shop)
    await ctl.request.post('/__demo/pause', { data: { on: true } }) // the staff stop selling: the receipt is ours
    await ctl.request.post('/__demo/cheat/open-till', { data: {} })

    const page = await session(browser, cashier, till)
    await page.goto('/till')
    await page.getByLabel(search).waitFor()
    const before = (await (await ctl.request.get('/__demo/prints')).json()).receipt?.at(-1)?.id ?? 0
    const codes = await threeProducts(page)
    await frame(page, k.kind, '01-sale-empty')
    await scanInto(page, codes, 1)
    await frame(page, k.kind, '02-sale-scan')
    await scanInto(page, codes, 2)
    await frame(page, k.kind, '03-sale-cart')
    await page.getByRole('button', { name: /^Exact/ }).locator('xpath=following-sibling::button[1]').click() // a round note
    await frame(page, k.kind, '04-sale-pay')
    await page.getByRole('button', { name: /Complete sale/ }).click()
    await page.waitForTimeout(300)
    await frame(page, k.kind, '05-sale-done')

    // The slip the receipt printer got, on the counter-coloured page the till frames use.
    let id = 0
    await test.expect.poll(async () => {
      const r = (await (await ctl.request.get(`/__demo/prints?receipt=${before}`)).json()).receipt ?? []
      id = r.at(-1)?.id ?? 0
      return id
    }, { timeout: 20_000 }).toBeGreaterThan(0)
    const slip = await ctl.request.get(`/__demo/print/receipt/${id}`)
    const raw = await slip.body()
    writeFileSync(`${out}/tour/${k.kind}/receipt.png`, raw) // the slip itself, for the site's hero
    const png = raw.toString('base64')
    const paper = await browser.newPage({ viewport: till })
    await paper.setContent(`<body style="margin:0;height:100vh;display:grid;place-items:center;background:#eef0f2">
      <img src="data:image/png;base64,${png}" style="max-height:92vh;box-shadow:0 12px 32px -12px rgb(0 0 0/.3)"></body>`)
    await frame(paper, k.kind, '06-sale-receipt')
    await paper.close()
    await page.context().close()

    const admin = await session(browser, owner, phone)
    for (const [path, name] of screens) {
      await admin.goto(path || k.headline)
      await frame(admin, k.kind, name)
    }
    await admin.context().close()
    await ctl.request.post('/__demo/pause', { data: { on: false } })
    await ctl.context().close()
  })
}
