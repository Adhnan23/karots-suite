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
  test.setTimeout(90_000) // two backups, a restore and two sign-ins: 7 s on a quiet PC, past 30 s in a full run
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
  await expect(page.getByRole('complementary', { name: 'Receipt preview' }).getByRole('img', { name: 'Receipt preview' })).toBeVisible()
  const sample = await (await page.request.post('/api/v1/admin/settings/sample-slip?preview=1')).text()
  expect(sample).toContain(`Thank you ${tag}`)

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

test('cashier menu sheet: preview, cancel, replace', async ({ page }) => {
  await signIn(page)
  // Every later spec on this worker reads the seeded menu: keep it and put it back.
  const saved = Buffer.from(await (await page.request.get('/api/v1/admin/data/menu/export?format=csv')).body())
  const putBack = () => page.request.post('/api/v1/admin/data/menu/import', { multipart: { file: { name: 'menu.csv', mimeType: 'text/csv', buffer: saved } } })
  try {
    for (const name of [`Menu A ${tag}`, `Menu B ${tag}`]) {
      expect((await page.request.post('/api/v1/admin/groups', { data: { name, emoji: '', parentId: 0 } })).ok()).toBe(true)
    }
    const count = async () => ((await (await page.request.get('/api/v1/admin/groups')).json()) as unknown[]).length
    const before = await count()
    // every group there is now, hidden ones too: the export's group rows (no product) + the two just made
    const all = saved.toString('utf8').replace(/^\uFEFF/, '').trim().split('\n').slice(1).filter((l) => l.split(',')[3] === '').length + 2
    const sheet = { name: 'menu.csv', mimeType: 'text/csv', buffer: Buffer.from(`Group,Product\nMenu A ${tag},\nMenu A ${tag},No such product ${tag}\n`) }

    await open(page, '/admin/data')
    const row = page.getByRole('region', { name: 'Spreadsheets' }).getByText('Cashier menu', { exact: true })
    await expect(row).toBeVisible()
    await page.locator('input[type=file][data-sheet="menu"]').setInputFiles(sheet)
    const dialog = page.getByRole('dialog')
    await expect(dialog).toContainText('1 group')
    await expect(dialog).toContainText(`${all - 1} groups removed`)
    await expect(dialog).toContainText('1 line skipped')
    await expect(dialog).toContainText(`line 3: no product with this barcode or name`)
    await dialog.getByRole('button', { name: 'Cancel' }).click()
    await expect(dialog).toBeHidden()
    expect(await count()).toBe(before)

    await page.locator('input[type=file][data-sheet="menu"]').setInputFiles(sheet)
    await page.getByRole('dialog').getByRole('button', { name: 'Replace the menu' }).click()
    await toast(page, /Menu replaced: 1 group/)
    expect(await count()).toBe(1)
    await page.setViewportSize({ width: 360, height: 740 })
    await noSideScroll(page)
  } finally {
    expect((await putBack()).ok()).toBe(true)
  }
})

test('cashier menu page: Sheets ▾ exports the menu and imports through the same preview', async ({ page }) => {
  await signIn(page)
  const saved = Buffer.from(await (await page.request.get('/api/v1/admin/data/menu/export?format=csv')).body())
  try {
    await open(page, '/admin/groups')
    await page.getByRole('button', { name: 'Sheets' }).click()
    const [dl] = await Promise.all([page.waitForEvent('download'), page.getByRole('menuitem', { name: 'CSV' }).click()])
    expect(dl.suggestedFilename()).toBe('cashier-menu.csv')

    await page.getByRole('button', { name: 'Sheets' }).click()
    await page.getByRole('menuitem', { name: /Import from a sheet/ }).click()
    await page.locator('input[type=file][data-sheet="menu"]').setInputFiles({ name: 'menu.csv', mimeType: 'text/csv', buffer: Buffer.from(`Group\nPage menu ${tag}\n`) })
    const dialog = page.getByRole('dialog')
    await expect(dialog).toContainText('Replace the cashier menu?')
    await dialog.getByRole('button', { name: 'Replace the menu' }).click()
    await toast(page, /Menu replaced: 1 group/)
    await expect(page.getByRole('list', { name: 'Groups' })).toContainText(`Page menu ${tag}`)
  } finally {
    expect((await page.request.post('/api/v1/admin/data/menu/import', { multipart: { file: { name: 'menu.csv', mimeType: 'text/csv', buffer: saved } } })).ok()).toBe(true)
  }
})
