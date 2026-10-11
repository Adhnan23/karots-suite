import { readFile } from 'node:fs/promises'
import { expect, test, type Page } from './fixtures'

// M6 data and settings flows. Needs `make e2e-seed`. The restore here puts back
// a backup taken seconds earlier, so the shop is the same afterwards for every
// later spec.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString(36).slice(-5)

async function signIn(page: Page, phone = '0770000001') {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}
async function open(page: Page, path: string) {
  await page.goto(path)
  await page.waitForLoadState('networkidle')
}
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()
const noSideScroll = (page: Page) => expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth + 1)).toBe(true)

test('data: download a backup, restore it behind RESTORE, sign in again, the shop is unchanged', async ({ page }) => {
  await signIn(page)
  await open(page, '/admin/data')
  const before = (await (await page.request.get('/api/v1/admin/products?limit=1')).json()).total as number

  const [dl] = await Promise.all([page.waitForEvent('download'), page.getByRole('link', { name: 'Download a backup' }).click()])
  expect(dl.suggestedFilename()).toMatch(/\.json\.gz$/)
  const file = { name: dl.suggestedFilename(), mimeType: 'application/gzip', buffer: await readFile(await dl.path()) }

  // Catalogue & setup only is a partial backup.
  await page.getByRole('radio', { name: /Catalogue & setup only/ }).click()
  const [setup] = await Promise.all([page.waitForEvent('download'), page.getByRole('region', { name: 'Backup', exact: true }).getByRole('link', { name: 'Download' }).click()])
  expect(setup.suggestedFilename()).toMatch(/-partial\.json\.gz$/)

  await page.locator('input[type=file][accept^=".gz"]').setInputFiles(file)
  const dialog = page.getByRole('dialog')
  await expect(dialog.getByRole('button', { name: 'Restore' })).toBeDisabled()
  await dialog.getByLabel(/Type RESTORE/).fill('restore')
  await expect(dialog.getByRole('button', { name: 'Restore' })).toBeDisabled()
  await dialog.getByLabel(/Type RESTORE/).fill('RESTORE')
  await dialog.getByRole('button', { name: 'Restore' }).click()
  await expect(dialog.getByRole('heading', { name: 'Restored' })).toBeVisible({ timeout: 60_000 })
  await expect(dialog.getByText(/before-restore\.json\.gz/)).toBeVisible()
  await dialog.getByRole('button', { name: 'Sign in again' }).click()
  await page.waitForURL(/\/login/)

  await signIn(page)
  expect((await (await page.request.get('/api/v1/admin/products?limit=1')).json()).total).toBe(before)
  await open(page, '/admin/data')
  await expect(page.getByLabel('Backup files').getByText('Before restore').first()).toBeVisible()
})

