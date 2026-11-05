import type { BrowserContext, Page } from '@playwright/test'
import { expect, test } from './fixtures'

// The plugins' own till screens on the phone, with the PC down: everything is done
// offline, then the PC takes it all when it's back.
const APP = 'https://localhost:8443/x/pocketcashier/'
const API = '/api/v1/x/pocketcashier'
test.use({ launchOptions: { args: ['--ignore-certificate-errors'] } })

const tag = String(Date.now() % 1_000_000)
const CARRIER = `PPCarrier${tag}`
const COPY = `PPCopy${tag}`
const PAPER = `PPPaper${tag}`
const A = `PPTea${tag}`
const B = `PPSugar${tag}`
const DEAL = `PPDeal${tag}`
const MED = `PPMed${tag}`
const PATIENT = `PPPatient${tag}`
const PART = `PPPart${tag}`
const OWNER = `PPOwner${tag}`
const DEVICE = `PPGalaxy${tag}`

let ctx: BrowserContext
let phone: Page
let admin: Page
let device = 0

async function product(name: string, price: string, stock = '50') {
  const cats = await (await admin.request.get('/api/v1/admin/categories')).json() as { id: number; name: string }[]
  const res = await admin.request.post('/api/v1/admin/products', { data: {
    name, nameLocal: '', barcode: '', categoryId: cats.find((c) => c.name === 'E2E Goods')!.id, unitId: 1, costPrice: '10', sellingPrice: price,
    wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
  } })
  expect(res.ok(), await res.text()).toBe(true)
  const id = (await res.json()).id as number
  expect((await admin.request.post('/api/v1/admin/stock/adjust', { data: { productId: id, newQuantity: stock, note: 'pocket e2e', batchId: 0 } })).ok()).toBe(true)
  return id
}

async function setUpPlugins() {
  await admin.goto('/login')
  await admin.getByLabel('Phone').fill('0770000001')
  await admin.getByLabel('PIN').fill('1234')
  await admin.getByRole('button', { name: /sign in/i }).click()
  await admin.waitForURL(/\/admin/)

  // Reload & Bills: a carrier with a device holding float.
  await admin.goto('/admin/x/recharge/carriers')
  await admin.getByRole('button', { name: 'Add carrier' }).click()
  await admin.getByLabel('Carrier name').fill(CARRIER)
  await admin.getByRole('button', { name: 'Add', exact: true }).click()
  const card = admin.getByRole('region', { name: CARRIER })
  await card.getByRole('button', { name: 'Add device' }).click()
  await admin.getByLabel('Device name').fill('Shop phone')
  await admin.getByLabel('Float on it now').fill('1000')
  await admin.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(card.getByText('Shop phone')).toBeVisible()
  const setup = await (await admin.request.get('/api/v1/x/recharge/setup')).json() as { carriers: { id: number; name: string }[]; devices: { id: number; carrierId: number }[] }
  device = setup.devices.find((d) => d.carrierId === setup.carriers.find((c) => c.name === CARRIER)!.id)!.id

  // Print & Copy: a service priced from a grid, with paper.
  await product(PAPER, '5', '500')
  await admin.goto('/admin/x/documents')
  await admin.getByRole('button', { name: 'New print service' }).click()
  const dlg = admin.getByRole('dialog', { name: 'New print service' })
  if (await dlg.getByLabel('Start from').isVisible()) await dlg.getByLabel('Start from').selectOption('0')
  await dlg.getByLabel('Name').fill(COPY)
  await dlg.getByRole('button', { name: /Category|Pick a category/ }).first().click()
  await admin.getByRole('option', { name: /E2E Goods/ }).or(admin.getByRole('button', { name: /E2E Goods/ })).first().click()
  await dlg.getByRole('button', { name: 'Add' }).click()
  const grid = admin.getByRole('dialog', { name: `Prices & paper — ${COPY}` })
  const a4 = grid.getByRole('region', { name: 'A4' })
  await a4.getByLabel('A4 B&W 1-side from 1').fill('10')
  await a4.getByRole('button', { name: 'Pick the paper' }).click()
  await admin.getByLabel('Find a product').fill(PAPER)
  await admin.getByLabel('Find a product').press('Enter')
  await expect(a4.getByText(PAPER)).toBeVisible()
  await grid.getByRole('button', { name: 'Save' }).click()
  await expect(admin.getByRole('article').filter({ hasText: COPY })).toContainText('1 paper')

  // Combos: a two-item deal.
  const a = await product(A, '100')
  const b = await product(B, '200')
  const combo = await admin.request.post('/api/v1/x/combos', { data: { name: DEAL, discountType: 'fixed', discountValue: '50', startsOn: '', endsOn: '', items: [{ productId: a, qty: '1' }, { productId: b, qty: '1' }] } })
  expect(combo.ok(), await combo.text()).toBe(true)

  await product(MED, '20')
  await product(PART, '1000')
  const owner = await admin.request.post('/api/v1/till/customers', { data: { name: OWNER, phone: `07${tag}55`, creditLimit: '0' } })
  expect(owner.ok(), await owner.text()).toBe(true)
}

