import { expect, test, type Page } from './fixtures'

// Cheques end to end: taken at the till and cleared into a bank; two taken at
// the counter's Credit tab, one bounced; an expense paid by cheque, cancelled,
// paid again. Needs `make e2e-seed`. Serial: one manager drawer.
test.describe.configure({ mode: 'serial' })

const MILK = '4790000000011' // Rs 100
const today = () => new Date().toISOString().slice(0, 10)
const uniq = () => String(Date.now()).slice(-6)

async function signIn(page: Page, phone = '0770000003') { // E2E Manager: the till and Money
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 4 }, sourceLockerId: 0 } })
}

async function skipSlip(page: Page) {
  const skip = page.getByRole('button', { name: 'Skip' })
  await skip.waitFor({ state: 'visible', timeout: 2500 }).then(() => skip.click()).catch(() => undefined)
}

const json = async (page: Page, url: string) => (await page.request.get(url)).json()
async function post(page: Page, url: string, data: unknown) {
  const r = await page.request.post(url, { data })
  if (!r.ok()) throw new Error(`${url}: ${r.status()} ${await r.text()}`)
  return r.status() === 204 ? null : r.json()
}
const balance = async (page: Page, id: number) =>
  Number(((await json(page, '/api/v1/admin/lockers')).items as { id: number; balance: string }[]).find((l) => l.id === id)!.balance)
const inHand = async (page: Page) =>
  Number(((await json(page, '/api/v1/admin/lockers')).items as { kind: string; balance: string }[]).find((l) => l.kind === 'cheque')!.balance)
const chequeNo = async (page: Page, number: string) =>
  ((await json(page, '/api/v1/admin/cheques?status=')) as { id: number; number: string; status: string }[]).find((c) => c.number === number)!

let bank = 0
async function setup(page: Page) {
  const admin = await page.context().browser()!.newPage()
  await signIn(admin, '0770000001')
  const lockers = (await json(admin, '/api/v1/admin/lockers')).items as { id: number; name: string }[]
  bank = lockers.find((l) => l.name === 'E2E Cheque Bank')?.id ??
    (await post(admin, '/api/v1/admin/lockers', { name: 'E2E Cheque Bank', kind: 'bank', openingBalance: '50000', overdraft: false, cashierAccess: true })).id
  await admin.close()
}

async function newCustomer(page: Page) {
  const name = `E2E Cheque ${uniq()}`
  const r = await post(page, '/api/v1/till/customers', { name, phone: `07${String(Date.now()).slice(-8)}`, creditLimit: '5000' })
  return { id: r.customer.id as number, name }
}

/** Open the Cheques page on one cheque and its sheet. */
async function openCheque(page: Page, number: string, dir: 'Received' | 'Given' = 'Received') {
  await page.goto('/admin/cheques')
  await page.getByRole('group', { name: 'Cheques' }).getByRole('button', { name: dir }).click()
  await page.getByRole('group', { name: 'Status' }).getByRole('button', { name: 'All' }).click()
  await page.getByLabel('Search cheques').fill(number)
  await page.getByRole('row').filter({ hasText: number }).getByRole('button').first().click()
}

test('till: cheque + cash for a named customer, then cleared into a bank', async ({ page }) => {
  await signIn(page)
  await setup(page)
  const cust = await newCustomer(page)
  const no = `5${uniq()}`
  await page.goto('/till')
  await page.waitForLoadState('networkidle')
  const search = page.getByLabel('Scan a barcode or search products')
  await search.fill(MILK)
  await search.press('Enter')
  await page.getByRole('button', { name: /Walk-in customer/ }).click()
  await page.getByLabel('Search customers').fill(cust.name)
  await page.getByLabel('Search customers').press('Enter')
  await page.getByRole('group', { name: 'Payment method' }).getByRole('button', { name: 'Cheque' }).click()
  await page.getByLabel('Bank', { exact: true }).fill('Bank of Ceylon')
  await page.getByLabel('Cheque number').fill(no)
  const amount = page.getByLabel('Amount tendered')
  await amount.fill('60')
  await amount.press('Enter')
  await page.getByRole('group', { name: 'Payment method' }).getByRole('button', { name: 'Cash' }).click()
  await amount.fill('40')
  await amount.press('Enter')
  await amount.press('Enter')
  await expect(page.getByRole('dialog', { name: /complete/ })).toContainText('Cheque + Cash')
  await skipSlip(page)

  // The receipt prints the cheque line.
  const sales = (await json(page, '/api/v1/admin/sales?q=' + encodeURIComponent(cust.name))) as { items?: { receiptNo: string }[] }
  const receiptNo = (sales.items ?? (sales as unknown as { receiptNo: string }[]))[0].receiptNo
  await page.goto('/till/receipts')
  await page.getByLabel('Find a sale').fill(receiptNo)
  await page.getByLabel('Find a sale').press('Enter')
  await expect(page.getByRole('article', { name: 'Receipt preview' })).toContainText(/CHEQUE/i)

  // Pending on the Cheques page; Mark cleared into the bank.
  expect((await chequeNo(page, no)).status).toBe('pending')
  const [bankBefore, handBefore] = [await balance(page, bank), await inHand(page)]
  await openCheque(page, no)
  const sheet = page.getByRole('dialog', { name: `Cheque ${no}` })
  await expect(sheet).toContainText(/Pending/i)
  await sheet.getByRole('button', { name: 'Mark cleared…' }).click()
  const clear = page.getByRole('dialog', { name: `Cleared — cheque ${no}` })
  await clear.getByLabel('Went into').selectOption({ label: 'E2E Cheque Bank' })
  await clear.getByRole('button', { name: /Mark cleared · / }).click()
  await expect(clear).toBeHidden()
  await expect.poll(async () => (await chequeNo(page, no)).status).toBe('cleared')
  expect(await balance(page, bank)).toBeCloseTo(bankBefore + 60, 2)
  expect(await inHand(page)).toBeCloseTo(handBefore - 60, 2)
})

