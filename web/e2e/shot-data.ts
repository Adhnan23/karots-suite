import type { Page } from './fixtures'

// Seed data for the UI-gate screenshots (admin-shots.spec.ts). Each helper is
// idempotent where it can be, so the 8 palette × mode × width runs share it.

/** A supplier with history for the M3 shots: old debt, two deliveries, a payment. */
export async function supplierWithHistory(page: Page): Promise<number> {
  const post = async (url: string, data: unknown) => (await page.request.post(url, { data })).json()
  const mk = (name: string, opening: string, phone = '', contact = '', days = 0) =>
    post('/api/v1/admin/suppliers', { name, contact, phone, address: 'Pettah, Colombo 11', creditDays: days, openingBalance: opening })
  const n = Date.now().toString(36).slice(-3)
  const s = await mk(`Ceylon Traders ${n}`, '2500', '0112345678', 'Nimal', 30)
  await mk(`Kandy Dairy ${n}`, '0', '0812233445', 'Sunil', 14)
  await mk(`Galle Spices ${n}`, '800', '0912222333')
  const milk = await (await page.request.get('/api/v1/till/products/barcode/4790000000011')).json()
  for (const [qty, inv] of [['24', 'CT-1041'], ['12', 'CT-1077']])
    await post(`/api/v1/till/suppliers/${s.id}/receive`, { invoiceNo: inv, discount: '', discountType: 'fixed', notes: '', keepRemainder: false, deferDrawerKick: true,
      pay: { amount: '', method: 'cash', lockerId: 0 }, lines: [{ productId: milk.id, name: '', ordered: '', quantity: qty, freeQty: '', costPrice: '85', sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed' }] })
  await post(`/api/v1/admin/suppliers/${s.id}/payment`, { amount: '1500', mode: 'old', allocations: [], advance: '', oldDebt: '', method: 'online', reference: 'BOC 88213', note: '', source: '' })
  return s.id
}

/** A credit customer with history: old debt, a credit sale's worth of activity, a payment. */
export async function customerWithHistory(page: Page): Promise<number> {
  const post = async (url: string, data: unknown) => (await page.request.post(url, { data })).json()
  const n = String(Date.now()).slice(-7)
  const mk = (name: string, phone: string, limit: string, opening: string) => post('/api/v1/admin/customers', { name, phone, address: 'Kandy Road, Peradeniya', creditLimit: limit, openingBalance: opening })
  const c = await mk('Kamala Perera', `071${n}`, '5000', '3200')
  await mk('Sunil Traders', `072${n}`, '2000', '2600')
  await mk('Ruwan Silva', `075${n}`, '0', '0')
  await post(`/api/v1/admin/customers/${c.id}/payment`, { amount: '1200', method: 'online', reference: 'TXN4471', note: '', source: '', oldDebtFirst: false })
  return c.id
}

/** Choose the option of a labelled <select> whose text starts with `text`. */
export async function pick(page: Page, label: string, text: string) {
  const sel = page.getByLabel(label, { exact: true })
  await sel.selectOption((await sel.locator('option', { hasText: text }).first().getAttribute('value'))!)
}

/** Four lockers (made once), the safe as the till default, and some money through
 * the safe: a customer paying in, a supplier paid out. Returns the safe's id. */
export async function lockersWithHistory(page: Page): Promise<number> {
  const post = async (url: string, data: unknown) => (await page.request.post(url, { data })).json()
  const have = (await (await page.request.get('/api/v1/admin/lockers')).json()).items as { id: number; name: string }[]
  const find = (n: string) => have.find((l) => l.name === n)?.id
  for (const [name, kind, openingBalance, overdraft, cashierAccess] of [['Shop safe', 'safe', '48250', false, true], ['BOC current account', 'bank', '312500', false, false],
    ["Owner's pocket", 'pocket', '0', true, false], ['Card machine', 'card', '18640', false, true]] as const)
    if (!find(name)) have.push(await post('/api/v1/admin/lockers', { name, kind, openingBalance, overdraft, cashierAccess }))
  const safe = find('Shop safe')!
  await page.request.put('/api/v1/admin/settings/till-cash', { data: { defaultLockerId: safe, allowUntracked: false } })
  if ((await (await page.request.get(`/api/v1/admin/lockers/${safe}/ledger?`)).json()).length < 3) {
    await post(`/api/v1/admin/customers/${await customerWithHistory(page)}/payment`, { amount: '1800', method: 'cash', reference: '', note: 'Paid at the shop', source: `locker:${safe}`, oldDebtFirst: false })
    await post(`/api/v1/admin/suppliers/${await supplierWithHistory(page)}/payment`, { amount: '1000', mode: 'old', allocations: [], advance: '', oldDebt: '', method: 'cash', reference: '', note: 'Old balance', source: `locker:${safe}` })
  }
  return safe
}

/** A month of expenses (made once): some paid from the safe, some from outside. */
export async function expensesWithHistory(page: Page) {
  const safe = await lockersWithHistory(page)
  if (((await (await page.request.get('/api/v1/admin/expenses?preset=this-month&q=CEB')).json()) as unknown[]).length) return
  const d = (n: number) => new Date(Date.now() - n * 864e5).toLocaleDateString('en-CA')
  for (const [category, amount, description, n, source] of [['Rent', '45000', 'October shop rent', 4, ''], ['Electricity', '8420', 'CEB bill', 3, `locker:${safe}`],
    ['Transport', '1200', 'Tuk to Pettah wholesale', 2, `locker:${safe}`], ['Salary', '15000', 'Nimali — half month', 1, ''], ['Water', '1350', 'NWSDB', 0, `locker:${safe}`]] as const)
    await page.request.post('/api/v1/admin/expenses', { data: { category, amount, description, date: d(n), serviceId: 0, source } })
}

/** A closed till session (deposit, withdraw, counted short) and the till open
 * again, made once. Returns the closed session's id. */
export async function tillWithHistory(page: Page): Promise<number> {
  const safe = await lockersWithHistory(page)
  const closed = async () => ((await (await page.request.get('/api/v1/admin/tills?preset=today')).json()) as { id: number; open: boolean }[]).find((s) => !s.open)?.id
  const have = await closed()
  if (have) return have
  const post = (url: string, data: unknown) => page.request.post(url, { data })
  if (!(await (await page.request.get('/api/v1/till/drawer')).json()).summary?.session)
    await post('/api/v1/till/drawer/open', { breakdown: { 1000: 4, 500: 2 }, sourceLockerId: safe })
  await post('/api/v1/till/drawer/deposit', { amount: '2000', reason: 'Change float top-up', lockerId: safe, deferDrawerKick: true })
  await post('/api/v1/till/drawer/withdraw', { amount: '3000', reason: 'Banked mid-day', lockerId: safe, deferDrawerKick: true })
  const s = (await (await page.request.get('/api/v1/till/drawer')).json()).summary
  const expected = Number(s.expected)
  await post('/api/v1/till/drawer/close', { breakdown: { 5000: Math.floor((expected - 50) / 5000), 100: Math.round(((expected - 50) % 5000) / 100) }, destLockerId: safe })
  await post('/api/v1/till/drawer/open', { breakdown: { 1000: 3, 500: 4 }, sourceLockerId: safe })
  return (await closed())!
}

/** A few sales today on the admin's open till (made once): cash, card, and one
 * on credit. */
export async function salesWithHistory(page: Page) {
  await tillWithHistory(page)
  if (((await (await page.request.get('/api/v1/admin/sales?preset=today&q=')).json()) as unknown[]).length >= 3) return
  const byCode = async (c: string) => (await (await page.request.get(`/api/v1/till/products/barcode/${c}`)).json()).id as number
  const milk = await byCode('4790000000011')
  const soap = await byCode('4790000000028')
  const cust = (await (await page.request.get('/api/v1/admin/customers?q=Kamala')).json()).items?.[0]?.id ?? await customerWithHistory(page)
  const sale = (items: [number, string][], payments: { method: string; amount: string }[], customerId?: number) =>
    page.request.post('/api/v1/till/sales', { data: { saleType: 'retail', discountType: 'fixed', customerId, payments,
      items: items.map(([productId, quantity]) => ({ productId, quantity, discountType: 'fixed' })) } })
  await sale([[milk, '4'], [soap, '2']], [{ method: 'cash', amount: '1000' }])
  await sale([[soap, '6']], [{ method: 'card', amount: '1000' }])
  await sale([[milk, '3']], [{ method: 'credit', amount: '0' }], cust)
}
