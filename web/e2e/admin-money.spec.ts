import { expect, test, type Page } from './fixtures'

// M4 money admin flows: lockers, moves, expenses, cash flow, tills, receipts,
// finance. Needs `make e2e-seed`.
test.describe.configure({ mode: 'serial' })
const tag = Date.now().toString(36).slice(-5)
const safe = `Flow Safe ${tag}`
const bank = `Flow Bank ${tag}`
const card = `Flow Card ${tag}`

async function admin(page: Page, path: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(path)
  await page.waitForLoadState('networkidle')
}
async function pick(page: Page, label: string, text: string) {
  const sel = page.getByLabel(label, { exact: true })
  await sel.selectOption((await sel.locator('option', { hasText: text }).first().getAttribute('value'))!)
}
const toast = (page: Page, text: RegExp) => expect(page.locator('[data-sonner-toast]').filter({ hasText: text }).first()).toBeVisible()
const tile = (page: Page, name: string) => page.getByRole('listitem').filter({ has: page.getByRole('link', { name, exact: true }) })

async function newLocker(page: Page, name: string, kind: string, opening: string) {
  await page.getByRole('button', { name: 'New locker' }).click()
  await page.getByRole('textbox', { name: 'Name', exact: true }).fill(name)
  await page.getByRole('radio', { name: kind }).click()
  await page.getByLabel('Opening balance').fill(opening)
  await page.getByRole('button', { name: 'Add locker' }).click()
  await toast(page, new RegExp(`${name} added`))
}

test('lockers: create, move money, ledger, bank charge books an expense, settle the card machine', async ({ page }) => {
  await admin(page, '/admin/lockers')
  await newLocker(page, safe, 'Safe', '5000')
  await newLocker(page, bank, 'Bank account', '0')
  await newLocker(page, card, 'Card machine', '2000')

  await page.getByRole('button', { name: `${safe} actions` }).click()
  await page.getByRole('menuitem', { name: 'Move money from here' }).click()
  await pick(page, 'To', bank)
  await page.getByLabel('Amount', { exact: true }).fill('1,000')
  await page.getByLabel('Note').fill('Banked the takings')
  await page.getByRole('button', { name: '⇄ Move it' }).click()
  await toast(page, new RegExp(`Moved Rs\\. 1,000\\.00 from ${safe} to ${bank}`))
  await expect(tile(page, safe)).toContainText('Rs. 4,000.00')

  await page.getByRole('button', { name: `${bank} actions` }).click()
  await page.getByRole('menuitem', { name: 'Adjust' }).click()
  await page.getByRole('radio', { name: /Bank charge/ }).click()
  await page.getByLabel('Amount', { exact: true }).fill('25')
  await page.getByLabel('Note').fill(`Flow fee ${tag}`)
  await page.getByRole('button', { name: 'Record it' }).click()
  await toast(page, /Bank charge of Rs\. 25\.00 recorded/)

  await page.getByRole('button', { name: 'Settle card' }).click()
  await pick(page, 'Card machine', card)
  await pick(page, 'Into bank', bank)
  await expect(page.getByLabel('Amount settled')).toHaveValue('2000.00')
  await page.getByLabel('Service fee').fill('40')
  await expect(page.getByText('Rs. 1,960.00')).toBeVisible()
  await page.getByRole('button', { name: 'Settle', exact: true }).click()
  await toast(page, new RegExp(`Settled Rs\\. 1,960\\.00 into ${bank}`))
  await expect(tile(page, bank)).toContainText('Rs. 2,935.00')
  await expect(tile(page, card)).toContainText('Rs. 0.00')

  await tile(page, bank).getByRole('link', { name: bank, exact: true }).click()
  await page.waitForURL(/\/admin\/lockers\/\d+/)
  const ledger = page.getByLabel('Ledger').locator('visible=true')
  await expect(ledger.getByText('Transfer').first()).toBeVisible()
  await expect(ledger.getByText('Bank charge').first()).toBeVisible()
  await expect(ledger.getByText(/CR-\d+/).first()).toBeVisible()

  await page.goto(`/admin/expenses?preset=today&q=${encodeURIComponent(`Flow fee ${tag}`)}`)
  await expect(page.getByLabel('Expenses').locator('visible=true').getByText('Bank charges')).toBeVisible()
})

