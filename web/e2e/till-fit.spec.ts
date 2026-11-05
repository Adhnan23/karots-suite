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
