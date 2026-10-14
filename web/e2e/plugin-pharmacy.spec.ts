import { expect, test, type Page } from './fixtures'

// Pharmacy (plugin, built in for Playwright with Product Plus + Alternatives).
// Needs `make e2e-seed`. Serial: one drawer, and later flows use earlier data.
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const METFOR = `Metfor ${tag}`
const GROUP = `Metformin ${tag}`
const DIAZ = `Diazepam ${tag}`
const NAME = `Pharm Nimal ${tag}`
const PHONE = `07${tag}99`
const ids = { metfor: 0, diaz: 0, group: 0, patient: 0 }

const json = async <T,>(p: Promise<{ json: () => Promise<unknown> }>) => (await (await p).json()) as T
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()

async function login(page: Page, phone: string, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}

async function product(page: Page, name: string, stock: string, extra?: unknown) {
  const cats = await json<{ id: number; name: string }[]>(page.request.get('/api/v1/admin/categories'))
  const res = await page.request.post('/api/v1/admin/products', { data: {
    name, nameLocal: '', barcode: '', categoryId: cats.find((c) => c.name === 'E2E Goods')!.id, unitId: 1, costPrice: '4', sellingPrice: '6',
    wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
    ...(extra ? { extra: { pharmacy: extra } } : {}),
  } })
  expect(res.ok(), await res.text()).toBe(true)
  const id = (await res.json()).id as number
  expect((await page.request.post('/api/v1/admin/stock/adjust', { data: { productId: id, newQuantity: stock, note: 'pharmacy e2e', batchId: 0 } })).ok()).toBe(true)
  return id
}

const search = (page: Page) => page.getByLabel('Scan a barcode or search products')
const cart = (page: Page) => page.getByRole('region', { name: 'Cart' })
async function till(page: Page) {
  await login(page, '0770000002', '/till')
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  if (await dlg.isVisible()) {
    await dlg.getByLabel(/Count of .*500\.00/).fill('2')
    await dlg.getByRole('button', { name: 'Open the till' }).click()
    await expect(dlg).toBeHidden()
    const skip = page.getByRole('button', { name: 'Skip' })
    await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
  }
  if (await cart(page).getByText(/Cart is empty/).isHidden()) {
    await page.getByRole('button', { name: 'New sale' }).click()
    const yes = page.getByRole('dialog').getByRole('button', { name: /new sale|clear/i })
    if (await yes.isVisible().catch(() => false)) await yes.click()
  }
}
async function complete(page: Page) {
  // Paid in cash (with a patient on the sale, core would offer their account).
  await page.getByRole('button', { name: 'Cash', exact: true }).click()
  await page.getByRole('button', { name: 'Fill' }).click()
  await page.getByRole('button', { name: /Complete sale/ }).click()
  await expect(page.getByRole('dialog', { name: /complete/ })).toBeVisible()
  await page.keyboard.press('Enter')
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage()
  await login(page, '0770000001', '/admin')
  ids.metfor = await product(page, METFOR, '100')
  ids.diaz = await product(page, DIAZ, '50', { controlled: true, schedule: 'Schedule II', rxOnly: true })
  const g = await json<{ id: number; tiers: { id: number }[] }>(page.request.post('/api/v1/x/alternatives', { data: { name: GROUP } }))
  ids.group = g.id
  expect((await page.request.post(`/api/v1/x/alternatives/tiers/${g.tiers[0].id}/members`, { data: { productId: ids.metfor } })).ok()).toBe(true)
  await page.close()
})

