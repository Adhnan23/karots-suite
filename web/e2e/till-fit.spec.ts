import { expect, test } from './fixtures'

// The till's columns fit the screen: nothing of the Pay panel runs off the right edge.
for (const [w, h] of [[1200, 700], [1280, 720], [1366, 768], [1600, 900]]) {
  test(`the Pay panel fits at ${w} wide`, async ({ page }) => {
    await page.setViewportSize({ width: w, height: h })
    await page.goto('/login')
    await page.getByLabel('Phone').fill('0770000001')
    await page.getByLabel('PIN').fill('1234')
    await page.getByRole('button', { name: /sign in/i }).click()
    await page.waitForURL(/\/(till|admin)/)
    const st = await (await page.request.get('/api/v1/till/drawer')).json()
    if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
    await page.goto('/till')
    const pay = page.getByRole('region', { name: 'Payment' })
    await expect(pay).toBeVisible()
    const box = (await pay.boundingBox())!
    expect(box.x + box.width).toBeLessThanOrEqual(w)
    const search = (await page.getByRole('region', { name: 'Catalogue' }).boundingBox())!
    expect(search.x + search.width).toBeLessThanOrEqual(box.x)
    if (process.env.SHOTS) await page.screenshot({ path: `shots/${process.env.SHOTS}/fit-${w}.png` })
  })
}

// Sell page 2.0: a layout saved for a screen size is what a till of that size shows.
test('a till follows the layout saved for its screen size', async ({ page, browser }) => {
  const sys = await browser.newPage()
  await sys.goto('/login')
  await sys.getByLabel('Phone').fill('0000000001')
  await sys.getByLabel('PIN').fill(process.env.POS_SYSTEM_PIN ?? '2273')
  await sys.getByRole('button', { name: /sign in/i }).click()
  await sys.waitForURL(/\/admin/)
  const save = (layouts: object) => sys.request.put('/api/v1/system/sell-layout', { data: { layouts } })
  expect((await save({ '1366x768': { arr: 'C_PY', fr: [0.4, 0.6], split: 0.6, card: 'L', text: 100 } })).ok()).toBeTruthy()
  try {
    await page.setViewportSize({ width: 1366, height: 768 })
    await page.goto('/login')
    await page.getByLabel('Phone').fill('0770000001')
    await page.getByLabel('PIN').fill('1234')
    await page.getByRole('button', { name: /sign in/i }).click()
    await page.waitForURL(/\/(till|admin)/)
    const st = await (await page.request.get('/api/v1/till/drawer')).json()
    if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
    await page.goto('/till')
    const box = async (name: string) => (await page.getByRole('region', { name }).boundingBox())!
    await expect(page.getByRole('region', { name: 'Payment' })).toBeVisible()
    const [cart, cat, pay] = [await box('Cart'), await box('Catalogue'), await box('Payment')]
    expect(cart.x).toBeLessThan(cat.x) // the cart is on the left
    expect(pay.y).toBeGreaterThanOrEqual(cat.y + cat.height) // pay under the products
    expect(pay.x).toBeCloseTo(cat.x, 0)

    await page.setViewportSize({ width: 1600, height: 900 }) // no layout for this size: today's
    await expect.poll(async () => (await box('Cart')).x > (await box('Catalogue')).x).toBe(true)
  } finally {
    await save({})
    await sys.close()
  }
})
