import type { Page } from './fixtures'

/** Dashboard: Who's on now with a cashier serving, a locked manager and a phone. */
export const presencePrep: Record<string, (page: Page) => Promise<void>> = {
  presence: async (page) => {
    const browser = page.context().browser()!
    const as = async (phone: string, device: string, ua: string, path: string, extra: object = {}, lock = false) => {
      const ctx = await browser.newContext({ userAgent: ua })
      await ctx.request.post('/api/v1/auth/login', { data: { phone, pin: '1234' } })
      if (lock) await ctx.request.post('/api/v1/auth/lock')
      await ctx.request.post('/api/v1/presence', { headers: { 'X-Ks-Device': device }, data: { path, idleSec: 0, cartItems: 0, cartTotal: '', ...extra } })
      await ctx.close()
    }
    await as('0770000002', 'shot-till', 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/130.0 Safari/537.36', '/till', { cartItems: 3, cartTotal: '1250.00' })
    await as('0770000003', 'shot-office', 'Mozilla/5.0 (X11; Linux x86_64; rv:131.0) Gecko/20100101 Firefox/131.0', '/lock', { cartItems: 2, cartTotal: '400.00' }, true)
    await as('0770000001', 'shot-phone', 'Mozilla/5.0 (Linux; Android 14) Chrome/130.0 Mobile Safari/537.36', '/admin/stock', { idleSec: 400 })
    await page.reload()
    if (await page.getByText("Who's on now").count()) await page.getByText("Who's on now").scrollIntoViewIfNeeded()
  },
}
