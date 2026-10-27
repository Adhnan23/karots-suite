import { createServer, type Server } from 'node:net'
import { expect, test, type Page } from './fixtures'

// Labels as pictures, end to end (spec 2026-10-26): the label printer is a TCP
// listener here, so the test reads the exact TSPL job — a BITMAP in Image mode,
// TEXT / BARCODE in Classic — for the price label, Pharmacy's dose label and
// Repairs' ticket label (Playwright builds every plugin).
test.describe.configure({ mode: 'serial' })

const jobs: Buffer[] = []
let server: Server
let port = 0
const tag = Date.now().toString(36).slice(-5)

test.beforeAll(async () => {
  server = createServer((s) => {
    const parts: Buffer[] = []
    s.on('data', (d) => parts.push(d))
    s.on('end', () => jobs.push(Buffer.concat(parts)))
  })
  await new Promise<void>((ok) => server.listen(0, '127.0.0.1', ok))
  port = (server.address() as { port: number }).port
})

async function signIn(page: Page, phone: string, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
}

async function labels(page: Page, labelPrintAs: string, labelPrinter = `tcp://127.0.0.1:${port}`) {
  const s = (await (await page.request.get('/api/v1/admin/settings')).json()).settings
  const res = await page.request.put('/api/v1/admin/settings', { data: { ...s, labelPrintAs, labelPrinter, labelWidthMm: 50, labelHeightMm: 25 } })
  expect(res.ok(), await res.text()).toBe(true)
}

async function openTill(page: Page) {
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 4 }, sourceLockerId: 0 } })
}

/** The next job the label printer gets, as text (the bitmap's bytes stay bytes). */
async function nextJob(): Promise<string> {
  const n = jobs.length
  await expect.poll(() => jobs.length, { timeout: 20_000 }).toBeGreaterThan(n)
  return jobs[n].toString('latin1')
}

test.afterAll(async ({ browser }) => {
  const page = await browser.newPage()
  await signIn(page, '0770000003', '/admin')
  await labels(page, 'classic', '')
  await page.close()
  server.close()
})

test('counter labels: Image sends a BITMAP, Classic sends TEXT and BARCODE', async ({ page }) => {
  await signIn(page, '0770000003', '/admin')
  await labels(page, 'image')
  await page.goto('/till/labels')
  await page.getByLabel('Barcode', { exact: true }).fill('4792024012348')
  await page.getByLabel('Barcode format').selectOption('EAN13')
  await page.getByLabel('First text', { exact: true }).fill('Milk')
  await page.getByLabel('Copies').fill('3')
  await expect(page.getByRole('img', { name: 'Label as it prints' })).toHaveAttribute('src', /^blob:/)
  await page.getByRole('button', { name: 'Print 3 labels' }).click()
  let job = await nextJob()
  expect(job.startsWith('SIZE 50 mm, 25 mm')).toBe(true)
  expect(job).toContain('BITMAP 0,0,50,200,0,')
  expect(job).toContain('PRINT 3,1')
  expect(job).not.toContain('BARCODE ')

  await labels(page, 'classic')
  await page.reload()
  await page.getByLabel('Barcode', { exact: true }).fill('4792024012348')
  await page.getByLabel('First text', { exact: true }).fill('Milk')
  await page.getByRole('button', { name: 'Print 1 label' }).click()
  job = await nextJob()
  expect(job).toContain('TEXT ')
  expect(job).toContain('BARCODE ')
  expect(job).not.toContain('BITMAP')
})

test('Pharmacy’s dose label prints as a picture', async ({ page }) => {
  await signIn(page, '0770000003', '/admin')
  await labels(page, 'image')
  const cats = await (await page.request.get('/api/v1/admin/categories')).json() as { id: number; name: string }[]
  const name = `Amoxil ${tag}`
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name, nameLocal: '', barcode: '', categoryId: cats.find((c) => c.name === 'E2E Goods')!.id, unitId: 1, costPrice: '4', sellingPrice: '6',
    wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
    extra: { pharmacy: { controlled: false, schedule: '', rxOnly: false, medicine: true, dose: '1-0-1', note: 'After meals' } },
  } })
  expect(res.ok(), await res.text()).toBe(true)
  expect((await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: (await res.json()).id, newQuantity: '20', note: 'label e2e', batchId: 0 } })).ok()).toBe(true)
  await openTill(page)
  await page.goto('/till')
  const search = page.getByLabel('Scan a barcode or search products')
  await search.fill(name)
  await page.getByRole('button', { name: new RegExp(name) }).first().click()
  const cart = page.getByRole('region', { name: 'Cart' })
  await cart.getByRole('button', { name: 'Label' }).click()
  await expect(cart.getByRole('img', { name: 'Label as it prints' })).toHaveAttribute('src', /^blob:/)
  await cart.getByRole('button', { name: 'Print label' }).click()
  const job = await nextJob()
  expect(job).toContain('BITMAP 0,0,50,200,0,')
  expect(job).toContain('PRINT 1,1')
})

test('Repairs’ ticket label prints as a picture', async ({ page }) => {
  await signIn(page, '0770000003', '/admin')
  await labels(page, 'image')
  const c = await (await page.request.post('/api/v1/till/customers', { data: { name: `Kamal ${tag}`, phone: `07799${tag.replace(/\D/g, '0').padEnd(5, '0').slice(0, 5)}`, creditLimit: '0' } })).json()
  const j = await (await page.request.post('/api/v1/x/repairs/jobs', { data: { customerId: c.customer.id, repairType: '', deviceModel: `Galaxy ${tag}`, repairedBy: '', fault: 'Cracked', notes: '', promisedDate: '', urgent: false } })).json()
  await openTill(page)
  await page.goto('/till')
  const search = page.getByLabel('Scan a barcode or search products')
  await search.fill(j.ticketCode)
  await search.press('Enter')
  await page.getByRole('dialog').getByRole('button', { name: 'Label' }).click()
  const ask = page.getByRole('dialog', { name: 'Print the ticket label?' })
  await expect(ask.getByRole('img', { name: 'Label as it prints' })).toHaveAttribute('src', /^blob:/)
  await ask.getByRole('button', { name: 'Print' }).click()
  const job = await nextJob()
  expect(job).toContain('BITMAP 0,0,50,200,0,')
})
