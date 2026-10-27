import { expect, test, type Page } from './fixtures'

// Appearance → Labels (spec 2026-10-26): the system user picks a style per label
// kind; the counter's Labels preview (Image mode) then draws that style.
test.describe.configure({ mode: 'serial' })

async function signIn(page: Page, phone: string, pin: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill(pin)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}
const asSystem = (p: Page) => signIn(p, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
const asManager = (p: Page) => signIn(p, '0770000003', '1234')

async function setLabelsAs(page: Page, labelPrintAs: string) {
  const s = (await (await page.request.get('/api/v1/admin/settings')).json()).settings
  expect((await page.request.put('/api/v1/admin/settings', { data: { ...s, labelPrintAs } })).ok()).toBe(true)
}

test.afterAll(async ({ browser }) => {
  const page = await browser.newPage()
  await asSystem(page)
  await page.request.put('/api/v1/system/appearance', { data: { ...(await (await page.request.get('/api/v1/system/appearance')).json()), labelStyles: { price: 'classic' } } })
  await page.close()
  const m = await browser.newPage()
  await asManager(m)
  await setLabelsAs(m, 'classic')
  await m.close()
})

test('pick a label style per kind; the counter preview draws it', async ({ page, browser }) => {
  const m = await browser.newPage()
  await asManager(m)
  await setLabelsAs(m, 'classic')

  await asSystem(page)
  await page.goto('/admin/system/appearance')
  const labels = page.getByRole('region', { name: 'Label styles' })
  const kinds = labels.getByRole('group', { name: 'Label kind' })
  await expect(kinds.getByRole('button', { name: 'Price label' })).toBeVisible()
  await expect(kinds.getByRole('button', { name: /Dose label/ })).toBeVisible()
  await expect(kinds.getByRole('button', { name: /Repair ticket/ })).toBeVisible()
  await expect(labels.getByText('This shop prints labels as Classic')).toBeVisible()

  const preview = labels.getByRole('img', { name: 'Label preview' })
  await expect(preview).toHaveAttribute('data-label-style', 'classic')
  await labels.getByRole('group', { name: 'Label style' }).getByRole('button', { name: 'Price Tag', exact: true }).click()
  await expect(preview).toHaveAttribute('data-label-style', 'pricetag')
  await page.getByRole('button', { name: 'Save' }).click()
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()

  await setLabelsAs(m, 'image')
  await m.goto('/till/labels')
  await m.getByLabel('Barcode', { exact: true }).fill('4792024012348')
  await m.getByLabel('First text', { exact: true }).fill('Milk')
  await expect(m.getByRole('img', { name: 'Label as it prints' })).toHaveAttribute('data-label-style', 'pricetag')
  await m.close()
})
