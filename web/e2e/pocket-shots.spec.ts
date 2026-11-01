import { expect, test, type BrowserContext, type Page } from '@playwright/test'

// UI-gate screenshots of Pocket Cashier (not a behaviour test). Needs `make e2e-seed`
// data and a server with the plugin built in.
//   SHOTS=pocket bunx playwright test e2e/pocket-shots.spec.ts
// Output: web/shots/<gate>/<screen>-<palette>-<mode>.png
const gate = process.env.SHOTS
test.skip(!gate, 'set SHOTS=<gate> to take UI-gate screenshots')
test.describe.configure({ mode: 'serial' })
test.use({ launchOptions: { args: ['--ignore-certificate-errors'] } })
test.setTimeout(180_000)

const APP = 'https://localhost:8443/x/pocketcashier/'
let ctx: BrowserContext
let phone: Page

async function login(page: Page, phoneNo: string, pin: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill(phoneNo)
  await page.getByLabel('PIN').fill(pin)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(admin|till)/)
}

async function setPalette(page: Page, palette: string) {
  await login(page, '0000000001', process.env.POS_SYSTEM_PIN ?? '2273')
  const cur = await (await page.request.get('/api/v1/system/appearance')).json()
  expect((await page.request.put('/api/v1/system/appearance', { data: { ...cur, palette } })).ok()).toBe(true)
  await page.context().clearCookies()
}

async function pinIn(p: Page, name: string) {
  await p.getByRole('radiogroup', { name: 'Who are you?' }).getByRole('radio', { name }).click()
  for (const k of '1234') await p.getByRole('button', { name: k, exact: true }).click()
  await p.getByRole('button', { name: 'Sign in' }).click()
  await expect(p.getByRole('navigation', { name: 'Phone tabs' })).toBeVisible()
}

const shot = (p: Page, name: string, palette: string, mode: string) =>
  p.screenshot({ path: `shots/${gate}/${name}-${palette}-${mode}.png` })

test.beforeAll(async ({ browser }) => {
  const admin = await browser.newPage()
  await login(admin, '0770000001', '1234')
  ctx = await browser.newContext({ viewport: { width: 390, height: 844 }, ignoreHTTPSErrors: true, isMobile: true, hasTouch: true })
  phone = await ctx.newPage()
  await phone.goto(APP)
  await phone.getByLabel('This phone’s name').fill('Back store 1')
  await phone.getByRole('button', { name: 'Pair this phone' }).click()
  const code = (await phone.getByLabel('Pairing code').textContent())!.trim()
  await admin.request.post('/api/v1/x/pocketcashier/phones/approve', { data: { code } })
  await pinIn(phone, 'E2E Cashier')
  // A float in the drawer and one held sale, so Menu and Held aren't empty.
  await phone.getByRole('button', { name: 'Menu', exact: true }).click()
  await phone.getByRole('button', { name: 'Open the drawer' }).click()
  await phone.getByLabel('Float in the drawer').fill('5000')
  await phone.getByRole('button', { name: 'Open', exact: true }).click()
  await phone.getByRole('button', { name: 'Sell', exact: true }).click()
  await phone.getByRole('list', { name: 'Products' }).getByRole('button', { name: /E2E Yoghurt/ }).first().click()
  await phone.getByRole('dialog', { name: 'Which price?' }).getByRole('button').filter({ hasNotText: 'Expired' }).first().click() // its FEFO lot is expired
  await phone.getByRole('button', { name: 'Held', exact: true }).click()
  await phone.getByLabel('Name for the held sale').fill('Ravi — back later')
  await phone.getByRole('button', { name: 'Hold', exact: true }).click()
  await admin.close()
})
test.afterAll(async () => { await ctx?.close() })

