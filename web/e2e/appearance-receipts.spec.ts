import { expect, test, type Page } from './fixtures'

// Appearance → Receipt style (spec 2026-10-21): the system user picks one of ten
// Image-mode styles with a live preview; the saved one is what prints next.
test.describe.configure({ mode: 'serial' })

const MILK = '4790000000011'

async function asSystem(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0000000001')
  await page.getByLabel('PIN').fill(process.env.POS_SYSTEM_PIN ?? '2273')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/admin/)
}

async function asManager(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000003')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 4 }, sourceLockerId: 0 } })
}

async function setPrintAs(page: Page, printAs: string) {
  const s = (await (await page.request.get('/api/v1/admin/settings')).json()).settings
  expect((await page.request.put('/api/v1/admin/settings', { data: { ...s, printAs, askToPrint: false } })).ok()).toBe(true)
}

async function setStyle(page: Page, receiptStyle: string) {
  const a = await (await page.request.get('/api/v1/system/appearance')).json()
  expect((await page.request.put('/api/v1/system/appearance', { data: { ...a, receiptStyle } })).ok()).toBe(true)
}

/** One auto-printed cash sale; returns the picture the printer got. */
async function printedSale(page: Page): Promise<number[]> {
  let body: number[] | null = null
  await page.route(/\/api\/v1\/.*\/print(\?.*)?$/, async (route) => {
    const req = route.request()
    if (req.method() !== 'POST' || req.url().includes('preview=1') || req.url().includes('data=1')) return route.continue()
    body = Array.from(req.postDataBuffer() ?? [])
    await route.fulfill({ status: 204 })
  })
  await page.goto('/till')
  const search = page.getByLabel('Scan a barcode or search products')
  await search.fill(MILK)
  await search.press('Enter')
  await expect(page.getByRole('region', { name: 'Cart' }).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
  await page.keyboard.press('F10')
  await page.keyboard.type('500')
  await page.keyboard.press('Enter')
  await page.getByLabel('Amount tendered').press('Enter')
  await expect.poll(() => body).not.toBeNull()
  await page.unroute(/\/api\/v1\/.*\/print(\?.*)?$/)
  return body!
}

test.afterAll(async ({ browser }) => {
  const page = await browser.newPage()
  await asSystem(page)
  await setStyle(page, 'classic')
  await page.close()
  const m = await browser.newPage()
  await asManager(m)
  const s = (await (await m.request.get('/api/v1/admin/settings')).json()).settings
  await m.request.put('/api/v1/admin/settings', { data: { ...s, printAs: 'classic', askToPrint: true } })
  await m.close()
})

test('pick a style with a live preview; the next print uses it', async ({ page, browser }) => {
  const m = await browser.newPage()
  await asManager(m)
  await setPrintAs(m, 'image')
  const before = await printedSale(m)

  await asSystem(page)
  await setStyle(page, 'classic')
  await page.goto('/admin/system/appearance')
  const styles = page.getByRole('group', { name: 'Receipt style' })
  for (const name of ['Classic', 'Bold', 'Minimal', 'Ticket', 'Poster', 'Grid', 'Stamp', 'Elegant', 'Cards', 'Sections'])
    await expect(styles.getByRole('img', { name })).toBeVisible()

  const preview = page.getByRole('img', { name: 'Receipt preview' })
  await expect(preview).toBeVisible()
  const first = await preview.getAttribute('src')
  await styles.getByRole('button', { name: /Ticket/ }).click()
  await expect(preview).not.toHaveAttribute('src', first!)

  await page.getByRole('group', { name: 'Slip' }).getByRole('button', { name: 'Refund' }).click()
  await page.getByRole('group', { name: 'Roll width' }).getByRole('button', { name: '58 mm' }).click()
  await expect.poll(() => preview.evaluate((i: HTMLImageElement) => i.complete && i.naturalWidth)).toBe(384)

  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Saved')).toBeVisible()
  expect((await (await page.request.get('/api/v1/till/config')).json()).receiptStyle).toBe('ticket')

  const after = await printedSale(m)
  expect(after).not.toEqual(before)
  await m.close()
})

test('a Classic shop is told styles need Image printing', async ({ page, browser }) => {
  const m = await browser.newPage()
  await asManager(m)
  await setPrintAs(m, 'classic')
  await m.close()
  await asSystem(page)
  await page.goto('/admin/system/appearance')
  await expect(page.getByText(/prints Classic/)).toBeVisible()
})
