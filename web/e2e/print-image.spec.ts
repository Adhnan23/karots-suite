import { expect, test, type Page } from './fixtures'

// Print as image (spec 2026-10-21): in Image mode every slip goes to the printer
// as the very picture the preview showed. Runs in Chromium and Firefox. The
// printer is never reached: print uploads are caught here and answered 204.
test.describe.configure({ mode: 'serial' })

const MILK = '4790000000011'

type Job = { url: string; type: string; body: Uint8Array | null }

async function signIn(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000003') // E2E Manager: till + settings
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 4 }, sourceLockerId: 0 } })
}

async function setPrinting(page: Page, printAs: string, askToPrint: boolean) {
  const s = (await (await page.request.get('/api/v1/admin/settings')).json()).settings
  const res = await page.request.put('/api/v1/admin/settings', { data: { ...s, printAs, askToPrint } })
  expect(res.ok(), await res.text()).toBe(true)
}

/** Every print POST (not ?preview=1) is caught and recorded. */
async function catchPrints(page: Page) {
  const jobs: Job[] = []
  await page.route(/\/api\/v1\/.*\/print(\?.*)?$/, async (route) => {
    const req = route.request()
    if (req.method() !== 'POST' || req.url().includes('preview=1')) return route.continue()
    jobs.push({ url: req.url(), type: req.headers()['content-type'] ?? '', body: (() => { const b = req.postDataBuffer(); return b ? new Uint8Array(b) : null })() })
    await route.fulfill({ status: 204 })
  })
  return jobs
}

const pngWidth = (b: Uint8Array) => new DataView(b.buffer, b.byteOffset).getUint32(16) // IHDR width
const isPNG = (j: Job) => j.type.startsWith('image/png') && !!j.body && String.fromCharCode(...j.body.subarray(1, 4)) === 'PNG'