for (const palette of (process.env.SHOTS_PALETTES ?? 'studio,night,bazaar').split(',')) {
  for (const mode of ['light', 'dark']) {
    test(`${palette} ${mode}`, async ({ browser }) => {
      const sys = await browser.newPage()
      await setPalette(sys, palette)
      await sys.close()
      await phone.evaluate((m) => localStorage.setItem('ks-mode', m), mode)
      await phone.goto(APP)
      await expect(phone.getByRole('navigation', { name: 'Phone tabs' })).toBeVisible()

      // Sell with a cart, then Pay.
      await phone.getByRole('button', { name: 'Sell', exact: true }).click()
      const products = phone.getByRole('list', { name: 'Products' })
      await products.getByRole('button', { name: /E2E Milk$|E2E Milk\b/ }).first().click()
      await products.getByRole('button', { name: /E2E Lot Soap/ }).first().click()
      const which = phone.getByRole('dialog', { name: 'Which price?' })
      await which.waitFor()
      await phone.waitForTimeout(300)
      await shot(phone, 'which-price', palette, mode)
      await which.getByRole('button', { name: /120\.00/ }).click()
      await expect(which).toBeHidden()
      await shot(phone, 'sell', palette, mode)
      await phone.getByRole('button', { name: /items? ·/ }).click()
      await shot(phone, 'cart', palette, mode)
      await phone.getByRole('button', { name: /Edit E2E Milk/ }).first().click()
      await phone.waitForTimeout(300)
      await shot(phone, 'line', palette, mode)
      await phone.getByRole('button', { name: 'Done' }).click()
      await phone.getByRole('button', { name: /items? ·/ }).click()
      await phone.getByRole('button', { name: 'Pay ›' }).click()
      await phone.getByRole('group', { name: 'Notes handed over' }).getByRole('button').last().click()
      await shot(phone, 'pay', palette, mode)
      await phone.getByRole('status').filter({ hasText: 'Give change' }).scrollIntoViewIfNeeded()
      await shot(phone, 'pay-change', palette, mode)
      await phone.getByRole('button', { name: 'Back to items' }).click()
      await phone.getByRole('button', { name: 'Held', exact: true }).click()
      await shot(phone, 'held', palette, mode)
      await phone.getByRole('button', { name: 'Menu', exact: true }).click()
      await shot(phone, 'menu', palette, mode)

      // PC down: the amber state.
      await ctx.setOffline(true)
      await phone.reload()
      await phone.getByRole('button', { name: 'Sell', exact: true }).click()
      await expect(phone.getByRole('note')).toBeVisible({ timeout: 30_000 })
      await shot(phone, 'pc-down', palette, mode)
      await ctx.setOffline(false)

      // Sign-in (switch person), then back in.
      await phone.getByRole('button', { name: 'E2E Cashier — menu' }).click()
      await phone.getByRole('menuitem', { name: 'Switch person' }).click()
      await phone.getByRole('radiogroup', { name: 'Who are you?' }).getByRole('radio', { name: 'E2E Cashier' }).click()
      await phone.waitForTimeout(300)
      await shot(phone, 'sign-in', palette, mode)
      for (const k of '1234') await phone.getByRole('button', { name: k, exact: true }).click()
      await phone.getByRole('button', { name: 'Sign in' }).click()
      await expect(phone.getByRole('navigation', { name: 'Phone tabs' })).toBeVisible()
      await phone.getByRole('button', { name: 'Sell', exact: true }).click()
      // Empty the cart for the next round.
      await phone.evaluate(() => Object.keys(localStorage).filter((k) => k.startsWith('ks-till-cart-')).forEach((k) => localStorage.removeItem(k)))

      // The setup page (plain HTTP) and the admin Phones page at 360 px.
      const other = await browser.newContext({ viewport: { width: 390, height: 844 } })
      const setup = await other.newPage()
      await setup.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
      await setup.goto('/x/pocketcashier/setup')
      await setup.getByRole('heading', { name: /Set up this phone/ }).waitFor()
      await shot(setup, 'setup', palette, mode)
      await other.close()
      const adm = await browser.newContext({ viewport: { width: 360, height: 800 } })
      const ap = await adm.newPage()
      await ap.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
      await login(ap, '0770000001', '1234')
      await ap.goto('/admin/x/pocketcashier')
      await ap.getByRole('list', { name: 'Phones' }).waitFor()
      await ap.waitForTimeout(500)
      await ap.screenshot({ path: `shots/${gate}/phones-${palette}-${mode}.png`, fullPage: true })
      await adm.close()
    })
  }
}
