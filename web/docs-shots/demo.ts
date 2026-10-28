import type { Browser } from '@playwright/test'
import { test, type Page } from '../e2e/fixtures'

// What the docs screenshots and the promo-video clips share: signing in to a
// running Demo Room's Karots Super Store with the demo chrome hidden.
// DOCS_CODE = the hourly code (unlocks the demo and is the system user's PIN).
export const code = process.env.DOCS_CODE ?? ''
export const only = process.env.DOCS_ONLY ?? ''
export const out = '../docs/images'
export const owner = { phone: '0771000001', pin: '1111' }
export const cashier = { phone: '0772000001', pin: '4444' } // a sim cashier: their till is open
export const system = { phone: '0000000001', pin: code }
export const desk = { width: 1440, height: 900 }
export const phone = { width: 390, height: 844 }

// Hide the demo's own chrome (watermark tab, paper strip, logins) in every shot.
const hideChrome = () => {
  const s = document.createElement('style')
  s.textContent = '.ksd, [class^="ksd-"] { display: none !important }'
  document.addEventListener('DOMContentLoaded', () => document.head.appendChild(s))
}

export async function unlock(page: Page) {
  const r = await page.request.post('/__demo/unlock', { data: { code } })
  if (!r.ok()) throw new Error(`demo unlock: ${r.status()} ${await r.text()}`)
}

export type Who = { phone: string; pin: string }
export async function session(browser: Browser, who: Who | null, viewport = desk, mode: 'light' | 'dark' = 'light', extra: Parameters<Browser['newContext']>[0] = {}) {
  const ctx = await browser.newContext({ viewport, baseURL: test.info().project.use.baseURL, ...extra })
  await ctx.addInitScript((m) => localStorage.setItem('ks-mode', m), mode)
  await ctx.addInitScript(hideChrome)
  const page = await ctx.newPage()
  await unlock(page)
  if (who) {
    let r = await page.request.post('/api/v1/auth/login', { data: who })
    for (let i = 0; r.status() === 429 && i < 10; i++) { // the login limiter (burst 10, +1 per 2s)
      await page.waitForTimeout(2100)
      r = await page.request.post('/api/v1/auth/login', { data: who })
    }
    if (!r.ok()) throw new Error(`login ${who.phone}: ${r.status()} ${await r.text()}`)
  }
  return page
}

// Fill an empty screen so the shot shows the screen at work, not its blank state.
export const pickFirst = (list: string) => async (p: Page) => {
  await p.locator(list.startsWith('ul') ? list : `ul[aria-label="${list}"]`).first().locator('li').first().click()
  await p.waitForLoadState('networkidle')
}
export const findProduct = (q: string, list = 'Products') => async (p: Page) => {
  await p.getByLabel('Find a product').fill(q)
  await p.waitForLoadState('networkidle')
  await pickFirst(list)(p)
}
export const scan = (codes: string[], field: string) => async (p: Page) => {
  for (const c of codes) {
    await p.getByLabel(field).fill(c)
    await p.getByLabel(field).press('Enter')
    await p.waitForLoadState('networkidle')
    await p.waitForTimeout(400)
  }
}

/** Starts the Super Store (every add-on) unless it's already up. */
export async function superStore(page: Page) {
  const st = await (await page.request.get('/__demo/state')).json()
  if (st.phase === 'ready' && st.shop?.key === 'everything') return st
  await page.request.post('/__demo/start', { data: { shop: 'everything' } })
  await test.expect
    .poll(async () => (await (await page.request.get('/__demo/state')).json()).phase, { timeout: 240_000, intervals: [2000] })
    .toBe('ready')
  return st
}