test('data: a manager can’t reach Data; a bad file is refused and nothing is restored', async ({ page }) => {
  await signIn(page, '0770000003')
  await page.goto('/admin/data')
  await page.waitForURL(/\/admin\/?$/)
  expect((await page.request.get('/api/v1/admin/data')).status()).toBe(403)

  await signIn(page)
  await open(page, '/admin/data')
  await page.locator('input[type=file][accept^=".gz"]').setInputFiles({ name: 'broken.json.gz', mimeType: 'application/gzip', buffer: Buffer.from('not a backup') })
  await page.getByRole('dialog').getByLabel(/Type RESTORE/).fill('RESTORE')
  await page.getByRole('dialog').getByRole('button', { name: 'Restore' }).click()
  await expect(page.getByRole('dialog').getByRole('alert')).toContainText(/can't be restored, nothing was changed/)
})

test('settings: save one card, the receipt preview follows; stock-take off hides its page; logo up and away', async ({ page }) => {
  await signIn(page)
  await open(page, '/admin/settings')
  const shop = page.getByRole('region', { name: 'Shop & receipt' })
  const footer = shop.getByLabel('Receipt footer')
  const old = await footer.inputValue()
  await footer.fill(`Thank you ${tag}`)
  await shop.getByRole('button', { name: 'Save' }).click()
  await toast(page, /Shop & receipt saved/)
  await expect(page.getByLabel('Receipt preview').getByText(`Thank you ${tag}`)).toBeVisible()

  const stock = page.getByRole('region', { name: 'Stock' })
  await stock.getByText('Stock-take & intake').click()
  await stock.getByRole('button', { name: 'Save' }).click()
  await toast(page, /Stock saved/)
  await page.goto('/admin/stock/take')
  await expect(page.getByRole('link', { name: 'Stock-take' })).toHaveCount(0)

  await open(page, '/admin/settings')
  await page.getByRole('region', { name: 'Stock' }).getByText('Stock-take & intake').click()
  await page.getByRole('region', { name: 'Stock' }).getByRole('button', { name: 'Save' }).click()
  await toast(page, /Stock saved/)
  await page.getByRole('region', { name: 'Shop & receipt' }).getByLabel('Receipt footer').fill(old)
  await page.getByRole('region', { name: 'Shop & receipt' }).getByRole('button', { name: 'Save' }).click()

  // A 1×1 PNG.
  const png = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=', 'base64')
  await page.getByRole('region', { name: 'Logo' }).locator('input[type=file]').setInputFiles({ name: 'logo.png', mimeType: 'image/png', buffer: png })
  await toast(page, /Logo uploaded/)
  await page.getByRole('region', { name: 'Logo' }).getByRole('button', { name: 'Remove' }).click()
  await toast(page, /Logo removed/)
})

test('import: a customer sheet with one bad row; product export downloads', async ({ page }) => {
  await signIn(page)
  await open(page, '/admin/customers')
  await page.getByRole('button', { name: 'Sheets' }).click()
  await page.getByRole('menuitem', { name: /Import from a sheet/ }).click()
  const csv = `Customer Name,Mobile,Credit Limit,Balance\nImp ${tag},0719${String(Date.now()).slice(-6)},"5,000","1,250.00"\n\nImp bad ${tag},,abc,\n`
  await page.locator('input[type=file][accept=".csv,.xlsx,.ods"]').setInputFiles({ name: 'customers.csv', mimeType: 'text/csv', buffer: Buffer.from(csv) })
  await page.getByRole('button', { name: 'Import', exact: true }).click()
  const report = page.getByLabel('Import result')
  await expect(report.getByText('1 added')).toBeVisible()
  await expect(report.getByText(/Line 4/)).toBeVisible()
  await page.keyboard.press('Escape')
  await page.getByPlaceholder(/Name or phone/).fill(`Imp ${tag}`)
  await expect(page.getByText('Rs. 1,250.00').first()).toBeVisible()

  await open(page, '/admin/products')
  await page.getByRole('button', { name: 'Sheets' }).click()
  const [dl] = await Promise.all([page.waitForEvent('download'), page.getByRole('menuitem', { name: 'XLSX' }).click()])
  expect(dl.suggestedFilename()).toBe('products.xlsx')
})

test('products export: Skip disabled off adds a disabled one with active = no; a re-import keeps it disabled', async ({ page }) => {
  await signIn(page)
  const code = `SKIP${tag}`
  const cat = (await (await page.request.get('/api/v1/admin/categories')).json())[0]
  const id = (await (await page.request.post('/api/v1/admin/products', { data: {
    name: `Skip ${tag}`, nameLocal: '', barcode: code, categoryId: cat.id, unitId: 1, costPrice: '40', sellingPrice: '60', wholesalePrice: '', taxRate: '',
    reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })).json()).id
  await page.request.delete(`/api/v1/admin/products/${id}`)
  const csv = async (qs: string) => (await page.request.get(`/api/v1/admin/products/export?format=csv${qs}`)).text()
  expect(await csv('')).not.toContain(code) // Skip disabled on: today's sheet
  const lines = (await csv('&disabled=1')).split('\n')
  expect(lines[0].split(',')).toContain('active')
  const row = lines.find((l) => l.includes(code))!
  expect(row.split(',')).toContain('no')
  const res = await page.request.post('/api/v1/admin/products/import?disabled=1', { multipart: { file: { name: 'p.csv', mimeType: 'text/csv', buffer: Buffer.from(`${lines[0]}\n${row}\n`) } } })
  expect(res.ok()).toBe(true)
  const p = await (await page.request.get(`/api/v1/admin/products/${id}`)).json()
  expect(p.isActive).toBe(false)
  const again = await (await page.request.get(`/api/v1/admin/products?q=${code}&disabled=1&page=1`)).json()
  expect(again.total).toBe(1) // matched, not added again
})

test('setup checklist on the dashboard; Data and Settings fit a phone', async ({ page }) => {
  await signIn(page)
  await open(page, '/admin')
  await expect(page.getByRole('region', { name: 'Get the shop ready' })).toBeVisible()
  await page.setViewportSize({ width: 360, height: 780 })
  for (const path of ['/admin/data', '/admin/settings']) {
    await open(page, path)
    await noSideScroll(page)
  }
})
