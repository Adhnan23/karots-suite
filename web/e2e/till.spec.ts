import { expect, receiveExpired, test, type Page } from './fixtures'

// Golden till flows (spec §9). Needs `make e2e-seed`. Serial: they share one drawer.
test.describe.configure({ mode: 'serial' })

const MILK = '4790000000011' // Rs 100, plenty
const SOAP = '4790000000028' // lots at 100 and 120
const YOGHURT = '4790000000035' // FEFO-first lot expired
const SCARCE = '4790000000042' // 1 on hand

async function login(page: Page, phone: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phone)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  if (!page.url().includes('/till')) await page.goto('/till')
  await page.waitForLoadState('networkidle')
}

async function skipSlip(page: Page) {
  const skip = page.getByRole('button', { name: 'Skip' })
  await skip.waitFor({ state: 'visible', timeout: 1500 }).then(() => skip.click()).catch(() => undefined)
}

async function ensureOpen(page: Page) {
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  if (await dlg.isVisible()) {
    await dlg.getByLabel(/Count of .*500\.00/).fill('2')
    await dlg.getByRole('button', { name: 'Open the till' }).click()
    await expect(dlg).toBeHidden()
    await skipSlip(page)
  }
}

const search = (page: Page) => page.getByLabel('Scan a barcode or search products')
async function scan(page: Page, code: string) {
  await search(page).fill(code)
  await search(page).press('Enter')
}
const cart = (page: Page) => page.getByRole('region', { name: 'Cart' })
const amount = (page: Page) => page.getByLabel('Amount tendered')
const done = (page: Page) => page.getByRole('dialog', { name: /complete/ })

test('open the till with a counted float', async ({ page }) => {
  await login(page, '0770000002')
  const dlg = page.getByRole('dialog', { name: 'Open the till' })
  await expect(dlg).toBeVisible()
  await dlg.getByLabel(/Count of .*500\.00/).fill('2')
  await expect(dlg.getByText(/1,000\.00/).first()).toBeVisible()
  await dlg.getByRole('button', { name: 'Open the till' }).click()
  await skipSlip(page)
  await expect(page.getByRole('button', { name: /Till open, .*1,000\.00 in the drawer/ })).toBeVisible()
})

test('three fast scans = qty 3; cash with change; Enter starts the next sale', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  for (let i = 0; i < 3; i++) await scan(page, MILK)
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('3')
  await page.keyboard.press('F10')
  await page.keyboard.type('500')
  await page.keyboard.press('Enter')
  await expect(page.getByText('Change to give')).toBeVisible()
  await amount(page).press('Enter')
  await expect(done(page)).toContainText(/200\.00/)
  await page.keyboard.press('Enter')
  await expect(done(page)).toBeHidden()
  await expect(cart(page).getByText(/Cart is empty/)).toBeVisible()
})

test('F1 clears the search, never the cart; a reload keeps the cart', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await search(page).fill('zzz')
  await page.keyboard.press('F1')
  await expect(search(page)).toHaveValue('')
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
  await page.reload()
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
})

test('Clear in the cart header asks, then empties the sale', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await cart(page).getByRole('button', { name: 'Clear' }).click()
  await page.getByRole('button', { name: 'Clear sale' }).click()
  await expect(cart(page).getByText(/Cart is empty/)).toBeVisible()
  await expect(cart(page).getByRole('button', { name: 'Clear' })).toHaveCount(0)
})

test('note tray: tap notes, Undo walks back one at a time, Clear empties', async ({ page }) => {
  await page.addInitScript(() => localStorage.setItem('ks-touch', 'on'))
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await page.getByRole('button', { name: 'Cash', exact: true }).click()
  for (const n of [500, 100, 100]) await page.getByRole('button', { name: `Add a ${n} note` }).click()
  await expect(amount(page)).toHaveValue('700')
  const undo = page.getByRole('button', { name: 'Undo the last note' })
  await undo.click()
  await expect(amount(page)).toHaveValue('600')
  await undo.click()
  await expect(amount(page)).toHaveValue('500')
  await page.getByRole('button', { name: 'Add a 100 note' }).click()
  await page.getByRole('button', { name: 'Clear the notes' }).click()
  await expect(amount(page)).toHaveValue('')
  await expect(undo).toBeDisabled()
})

test('split tender: card then cash', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await page.getByRole('button', { name: 'Card', exact: true }).click()
  await amount(page).fill('60')
  await amount(page).press('Enter')
  await page.getByRole('button', { name: 'Cash', exact: true }).click()
  await amount(page).fill('40')
  await amount(page).press('Enter')
  await amount(page).press('Enter')
  await expect(done(page)).toContainText('Card + Cash')
})