test('expenses: pay one from a locker, void it back into the same locker', async ({ page }) => {
  await admin(page, '/admin/expenses')
  await page.getByRole('button', { name: '↑ Record expense' }).click()
  await page.getByRole('button', { name: 'Transport', exact: true }).click()
  await page.getByLabel('Amount', { exact: true }).fill('300')
  await pick(page, 'Paid from', safe)
  await page.getByLabel('Note').fill(`Flow taxi ${tag}`)
  await page.getByRole('button', { name: '↑ Record it' }).click()
  await toast(page, /Transport · Rs\. 300\.00 recorded/)

  await page.getByRole('button', { name: new RegExp(`Flow taxi ${tag}`) }).locator('visible=true').first().click()
  await expect(page.getByLabel('Amount', { exact: true })).toHaveJSProperty('readOnly', true)
  await page.getByRole('button', { name: 'Void…' }).click()
  await expect(page.getByLabel('Put it back into')).toHaveValue(/^locker:\d+$/)
  await page.getByRole('button', { name: 'Void expense' }).click()
  await toast(page, /Voided — Rs\. 300\.00 put back/)

  await page.goto('/admin/lockers')
  await expect(tile(page, safe)).toContainText('Rs. 4,000.00')
})

test('cash flow, till sessions, receipts and finance', async ({ page }) => {
  await admin(page, '/admin/cashflow?preset=today')
  await expect(page.getByText('⇄ Moved between your places')).toBeVisible()
  await expect(page.getByText(/opening balances of lockers added/)).toBeVisible()
  await pick(page, 'Place', bank)
  await expect(page.getByLabel('Movements').locator('visible=true').getByText('moved').first()).toBeVisible()

  await page.goto('/admin/tills')
  await page.getByLabel('Sessions').locator('visible=true').getByRole('button').first().click()
  await page.waitForURL(/\/admin\/tills\/\d+/)
  await expect(page.getByRole('link', { name: 'Z report' })).toHaveAttribute('href', /\/print\/z\/\d+/)
  await expect(page.getByText('Drawer movements')).toBeVisible()

  await page.goto('/admin/receipts')
  for (const tab of ['Sales', 'Cash', 'Credit', 'Warranty']) {
    await page.getByRole('group', { name: 'Receipt kind' }).getByRole('button', { name: tab, exact: true }).click()
    await expect(page.getByRole('button', { name: tab, exact: true })).toHaveAttribute('aria-pressed', 'true')
  }
  await page.getByRole('button', { name: 'Cash', exact: true }).click()
  await page.getByLabel('Find a receipt').fill(safe)
  await page.getByLabel('Receipts').locator('visible=true').getByRole('button').first().click()
  await expect(page.getByRole('button', { name: /Print again/ })).toBeVisible()
  await expect(page.getByText('80 mm')).toBeVisible()

  await page.goto('/admin/finance')
  await expect(page.getByText('Revenue and profit')).toBeVisible()
  await page.getByRole('button', { name: 'Profit', exact: true }).click()
  await expect(page.getByText('Profit and loss')).toBeVisible()
  await page.getByRole('button', { name: 'Money owed' }).click()
  await expect(page.getByText('Owed right now')).toBeVisible()
})

test.describe('at 360px', () => {
  test.use({ viewport: { width: 360, height: 780 } })
  test('lockers, expenses and finance read as cards and tables on a phone', async ({ page }) => {
    await admin(page, '/admin/lockers')
    await expect(tile(page, safe)).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360)
    await page.goto('/admin/finance')
    await expect(page.getByRole('region', { name: 'Revenue and profit' }).getByRole('table')).toBeVisible()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360)
    await page.goto('/admin/cashflow')
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360)
  })
})
