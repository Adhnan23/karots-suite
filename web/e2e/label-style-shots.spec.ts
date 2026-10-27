import { mkdirSync, writeFileSync } from 'node:fs'
import { expect, test } from './fixtures'

// UI-gate pictures of every label style (not a behaviour test): the Appearance
// preview — the exact picture the printer gets — per kind, style and size.
//   SHOTS=ls8 bunx playwright test e2e/label-style-shots.spec.ts
// Output: web/shots/<gate>/<kind>-<style>-<size>.png
const gate = process.env.SHOTS
test.skip(!gate, 'set SHOTS=<gate> to take UI-gate screenshots')

const KINDS: [string, string[], string[]][] = [
  ['Price label', ['Classic', 'Price Tag', 'Inverse', 'QR Tag', 'Dual', 'Ticket', 'Minimal', 'Shelf'], ['50 × 25', '38 × 25', '100 × 50']],
  ['Dose label', ['Classic', 'Clear', 'Boxed'], ['50 × 30', '50 × 25']],
  ['Repair ticket', ['Classic', 'Job'], ['50 × 25', '38 × 25']],
]

test('every label style', async ({ page }) => {
  test.setTimeout(240_000)
  mkdirSync(`shots/${gate}`, { recursive: true })
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0000000001')
  await page.getByLabel('PIN').fill(process.env.POS_SYSTEM_PIN ?? '2273')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/admin/)
  await page.goto('/admin/system/appearance')
  await page.locator('summary', { hasText: 'Label styles' }).click() // starts closed
  const labels = page.getByRole('region', { name: 'Label styles' })
  const preview = labels.getByRole('img', { name: 'Label preview' })
  let prev = ''
  const save = async (file: string) => {
    await expect.poll(async () => { const s = (await preview.getAttribute('src')) ?? ''; return s.startsWith('blob:') && s !== prev }, { timeout: 20_000 }).toBe(true)
    prev = (await preview.getAttribute('src'))!
    const bytes = await preview.evaluate(async (i: HTMLImageElement) => Array.from(new Uint8Array(await (await fetch(i.src)).arrayBuffer())))
    writeFileSync(`shots/${gate}/${file}.png`, Buffer.from(bytes))
  }
  const pick = (group: string, name: string) => labels.getByRole('group', { name: group }).getByRole('button', { name, exact: true }).click()
  const slug = (s: string) => s.toLowerCase().replace(/ × /g, 'x').replace(/\s+/g, '')
  for (const [kind, styles, sizes] of KINDS) {
    await pick('Label kind', kind)
    for (const s of styles)
      for (const size of sizes) {
        await labels.getByRole('group', { name: 'Label style' }).getByRole('button', { name: new RegExp(`^${s}( Too small|$)`) }).click()
        await pick('Label size', size)
        await save(`${slug(kind)}-${slug(s)}-${slug(size)}`)
      }
  }
})

// A Sinhala name on the A4 sheet in every price style (Image mode, vector).
test('a Sinhala name, every price style, on the A4 sheet', async ({ page }) => {
  test.setTimeout(120_000)
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0000000001')
  await page.getByLabel('PIN').fill(process.env.POS_SYSTEM_PIN ?? '2273')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/admin/)
  const settings = async (labelPrintAs: string) => {
    const s = (await (await page.request.get('/api/v1/admin/settings')).json()).settings
    expect((await page.request.put('/api/v1/admin/settings', { data: { ...s, labelPrintAs } })).ok()).toBe(true)
  }
  const style = async (price: string) => {
    const a = await (await page.request.get('/api/v1/system/appearance')).json()
    expect((await page.request.put('/api/v1/system/appearance', { data: { ...a, labelStyles: { price } } })).ok()).toBe(true)
  }
  await settings('image')
  try {
    for (const s of ['classic', 'pricetag', 'inverse', 'qrtag', 'dual', 'ticket', 'minimal', 'shelf']) {
      await style(s)
      for (const size of ['50x25', '100x50']) {
        await page.goto(`/print/labels?code=4792024012348&format=EAN13&top=${encodeURIComponent('ඇන්කර් කිරි පිටි 400g')}&bottom=${encodeURIComponent('Rs 1,250.00')}&topKey=name&bottomKey=price&count=1&size=${size}`)
        const one = page.getByRole('img', { name: 'Label 1' })
        await expect(one.locator('[data-label-style]')).toHaveCount(1)
        await page.evaluate(() => document.fonts.ready)
        await one.screenshot({ path: `shots/${gate}/sinhala-${s}-${size}.png` })
      }
    }
  } finally {
    await style('classic')
    await settings('classic')
  }
})