async function cashSale(page: Page, ask = true) {
  await page.goto('/till')
  const search = page.getByLabel('Scan a barcode or search products')
  await search.fill(MILK)
  await search.press('Enter')
  await expect(page.getByRole('region', { name: 'Cart' }).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
  await page.keyboard.press('F10')
  await page.keyboard.type('500')
  await page.keyboard.press('Enter')
  await page.getByLabel('Amount tendered').press('Enter')
  // Ask mode stops at "Sale complete" (Print / New sale); auto-print prints and moves on.
  if (ask) await expect(page.getByRole('dialog', { name: /complete/ })).toBeVisible()
}

test.afterAll(async ({ browser }) => {
  const page = await browser.newPage()
  await signIn(page)
  await setPrinting(page, 'classic', true)
  await page.close()
})

test('ask-to-print: the preview is the picture that prints', async ({ page }) => {
  await signIn(page)
  await setPrinting(page, 'image', true)
  const jobs = await catchPrints(page)
  await page.goto('/till')
  await page.getByRole('button', { name: /Deposit/ }).first().click()
  const dep = page.getByRole('dialog', { name: /Deposit cash/ })
  await dep.getByLabel('Amount').fill('10')
  await dep.getByRole('button', { name: 'Deposit' }).click()

  const ask = page.getByRole('dialog', { name: /^Print/ })
  const img = ask.getByRole('img', { name: 'Receipt preview' })
  await expect(img).toBeVisible()
  expect(await img.evaluate((i: HTMLImageElement) => i.naturalWidth)).toBe(576)
  const shown = new Uint8Array(await img.evaluate(async (i: HTMLImageElement) => Array.from(new Uint8Array(await (await fetch(i.src)).arrayBuffer()))))
  await ask.getByRole('button', { name: 'Print' }).click()

  await expect.poll(() => jobs.length).toBe(1)
  expect(isPNG(jobs[0])).toBe(true)
  expect(Array.from(jobs[0].body!)).toEqual(Array.from(shown)) // the same pixels, not a re-render
  expect(jobs[0].url).not.toContain('kick=1') // ask mode: the server kicked at the move
})

test('a sale prints as a picture; auto-print carries the kick', async ({ page }) => {
  await signIn(page)
  await setPrinting(page, 'image', true)
  let jobs = await catchPrints(page)
  await cashSale(page)
  await page.getByRole('button', { name: 'Print receipt' }).click()
  await expect.poll(() => jobs.length).toBe(1)
  expect(isPNG(jobs[0]) && pngWidth(jobs[0].body!)).toBe(576)
  expect(jobs[0].url).not.toContain('kick=1')

  await setPrinting(page, 'image', false)
  await page.reload() // the till reads its config once
  jobs = await catchPrints(page)
  await cashSale(page, false)
  await expect.poll(() => jobs.length).toBe(1)
  expect(isPNG(jobs[0])).toBe(true)
  expect(jobs[0].url).toContain('kick=1')
})

test('a reprint at 58 mm is a 384-dot picture', async ({ page }) => {
  await signIn(page)
  await setPrinting(page, 'image', true)
  const jobs = await catchPrints(page)
  await page.goto('/till/receipts')
  await page.getByRole('group', { name: 'Receipt kind' }).getByRole('button', { name: 'Cash' }).click() // the deposit above
  await page.getByRole('list', { name: 'Receipts' }).getByRole('button').first().click()
  await page.getByRole('button', { name: '58 mm' }).click()
  await page.getByRole('button', { name: /Print again · 58 mm/ }).click()
  const ask = page.getByRole('dialog', { name: /^Reprint/ })
  if (await ask.isVisible().catch(() => false)) await ask.getByRole('button', { name: 'Print' }).click()
  await expect.poll(() => jobs.length).toBe(1)
  expect(isPNG(jobs[0]) && pngWidth(jobs[0].body!)).toBe(384)
  expect(jobs[0].url).toContain('size=58')
})

test('if the picture fails, that slip prints Classic once', async ({ page }) => {
  await page.addInitScript(() => {
    HTMLCanvasElement.prototype.toBlob = () => { throw new Error('no canvas here') }
  })
  await signIn(page)
  await setPrinting(page, 'image', true)
  const jobs = await catchPrints(page)
  await cashSale(page)
  await page.getByRole('button', { name: 'Print receipt' }).click()
  await expect.poll(() => jobs.length).toBe(1)
  await page.waitForTimeout(500)
  expect(jobs).toHaveLength(1)
  expect(isPNG(jobs[0])).toBe(false)
})

// Every receipt preview is the roll's width, in both modes — also in a box sized to its
// content (the sale-complete overlay), which once squeezed it to a sliver.
for (const mode of ['image', 'classic'] as const) {
  test(`every receipt preview shows at the roll's width (${mode})`, async ({ page }) => {
    await signIn(page)
    await setPrinting(page, mode, true)
    await catchPrints(page)
    const preview = (scope: ReturnType<Page['locator']>) => scope.locator('img[alt="Receipt preview"], article[aria-label="Receipt preview"]').first()
    const wide = async (where: string, scope: ReturnType<Page['locator']>) => {
      const el = preview(scope)
      await expect(el, where).toBeVisible()
      if (mode === 'image') await expect.poll(() => el.evaluate((i: HTMLImageElement) => i.complete && i.naturalWidth), { message: where }).toBeGreaterThan(0)
      const w = (await el.boundingBox())!.width
      expect(w, `${where}: ${w}px`).toBeGreaterThan(250) // the 80 mm roll (≈ 400 px) or the width its box allows (Settings' 340 px column) — never a sliver
      await page.screenshot({ path: `shots/receipt-previews/${mode}-${where}.png` })
    }

    await cashSale(page)
    await wide('sale-complete', page.getByRole('dialog', { name: /complete/ }))
    await page.getByRole('button', { name: /Start next sale/ }).click()

    await page.getByRole('button', { name: /Deposit/ }).first().click()
    const dep = page.getByRole('dialog', { name: /Deposit cash/ })
    await dep.getByLabel('Amount').fill('10')
    await dep.getByRole('button', { name: 'Deposit' }).click()
    const ask = page.getByRole('dialog', { name: /^Print/ })
    await wide('print-prompt', ask)
    await ask.getByRole('button', { name: 'Skip' }).click()

    await page.goto('/till/receipts')
    await page.getByRole('group', { name: 'Receipt kind' }).getByRole('button', { name: 'Cash' }).click() // the deposit above
    await page.getByRole('list', { name: 'Receipts' }).getByRole('button').first().click()
    await wide('counter-receipts', page.locator('body'))

    await page.goto('/admin/sales')
    await page.getByRole('button', { name: /^S-\d+/ }).first().click()
    await wide('sale-sheet', page.getByRole('dialog'))

    await page.goto('/admin/settings')
    await wide('settings', page.locator('body'))
  })
}
