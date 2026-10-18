import { expect, test, type Page } from './fixtures'

// Reload & Bills (plugin, built in for Playwright). Needs `make e2e-seed`. Serial: one
// drawer, and later flows use earlier data (the carrier, its float).
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const CARRIER = `Dialog ${tag}`
const BANK = `BOC ${tag}`
const ids = { carrier: 0, device: 0, sale: 0 }

const json = async <T,>(p: Promise<{ json: () => Promise<unknown> }>) => (await (await p).json()) as T

async function login(page: Page, phone: string, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}

const cart = (page: Page) => page.getByRole('region', { name: 'Cart' })
const menu = (page: Page) => page.getByRole('tabpanel')

async function till(page: Page) {
  await login(page, '0770000002', '/till')
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  if (await dlg.isVisible()) {
    await dlg.getByLabel(/Count of .*500\.00/).fill('40')
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
  await page.getByRole('button', { name: /Reload & Bills/ }).first().click()
}

async function skipPrint(page: Page) {
  const skip = page.getByRole('button', { name: 'Skip' })
  await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
}

const float = async (page: Page) => {
  const d = await json<{ floats: { id: number; balance: string }[] }>(page.request.get('/api/v1/x/recharge/till'))
  return Number(d.floats.find((f) => f.id === ids.device)?.balance)
}

test('admin sets up a carrier: commission rates, a device with its float, a package', async ({ page }) => {
  await login(page, '0770000001', '/admin/x/recharge/carriers')
  await page.request.post('/api/v1/admin/lockers', { data: { name: BANK, kind: 'bank', openingBalance: '0', overdraft: false, cashierAccess: true } })
  await page.getByRole('button', { name: 'Add carrier' }).click()
  await page.getByLabel('Carrier name').fill(CARRIER)
  await page.getByRole('button', { name: 'Add', exact: true }).click()
  const card = page.getByRole('region', { name: CARRIER })
  await expect(card).toBeVisible()
  await card.getByRole('button', { name: 'Edit rates' }).click()
  await page.getByLabel('Reload percent').fill('3')
  await page.getByLabel('Bill payment service charge').fill('30')
  await page.getByRole('button', { name: 'Save rates' }).click()
  await expect(card.getByText('charge 30')).toBeVisible()
  await card.getByRole('button', { name: 'Add device' }).click()
  await page.getByLabel('Device name').fill('Shop phone')
  await page.getByLabel('Float on it now').fill('1000')
  await page.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(card.getByText('Shop phone')).toBeVisible()
  await card.getByRole('button', { name: 'Add package' }).click()
  await page.getByLabel('Package name').fill('Data 100')
  await page.getByLabel('Package price').fill('100')
  await page.getByLabel('Package commission').fill('12')
  await page.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(card.getByText(/uses Rs\. 88\.00/)).toBeVisible()
  const setup = await json<{ carriers: { id: number; name: string }[]; devices: { id: number; carrierId: number }[] }>(page.request.get('/api/v1/x/recharge/setup'))
  ids.carrier = setup.carriers.find((c) => c.name === CARRIER)!.id
  ids.device = setup.devices.find((d) => d.carrierId === ids.carrier)!.id
})

test('a reload, a package, a quick-added package and a bank bill go on one bill paid on Pay', async ({ page }) => {
  await till(page)
  await page.getByLabel("Customer's number").fill('0771112223')
  await page.getByLabel('Reload amount').fill('100')
  await page.getByRole('button', { name: /Add reload/ }).click()
  await page.getByRole('button', { name: /Data 100/ }).click()
  await page.getByRole('button', { name: 'New package' }).click()
  await page.getByRole('group', { name: 'New package' }).getByLabel('Package name').fill(`Night ${tag}`)
  await page.getByLabel('Package price').fill('50')
  await page.getByLabel('Commission (app)').fill('5')
  await page.getByRole('button', { name: 'Save & sell' }).click()
  await page.getByRole('tab', { name: /Pay a bill/ }).click()
  await menu(page).getByRole('button', { name: new RegExp(BANK) }).click()
  await page.getByLabel('Bill', { exact: true }).fill('Electricity')
  await page.getByLabel('Account number').fill('0123456789')
  await page.getByLabel('Bill amount').fill('2500')
  await page.getByLabel('Service charge').fill('30')
  await page.getByRole('button', { name: /Add bill/ }).click()
  await expect(cart(page).getByText(`${CARRIER} reload · 0771112223`)).toBeVisible()
  await expect(cart(page).getByText('Electricity · 0123456789')).toBeVisible()
  await expect(cart(page).getByText('Service charge · Electricity')).toBeVisible()
  const res = page.waitForResponse((r) => r.url().endsWith('/api/v1/till/sales') && r.request().method() === 'POST')
  await page.getByRole('button', { name: /Complete sale/ }).click()
  ids.sale = (await (await res).json()).id
  await skipPrint(page)
  // 1000 − 100 + 3% (3) − 88 + 12 − 45 + 5
  expect(await float(page)).toBe(787)
  const d = await json<{ banks: { name: string; balance: string }[] }>(page.request.get('/api/v1/x/recharge/till'))
  expect(Number(d.banks.find((b) => b.name === BANK)?.balance)).toBe(-2500) // paid in the bank's app: never refused
})

test('a cash out nets against goods: Pay gives the customer the difference', async ({ page }) => {
  await till(page)
  await page.getByRole('tab', { name: /Cash out/ }).click()
  await menu(page).getByRole('button', { name: new RegExp(`${CARRIER} · Shop phone`) }).click()
  await page.getByLabel('Cash to give').fill('5000')
  await page.getByRole('button', { name: /Add cash out/ }).click()
  await page.getByRole('tab', { name: /Reload/ }).first().click()
  await page.getByLabel('Reload amount').fill('200')
  await page.getByRole('button', { name: /Add reload/ }).click()
  await expect(page.getByText(/Give the customer/).first()).toBeVisible()
  await page.getByRole('button', { name: /Complete sale/ }).click()
  await skipPrint(page)
  await expect(page.getByText(/Give the customer|Start next sale/).first()).toBeVisible()
  // 787 + 5000 (cash out arrives on the phone) − 200 + 6 (3%)
  expect(await float(page)).toBe(5593)
})

test('a failed reload comes back through Returns and puts its float back; admin counts and sees the new package', async ({ page }) => {
  await login(page, '0770000002', `/till/returns?sale=${ids.sale}`)
  // The shared drawer may be short after the cash out: put cash in so the refund isn't refused.
  await page.request.post('/api/v1/till/drawer/deposit', { data: { amount: '10000', reason: 'e2e float' } })
  await page.getByRole('button', { name: `More ${CARRIER} reload · 0771112223` }).click()
  await expect(page.getByText(/Back to the float/)).toBeVisible()
  await page.getByRole('button', { name: /Refund Rs/ }).click()
  await skipPrint(page)
  await expect.poll(() => float(page)).toBe(5690) // + 100 back − its 3 commission
  await login(page, '0770000001', '/admin/x/recharge')
  await page.getByRole('button', { name: 'Count now' }).click()
  await page.getByLabel(`Float on ${CARRIER} Shop phone`).fill('5700')
  await page.getByRole('button', { name: 'Save the count' }).click()
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: /\+Rs\. 10\.00 booked/ }).first()).toBeVisible()
  await page.goto('/admin/x/recharge/carriers')
  const card = page.getByRole('region', { name: CARRIER })
  await expect(card.getByText(/new — added at the till/)).toBeVisible()
  await card.getByRole('button', { name: 'Looks right' }).click()
  await expect(card.getByText(/new — added at the till/)).toBeHidden()
})
