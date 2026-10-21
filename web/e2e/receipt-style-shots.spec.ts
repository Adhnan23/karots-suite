import { writeFileSync, mkdirSync } from 'node:fs'
import { expect, test } from './fixtures'

// UI-gate pictures of every receipt style (not a behaviour test): the Appearance
// preview — the exact picture the printer gets — saved as PNG per style, for the
// busy sale at 80 + 58 mm and a refund at 80 mm.
//   SHOTS=rs10 bunx playwright test e2e/receipt-style-shots.spec.ts
// Output: web/shots/<gate>/style-<key>-<slip>-<width>.png
const gate = process.env.SHOTS
test.skip(!gate, 'set SHOTS=<gate> to take UI-gate screenshots')

const STYLES = ['Classic', 'Bold', 'Minimal', 'Ticket', 'Poster', 'Grid', 'Stamp', 'Elegant', 'Cards', 'Sections']

test('every style', async ({ page }) => {
  test.setTimeout(180_000)
  mkdirSync(`shots/${gate}`, { recursive: true })
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0000000001')
  await page.getByLabel('PIN').fill(process.env.POS_SYSTEM_PIN ?? '2273')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/admin/)
  await page.goto('/admin/system/appearance')
  const preview = page.getByRole('img', { name: 'Receipt preview' })
  // Every combination draws a new picture: wait for the src to move on from the last one saved.
  let prev = ''
  const save = async (file: string) => {
    await expect.poll(async () => { const s = (await preview.getAttribute('src')) ?? ''; return s.startsWith('blob:') && s !== prev }, { timeout: 20_000 }).toBe(true)
    prev = (await preview.getAttribute('src'))!
    const bytes = await preview.evaluate(async (i: HTMLImageElement) => Array.from(new Uint8Array(await (await fetch(i.src)).arrayBuffer())))
    writeFileSync(`shots/${gate}/${file}.png`, Buffer.from(bytes))
  }
  const pick = (group: string, name: string) => page.getByRole('group', { name: group }).getByRole('button', { name, exact: true }).click()
  for (const s of STYLES) {
    await page.getByRole('group', { name: 'Receipt style' }).getByRole('button', { name: new RegExp(s) }).click()
    for (const [slip, w] of [['Sale', '80 mm'], ['Sale', '58 mm'], ['Refund', '80 mm']]) {
      await pick('Slip', slip)
      await pick('Roll width', w)
      if (slip === 'Sale') await pick('Sample', 'Busy sale')
      await save(`style-${s.toLowerCase()}-${slip.toLowerCase()}-${w.slice(0, 2)}`)
    }
  }
})