test('customer with a blank tender goes on account (never silent cash)', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await page.getByRole('button', { name: /Walk-in customer/ }).click()
  await page.getByLabel('Search customers').fill('E2E Credit')
  await page.getByLabel('Search customers').press('Enter')
  await expect(page.getByRole('button', { name: 'Cash', exact: true })).toHaveAttribute('aria-pressed', 'false')
  await amount(page).press('Enter')
  await page.getByRole('button', { name: 'Put on account' }).click()
  await expect(done(page)).toContainText('On account')
})

test('over the limit: cashier refused, admin approves', async ({ page }) => {
  // E2E Credit owes 100 (previous test), limit 1000: 10 × 100 more is over.
  for (const phone of ['0770000002', '0770000001']) {
    await login(page, phone)
    await ensureOpen(page)
    await scan(page, MILK)
    const qty = cart(page).getByLabel('Quantity of E2E Milk')
    await qty.fill('10')
    await qty.press('Enter')
    await page.getByRole('button', { name: /Walk-in customer/ }).click()
    await page.getByLabel('Search customers').fill('E2E Credit')
    await page.getByLabel('Search customers').press('Enter')
    await amount(page).press('Enter')
    await page.getByRole('button', { name: 'Put on account' }).click()
    if (phone === '0770000002') {
      await expect(page.locator('[data-sonner-toast][data-type="error"]')).toBeVisible()
      await expect(done(page)).toBeHidden()
      await page.getByRole('button', { name: 'New sale' }).click()
      await page.getByRole('button', { name: 'Clear sale' }).click()
      await page.context().clearCookies()
    } else {
      await page.getByRole('button', { name: 'Approve' }).click()
      await expect(done(page)).toBeVisible()
    }
  }
})

test('which price? pick the dearer lot', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, SOAP)
  const dlg = page.getByRole('dialog', { name: 'Which price?' })
  await dlg.getByRole('button', { name: /120\.00/ }).click()
  await expect(cart(page).getByText(/120\.00 each/)).toBeVisible()
})

test('recalled and stopped: the till warns, and sell anyway still works', async ({ page }) => {
  await login(page, '0770000003')
  const tag = Date.now().toString(36).slice(-6)
  const code = `RECALL${tag}`
  const cat = (await (await page.request.get('/api/v1/admin/categories')).json()).find((c: { name: string }) => c.name === 'E2E Goods')
  const product = await (await page.request.post('/api/v1/admin/products', { data: { name: `E2E Recalled ${tag}`, nameLocal: '', barcode: code, categoryId: cat.id, unitId: 1, costPrice: '40', sellingPrice: '60', wholesalePrice: '', taxRate: '', reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '' } })).json()
  const sup = await (await page.request.post('/api/v1/admin/suppliers', { data: { name: `E2E Recall Till ${tag}`, contact: '', phone: '', address: '', creditDays: 0, openingBalance: '' } })).json()
  const res = await page.request.post(`/api/v1/till/suppliers/${sup.id}/receive`, { data: { invoiceNo: '', discount: '', discountType: 'fixed', notes: '', keepRemainder: false, deferDrawerKick: true, pay: { amount: '', method: 'cash', lockerId: 0 },
    lines: [{ productId: product.id, name: '', ordered: '', quantity: '3', freeQty: '', costPrice: '40', sellingPrice: '', expiryDate: '', discount: '', discountType: 'fixed' }] } })
  expect(res.ok()).toBeTruthy()
  const lots = await (await page.request.get(`/api/v1/admin/stock/lots/${product.id}`)).json()
  const recall = await page.request.post(`/api/v1/admin/suppliers/${sup.id}/claims/lots/${lots.lots[0].id}/recall`, { data: { stopSelling: true } })
  expect(recall.ok()).toBeTruthy()
  await page.context().clearCookies()
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, code)
  await page.getByRole('dialog', { name: 'Which price?' }).getByRole('button', { name: /Recalled/ }).first().click()
  const warn = page.getByRole('dialog', { name: 'Recalled by the supplier' })
  await expect(warn).toBeVisible()
  await expect(warn.getByRole('button', { name: 'Write it off' })).toHaveCount(0)
  await warn.getByRole('button', { name: 'Sell anyway' }).click()
  await expect(cart(page).getByText(`E2E Recalled ${tag}`)).toBeVisible()
})

test('expired lot first: sell anyway', async ({ page }) => {
  // Its own expired lot (another flow may have written the seeded one off).
  await login(page, '0770000003')
  await receiveExpired(page, (await (await page.request.get(`/api/v1/till/products/barcode/${YOGHURT}`)).json()).id)
  await page.context().clearCookies()
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, YOGHURT)
  await page.getByRole('dialog', { name: 'Which price?' }).getByRole('button', { name: /Expired/ }).first().click()
  await expect(page.getByRole('dialog', { name: 'This lot is expired' })).toBeVisible()
  await page.getByRole('button', { name: 'Sell anyway' }).click()
  await expect(cart(page).getByText(/^exp /)).toBeVisible()
})