test('counter credit: paid by two cheques, one bounces back onto the statement', async ({ page }) => {
  await signIn(page)
  const cust = await newCustomer(page)
  const milk = await json(page, `/api/v1/till/products/barcode/${MILK}`)
  await post(page, '/api/v1/till/sales', { saleType: 'retail', discountType: 'fixed', customerId: cust.id,
    items: [{ productId: milk.id, quantity: '3', discountType: 'fixed' }], payments: [{ method: 'credit', amount: '300' }] })
  const [a, b] = [`6${uniq()}1`, `6${uniq()}2`]
  await page.goto('/till/credit')
  await page.waitForLoadState('networkidle')
  await page.getByRole('button', { name: new RegExp(cust.name) }).click()
  await page.getByLabel('Amount', { exact: true }).fill('300')
  await page.getByRole('radiogroup', { name: 'Paid by' }).getByRole('radio', { name: 'Cheque' }).click()
  await page.getByLabel('Cheque 1 bank').fill('Sampath Bank')
  await page.getByLabel('Cheque 1 number').fill(a)
  await page.getByLabel('Cheque 1 amount').fill('100')
  await page.getByRole('button', { name: 'Add a cheque' }).click()
  await page.getByLabel('Cheque 2 bank').fill('Sampath Bank')
  await page.getByLabel('Cheque 2 number').fill(b)
  await page.getByLabel('Cheque 2 amount').fill('200')
  await page.getByRole('button', { name: /^Collect/ }).click()
  await skipSlip(page)
  await expect.poll(async () => (await chequeNo(page, b))?.status).toBe('pending')

  await openCheque(page, b)
  await page.getByRole('dialog', { name: `Cheque ${b}` }).getByRole('button', { name: 'Mark bounced' }).click()
  await page.getByRole('dialog', { name: `Cheque ${b} bounced?` }).getByRole('button', { name: 'Mark bounced' }).click()
  await expect.poll(async () => (await chequeNo(page, b)).status).toBe('bounced')
  const st = await json(page, `/api/v1/admin/customers/${cust.id}/statement`)
  expect(JSON.stringify(st)).toContain('Cheque bounced')
  const c = (await json(page, '/api/v1/till/customers')).find((x: { id: number }) => x.id === cust.id)
  expect(Number(c.outstanding)).toBeCloseTo(200, 2)
})

test('admin expense by cheque: cancelled, then paid again', async ({ page }) => {
  await signIn(page)
  const no = `7${uniq()}`
  const what = `e2e cheque rent ${uniq()}`
  await post(page, '/api/v1/admin/expenses', { category: 'Rent', amount: '1500', description: what, date: '', serviceId: 0, source: '',
    method: 'cheque', cheques: [{ bank: '', lockerId: bank, number: no, amount: '1500', date: today(), note: '' }] })
  await openCheque(page, no, 'Given')
  await page.getByRole('dialog', { name: `Cheque ${no}` }).getByRole('button', { name: 'Mark cancelled' }).click()
  await page.getByRole('dialog', { name: `Cancel cheque ${no}?` }).getByRole('button', { name: 'Mark cancelled' }).click()
  await expect.poll(async () => (await chequeNo(page, no)).status).toBe('cancelled')

  await page.goto('/admin/expenses')
  const row = page.getByRole('row', { name: new RegExp(what) }).or(page.getByRole('listitem').filter({ hasText: what })).first()
  await expect(row).toContainText('Not paid — cheque cancelled')
  await row.getByRole('button', { name: 'Pay' }).click()
  const pay = page.getByRole('dialog', { name: /Pay Rent/ })
  await pay.getByRole('button', { name: /↑ Pay/ }).click()
  await expect(pay).toBeHidden()
  await expect(page.getByRole('row', { name: new RegExp(what) }).or(page.getByRole('listitem').filter({ hasText: what })).first()).not.toContainText('Not paid')
})