async function finish(payExact = false) {
  await phone.getByRole('button', { name: 'Pay ›' }).click()
  if (payExact) { // a named customer: the money has to be chosen; the biggest note covers it
    await phone.getByRole('group', { name: 'Payment method' }).getByRole('button', { name: 'Cash' }).click()
    await phone.getByRole('group', { name: 'Notes handed over' }).getByRole('button').last().click()
  }
  await phone.getByRole('button', { name: 'Finish sale' }).click()
  await expect(phone.getByText(/it will send when the PC is back/)).toBeVisible()
  await phone.getByRole('button', { name: 'New sale' }).click()
}

const closePanel = () => phone.keyboard.press('Escape')
const sell = () => phone.getByRole('button', { name: 'Sell', exact: true }).click()

test.describe.serial('Plugins on the phone with the PC down', () => {
  test.setTimeout(180_000)

  test.beforeAll(async ({ browser }) => {
    test.setTimeout(180_000)
    admin = await browser.newPage()
    await setUpPlugins()
    ctx = await browser.newContext({ viewport: { width: 360, height: 780 }, ignoreHTTPSErrors: true, hasTouch: true, isMobile: true })
    phone = await ctx.newPage()
    await phone.goto(APP)
    await phone.getByLabel('This phone’s name').fill(`Plugins ${tag}`)
    await phone.getByRole('button', { name: 'Pair this phone' }).click()
    const code = (await phone.getByLabel('Pairing code').textContent())!.trim()
    const parts = phone.waitForResponse((r) => r.url().includes(`${API}/parts`) && r.status() === 200, { timeout: 90_000 })
    expect((await admin.request.post(`${API}/phones/approve`, { data: { code } })).ok()).toBe(true)
    await phone.getByRole('radiogroup', { name: 'Who are you?' }).getByRole('radio', { name: 'E2E Cashier' }).click()
    for (const k of '1234') await phone.getByRole('button', { name: k, exact: true }).click()
    await phone.getByRole('button', { name: 'Sign in' }).click()
    await expect(phone.getByRole('navigation', { name: 'Phone tabs' })).toBeVisible()
    await phone.getByRole('button', { name: 'Menu', exact: true }).click()
    const close = phone.getByRole('button', { name: 'Close the drawer' })
    if (!(await close.isVisible())) {
      await phone.getByRole('button', { name: 'Open the drawer' }).click()
      await phone.getByLabel('Count of Rs. 1,000.00').fill('5')
      await phone.getByRole('button', { name: 'Open', exact: true }).click()
      await expect(close).toBeVisible()
    }
    // The plugins' data has come to the phone once its first sync is done.
    await parts
    await phone.waitForTimeout(1_000)
    await ctx.setOffline(true)
    await expect(phone.getByRole('note')).toBeVisible({ timeout: 30_000 })
  })
  test.afterAll(async () => { await ctx?.close(); await admin?.close() })

  test('a reload', async () => {
    await sell()
    await phone.getByRole('group', { name: 'Plugin menus' }).getByRole('button', { name: /Reload & Bills/ }).click()
    await phone.getByLabel("Customer's number").fill('0771112223')
    await phone.getByLabel('Reload amount').fill('100')
    await phone.getByRole('button', { name: /Add reload/ }).click()
    await closePanel()
    await finish()
  })

  test('a Print & Copy job, priced on the phone', async () => {
    await phone.getByRole('group', { name: 'Plugin menus' }).getByRole('button', { name: /Print & Copy/ }).click()
    await phone.getByRole('button', { name: new RegExp(COPY) }).first().click()
    await phone.getByLabel('Number of copies').fill('12')
    await expect(phone.getByText('Rs. 120.00').first()).toBeVisible()
    await phone.getByLabel('Number of copies').press('Enter')
    await expect(phone.getByText(/×12 added/)).toBeVisible()
    await closePanel()
    await finish()
  })

  test('a combo', async () => {
    await phone.getByRole('group', { name: 'Plugin menus' }).getByRole('button', { name: /Combos/ }).click()
    await phone.getByRole('button', { name: new RegExp(DEAL) }).click()
    await phone.getByRole('button', { name: /Add combo/ }).click()
    await expect(phone.getByText(new RegExp(`${DEAL} added`))).toBeVisible()
    await closePanel()
    await finish()
  })

  test('a new patient, a new prescription, and a sale from it', async () => {
    await phone.getByRole('button', { name: 'Menu', exact: true }).click()
    await phone.getByRole('button', { name: '💊 Patient' }).click()
    await phone.getByRole('button', { name: 'Register a new patient' }).click()
    await phone.getByLabel('Patient name').fill(PATIENT)
    await phone.getByLabel('Patient phone').fill(`07${tag}66`)
    await phone.getByLabel('Allergies').fill('Penicillin')
    await phone.getByRole('button', { name: 'Register', exact: true }).click()
    await expect(phone.getByText('⚠ Allergic to: Penicillin')).toBeVisible()
    await phone.getByRole('button', { name: 'Prescription' }).click()
    await phone.getByLabel('Doctor', { exact: true }).fill('Dr Silva')
    await phone.getByLabel('Medicine 1').fill(MED)
    await phone.getByRole('button', { name: new RegExp(MED) }).first().click()
    await phone.getByLabel('Dosage 1').fill('1-0-1')
    await phone.getByLabel('Days 1').fill('4')
    await phone.getByRole('button', { name: 'Save prescription' }).click()
    await phone.getByRole('button', { name: new RegExp(`Add · ${MED}`) }).click()
    await expect(phone.getByText(new RegExp(`Added 8 × ${MED}`))).toBeVisible()
    await phone.getByRole('button', { name: 'Done' }).click()
    await sell()
    await finish(true)
  })

  test('a repair taken in, a part and labour added, then billed', async () => {
    await phone.getByRole('button', { name: 'Menu', exact: true }).click()
    await phone.getByRole('button', { name: '🔧 Repairs' }).click()
    const d = phone.getByRole('dialog').last()
    await d.getByRole('button', { name: 'New job' }).click()
    await d.getByLabel('Find customer').fill(OWNER)
    await d.getByRole('list', { name: 'Customers' }).getByRole('button', { name: new RegExp(OWNER) }).click()
    await d.getByPlaceholder('e.g. Galaxy A12').fill(DEVICE)
    await d.getByPlaceholder(/What's wrong/).fill('Cracked screen')
    await d.getByRole('button', { name: 'Take it in' }).click()
    await expect(phone.getByText(/taken in/)).toBeVisible()
    await d.getByLabel('Add a part').fill(PART)
    await d.getByRole('button', { name: new RegExp(PART) }).click()
    await d.getByRole('button', { name: 'Add part' }).click()
    await d.getByLabel('Labour', { exact: true }).fill('Fitting')
    await d.getByLabel('Labour amount').fill('500')
    await d.getByRole('button', { name: 'Add labour' }).click()
    await expect(d.getByText(/Estimate\s*Rs\. 1,500\.00/)).toBeVisible()
    await d.getByRole('button', { name: 'Put on the bill' }).click()
    await expect(phone.getByText(/is on the bill/)).toBeVisible()
    await closePanel()
    await sell()
    await finish(true)
  })

  test('when the PC is back it takes everything, in order, through the real routes', async () => {
    await expect(phone.getByLabel(/waiting/)).toBeVisible()
    await ctx.setOffline(false)
    await expect(phone.getByRole('note')).toBeHidden({ timeout: 60_000 })
    await expect(phone.getByLabel(/waiting/)).toBeHidden({ timeout: 60_000 })

    const sends = await (await admin.request.get(`${API}/sends`)).json() as { id: number }[]
    const details = await Promise.all(sends.slice(0, 5).map(async (s) => (await admin.request.get(`${API}/sends/${s.id}`)).json()))
    const actions = details.flatMap((d: { actions: { kind: string; status: string; label: string; note: string }[] }) => d.actions)
    expect(actions.filter((a) => a.status !== 'applied').map((a) => `${a.kind} ${a.label}: ${a.note}`)).toEqual([])
    expect(actions.filter((a) => a.kind === 'sale').length).toBeGreaterThanOrEqual(5)

    // Reload: the float dropped.
    const till = await (await admin.request.get('/api/v1/x/recharge/till')).json() as { floats: { id: number; balance: string }[] }
    expect(Number(till.floats.find((f) => f.id === device)!.balance)).toBeLessThan(1000)
    // Print & Copy: the job is on the report.
    const doc = await (await admin.request.get('/api/v1/x/documents/report?preset=today')).json() as { rows: { name: string; copies: string }[] }
    expect(doc.rows.find((r) => r.name === COPY)?.copies).toBe('12')
    // Combos: the set is counted.
    const combos = await (await admin.request.get('/api/v1/x/combos/report?preset=today')).json() as { rows: { name: string; sets: string }[] }
    expect(Number(combos.rows.find((r) => r.name === DEAL)?.sets)).toBe(1)
    // Pharmacy: the patient is real, and the medicine is on their record.
    const found = await (await admin.request.get(`/api/v1/x/pharmacy/patients?q=${PATIENT}`)).json() as { patientId: number; allergies: string }[]
    expect(found[0]).toMatchObject({ allergies: 'Penicillin' })
    const scripts = await (await admin.request.get(`/api/v1/x/pharmacy/patients/${found[0].patientId}/scripts`)).json() as { items: { dispensed: string }[] }[]
    expect(Number(scripts[0].items[0].dispensed)).toBe(8)
    // Repairs: the job has a real number, and the sale collected it.
    const jobs = await (await admin.request.get(`/api/v1/x/repairs/jobs?show=all&q=${DEVICE}`)).json() as { ticketNo: string; status: string }[]
    expect(jobs[0].ticketNo).toMatch(/^R-\d+$/)
    expect(jobs[0].status).toBe('collected')
  })
})