test('found at till: selling more than the count asks, then sells', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, SCARCE)
  await cart(page).getByLabel('More E2E Scarce').click()
  await amount(page).press('Enter')
  await expect(page.getByRole('dialog', { name: 'Sell more than the count shows?' })).toBeVisible()
  await page.getByRole('button', { name: 'Sell 2' }).click()
  await expect(done(page)).toBeVisible()
})

test('hold (F4) and resume', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, MILK)
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1') // the scan resolves async
  await page.keyboard.press('F4')
  await expect(cart(page).getByText(/Cart is empty/)).toBeVisible()
  await cart(page).getByRole('button', { name: /E2E Milk ·/ }).click()
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
})

test('quick item for something not in the catalogue', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await scan(page, 'Brand new thing')
  const dlg = page.getByRole('dialog', { name: 'Quick item' })
  await expect(dlg.getByLabel('Item name')).toHaveValue('Brand new thing')
  await dlg.getByLabel('Price').fill('50')
  await dlg.getByRole('button', { name: 'Add to cart' }).click()
  await expect(cart(page).getByLabel('Quantity of Brand new thing')).toHaveValue('1')
})

test('the till opens Quick item from its own button', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await page.getByRole('button', { name: '+ Quick item' }).click()
  const dlg = page.getByRole('dialog', { name: /Quick item/ })
  await expect(dlg).toBeVisible()
  await expect(dlg.getByLabel('Item name')).toHaveValue('')
  await expect(dlg.getByLabel('Barcode')).toHaveValue('')
})

test('a code no product has: give it to an existing item, then it scans as that item', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  const code = `77${Date.now() % 100_000_000_000}`
  await scan(page, code)
  const ask = page.getByRole('dialog', { name: 'No product has this barcode' })
  await ask.getByRole('button', { name: /Existing item/ }).click()
  await ask.getByLabel('Find the item').fill('E2E Milk')
  await ask.getByRole('button', { name: /^E2E Milk/ }).first().click()
  await expect(ask).toBeHidden()
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('1')
  await scan(page, code) // now one of E2E Milk's barcodes
  await expect(cart(page).getByLabel('Quantity of E2E Milk')).toHaveValue('2')
})

test('a code no product has: or make a new item with it', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  const code = `78${Date.now() % 100_000_000_000}`
  await scan(page, code)
  await page.getByRole('dialog', { name: 'No product has this barcode' }).getByRole('button', { name: /New item/ }).click()
  const quick = page.getByRole('dialog', { name: 'Quick item' })
  await expect(quick.getByLabel('Barcode')).toHaveValue(code)
})

test('deposit, withdraw, then close and reconcile',async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await page.getByRole('button', { name: /Deposit/ }).first().click()
  let dlg = page.getByRole('dialog', { name: /Deposit cash/ })
  await dlg.getByLabel('Amount').fill('200')
  await dlg.getByRole('button', { name: 'Deposit' }).click()
  await skipSlip(page)
  await page.getByRole('button', { name: /Withdraw/ }).first().click()
  dlg = page.getByRole('dialog', { name: /Withdraw cash/ })
  await dlg.getByLabel('Amount').fill('100')
  await dlg.getByRole('button', { name: 'Withdraw' }).click()
  await skipSlip(page)
  await page.getByRole('button', { name: 'Close till' }).click()
  dlg = page.getByRole('dialog', { name: 'Close the till' })
  await dlg.getByLabel(/Count of .*500\.00/).fill('2')
  await dlg.getByRole('button', { name: /Close & reconcile/ }).click()
  await expect(page.getByRole('dialog', { name: 'Till closed' })).toContainText('Over / short')
  await page.getByRole('button', { name: 'Done' }).click()
  await expect(page.getByRole('dialog', { name: 'Open the till' })).toBeVisible()
})

test('signing out with the till open goes to the count, then signs out', async ({ page }) => {
  await login(page, '0770000002')
  await ensureOpen(page)
  await page.getByRole('button', { name: 'Sign out' }).click()
  const dlg = page.getByRole('dialog', { name: 'Close the till' })
  await expect(dlg).toContainText('Close your till before signing out')
  await dlg.getByRole('button', { name: /Close & reconcile/ }).click()
  await page.getByRole('button', { name: 'Sign out' }).last().click()
  await expect(page).toHaveURL(/\/login/)
})
