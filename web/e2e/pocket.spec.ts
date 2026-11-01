import type { Browser, BrowserContext, Page } from '@playwright/test'
import { expect, test } from './fixtures'

// The phone app on its own HTTPS port (the shop CA isn't trusted here). One
// phone context runs through the file: pair, takeover, sell live, sell offline.
const APP = 'https://localhost:8443/x/pocketcashier/'
const API = '/api/v1/x/pocketcashier'

// Chromium won't register a service worker under a certificate error even with
// ignoreHTTPSErrors; a real phone trusts the shop CA instead (the setup page).
test.use({ launchOptions: { args: ['--ignore-certificate-errors'] } })

let phoneCtx: BrowserContext
let phone: Page
let admin: Page

async function signInPC(page: Page, phoneNo: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phoneNo)
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(admin|till)/)
}

async function phoneSignIn(p: Page, name: string) {
  await p.getByRole('radiogroup', { name: 'Who are you?' }).getByRole('radio', { name }).click()
  for (const k of '1234') await p.getByRole('button', { name: k, exact: true }).click()
  await p.getByRole('button', { name: 'Sign in' }).click()
  await expect(p.getByRole('navigation', { name: 'Phone tabs' })).toBeVisible()
}

async function newPhone(browser: Browser) {
  const ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, ignoreHTTPSErrors: true, hasTouch: true, isMobile: true })
  return { ctx, page: await ctx.newPage() }
}

test.describe.serial('Pocket Cashier', () => {
  test.setTimeout(120_000)

  test.beforeAll(async ({ browser }) => {
    admin = await browser.newPage()
    await signInPC(admin, '0770000001')
    ;({ ctx: phoneCtx, page: phone } = await newPhone(browser))
  })
  test.afterAll(async () => { await phoneCtx?.close(); await admin?.close() })

  test('pair, approve, and the cashier moves from the PC to the phone with her cart', async ({ browser }) => {
    // The cashier is selling on the PC, with something in the cart.
    const pcCtx = await browser.newContext()
    const pc = await pcCtx.newPage()
    await signInPC(pc, '0770000002')
    await pc.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
    await pc.goto('/till')
    await pc.getByLabel('Scan a barcode or search products').fill('4790000000011')
    await pc.getByLabel('Scan a barcode or search products').press('Enter')
    await expect(pc.getByText('E2E Milk').first()).toBeVisible()

    // The phone pairs; the admin approves its code.
    await phone.goto(APP)
    await phone.getByLabel('This phone’s name').fill('E2E Phone')
    await phone.getByRole('button', { name: 'Pair this phone' }).click()
    const code = (await phone.getByLabel('Pairing code').textContent())!.trim()
    expect(code).toMatch(/^\d{6}$/)
    expect((await admin.request.post(`${API}/phones/approve`, { data: { code } })).status()).toBe(204)

    // She signs in on the phone: the PC locks, and her cart waits in Held.
    await phoneSignIn(phone, 'E2E Cashier')
    await expect(pc).toHaveURL(/\/lock$/, { timeout: 30_000 })
    await expect(pc.getByText(/now on/i)).toBeVisible()
    await phone.getByRole('button', { name: 'Held' }).click()
    await expect(phone.getByRole('list', { name: 'Held sales' })).toContainText('Moved to another device')
    await pcCtx.close()
  })

  test('sells live: two items, cash, the receipt prints at the counter', async () => {
    const prints: string[] = []
    await phone.route(/\/api\/v1\/till\/sales\/\d+\/print/, async (r) => { prints.push(r.request().url()); await r.fulfill({ status: 204 }) })
    await phone.getByRole('button', { name: 'Sell' }).click()
    await phone.getByLabel('Search or scan').fill('E2E Milk')
    const products = phone.getByRole('list', { name: 'Products' })
    await products.getByRole('button', { name: /E2E Milk/ }).first().click()
    await products.getByRole('button', { name: /E2E Milk/ }).first().click()
    await phone.getByRole('button', { name: 'Pay ›' }).click()
    await phone.getByRole('button', { name: 'Finish sale' }).click()
    // Ask before printing is on in the e2e shop: a dialog, never a toast.
    await phone.getByRole('dialog', { name: 'Print the receipt?' }).getByRole('button', { name: 'Print' }).click()
    await expect.poll(() => prints.length).toBe(1)
    const done = phone.getByRole('status').filter({ hasText: /Sale .* done/ })
    await expect(done).toBeVisible()
    const receipt = (await done.textContent())!.match(/Sale (\S+) done/)![1]

    await admin.goto(`/admin/sales?q=${receipt}`)
    await expect(admin.getByText(receipt).first()).toBeVisible()
    await phone.getByRole('button', { name: 'New sale' }).click()
  })

  test('sells offline from the copy, then the sale is sent and applied', async () => {
    await phoneCtx.setOffline(true)
    await phone.reload() // the app opens from the service worker's cache
    await expect(phone.getByRole('status').filter({ hasText: 'PC not reachable' }).first()).toBeVisible({ timeout: 30_000 })
    // Signing in again checks the PIN against the copy (no PC).
    await phone.getByRole('button', { name: 'E2E Cashier — menu' }).click()
    await phone.getByRole('menuitem', { name: 'Switch person' }).click()
    await phoneSignIn(phone, 'E2E Cashier')
    await phone.getByLabel('Search or scan').fill('E2E Milk')
    await phone.getByRole('list', { name: 'Products' }).getByRole('button', { name: /E2E Milk/ }).first().click()
    await phone.getByRole('button', { name: 'Pay ›' }).click()
    await phone.getByRole('button', { name: 'Finish sale' }).click()
    await expect(phone.getByText(/it will send when the PC is back/)).toBeVisible()
    await phone.getByRole('button', { name: 'New sale' }).click()
    await expect(phone.getByLabel('1 waiting')).toBeVisible()

    await phoneCtx.setOffline(false)
    await expect(phone.getByText('Drawer open')).toBeVisible({ timeout: 30_000 })
    const sends = await (await admin.request.get(`${API}/sends`)).json()
    const detail = await (await admin.request.get(`${API}/sends/${sends[0].id}`)).json()
    expect(detail.actions.map((a: { kind: string; status: string }) => `${a.kind}:${a.status}`)).toContain('sale:applied')
  })
})