test('the till registers a patient, writes a script, adds from it; the sale records the dispense with days', async ({ page }) => {
  await till(page)
  await page.getByRole('button', { name: '💊 Patient' }).click()
  await page.getByRole('button', { name: 'Register a new patient' }).click()
  await page.getByLabel('Patient name').fill(NAME)
  await page.getByLabel('Patient phone').fill(PHONE)
  await page.getByLabel('Allergies').fill('Penicillin')
  await page.getByRole('button', { name: 'Register', exact: true }).click()
  await expect(page.getByText('⚠ Allergic to: Penicillin')).toBeVisible()

  await page.getByRole('button', { name: 'Prescription' }).click()
  await page.getByLabel('Doctor', { exact: true }).fill('Dr Silva')
  await page.getByLabel('Medicine 1').fill(GROUP)
  await page.getByRole('button', { name: new RegExp(`${GROUP}.*group`) }).click()
  await page.getByLabel('Dosage 1').fill('1-0-1')
  await expect(page.getByLabel('A day 1')).toHaveValue('2')
  await page.getByLabel('Tablets 1').fill('8') // 4 days: inside the refill window straight away
  await page.getByRole('button', { name: 'Save prescription' }).click()
  await toast(page, /Prescription added/)

  await page.getByRole('button', { name: new RegExp(`Add 8 · ${METFOR}`) }).click()
  await toast(page, new RegExp(`Added 8 × ${METFOR}`))
  await page.getByRole('button', { name: 'Done' }).click()
  await expect(cart(page).getByText(`For ${NAME}’s ${GROUP}`)).toBeVisible()
  await complete(page)

  ids.patient = (await json<{ patientId: number }[]>(page.request.get(`/api/v1/x/pharmacy/patients?q=${PHONE}`)))[0].patientId
  await expect.poll(async () => {
    const s = await json<{ items: { dispensed: string; nextDue: string | null }[] }[]>(page.request.get(`/api/v1/x/pharmacy/patients/${ids.patient}/scripts`))
    return [s[0].items[0].dispensed, !!s[0].items[0].nextDue]
  }).toEqual(['8', true])
})

test('a controlled drug without a patient is only a hint; the sale goes through and waits for a record', async ({ page }) => {
  await till(page)
  await search(page).fill(DIAZ)
  await page.getByRole('button', { name: new RegExp(DIAZ) }).first().click()
  await expect(cart(page).getByText('⚠ Controlled — add the patient (💊)')).toBeVisible()
  await complete(page)

  await login(page, '0770000001', '/admin/x/pharmacy/controlled')
  const row = page.getByRole('table', { name: 'Controlled sales without a record' }).getByRole('row').filter({ hasText: DIAZ })
  await row.getByRole('button', { name: 'Add record' }).click()
  await page.getByLabel('Find patient').fill(PHONE)
  await page.getByRole('button', { name: new RegExp(NAME) }).click()
  await page.getByLabel('Doctor').fill('Dr Perera')
  await page.getByRole('button', { name: 'Save record' }).click()
  await toast(page, new RegExp(`recorded for ${NAME}`))
  await expect(row).toHaveCount(0)

  await page.goto(`/admin/x/pharmacy/controlled/${ids.diaz}`)
  const reg = page.getByRole('table', { name: 'Register' })
  await expect(reg.getByRole('row').filter({ hasText: 'Sold' })).toContainText(`${NAME} · Dr Perera`)
  await expect(reg.getByRole('row').last()).toContainText('49') // 50 counted in, 1 sold
})

test('refills due is a call list: WhatsApp the patient, Called takes them off', async ({ page }) => {
  await login(page, '0770000001', '/admin/x/pharmacy/refills')
  const row = page.getByRole('table', { name: 'Refills due' }).getByRole('row').filter({ hasText: NAME })
  await expect(row).toContainText(GROUP)
  await expect(row.getByRole('link', { name: 'WhatsApp' })).toHaveAttribute('href', new RegExp(`wa\\.me/94${PHONE.slice(1)}\\?text=`))
  await row.getByRole('button', { name: 'Called' }).click()
  await toast(page, /called/)
  await expect(row).toHaveCount(0)
})

test('the pharmacy pages fit a phone', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 })
  await login(page, '0770000001', '/admin/x/pharmacy')
  for (const p of ['', `patients/${ids.patient}`, 'refills', 'controlled', `controlled/${ids.diaz}`, 'dispensed']) {
    await page.goto(`/admin/x/pharmacy/${p}`)
    await page.waitForLoadState('networkidle')
    expect(await page.evaluate(() => document.documentElement.scrollWidth), p).toBeLessThanOrEqual(360)
  }
  await page.goto(`/admin/x/pharmacy/patients/${ids.patient}`)
  await expect(page.getByText(`${METFOR} × 8 (4 d)`)).toBeVisible()
})
