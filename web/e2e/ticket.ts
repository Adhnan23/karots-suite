import { expect, type Page } from './fixtures'
import type { Browser } from '@playwright/test'

// A sales person (e2e-users.sql) builds a cart on another machine and sends it to
// the cashier on `page`, whose till opens the ticket. What the plugin does at the
// sale must then come out as if the cashier had built that cart.
export async function viaTicket(browser: Browser, page: Page, cashier: string, add: (seller: Page) => Promise<void>) {
  const ctx = await browser.newContext({ viewport: { width: 1366, height: 768 } })
  const seller = await ctx.newPage()
  await seller.goto('/login')
  await seller.getByLabel('Phone').fill('0770000004')
  await seller.getByLabel('PIN').fill('1234')
  await seller.getByRole('button', { name: /sign in/i }).click()
  await seller.waitForURL(/\/till/)
  await seller.waitForLoadState('networkidle')
  await add(seller)
  await seller.getByRole('button', { name: `Send to ${cashier}` }).click()
  await expect(seller.getByRole('group', { name: 'Tickets' })).toBeVisible()
  await ctx.close()
  const tickets = page.getByRole('group', { name: 'Tickets' })
  await expect(tickets).toBeVisible({ timeout: 10_000 })
  await tickets.getByRole('button').first().click()
  await expect(tickets).toHaveCount(0)
}
