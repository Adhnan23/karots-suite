import { expect, type Page } from './fixtures'

// Appearance → Receipt style: wait until every thumbnail and the preview are drawn.
export const appearancePrep: Record<string, (page: Page) => Promise<void>> = {
  'appearance-receipts': async (page) => {
    await expect(page.getByRole('img', { name: 'Receipt preview' })).toBeVisible({ timeout: 20_000 })
    await expect(page.getByRole('group', { name: 'Receipt style' }).getByRole('img')).toHaveCount(10, { timeout: 20_000 })
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  },
  // Appearance → Palette with one custom palette (made once, kept for the other shots).
  'appearance-palettes': async (page) => {
    const group = page.getByRole('group', { name: 'Lagoon copy', exact: true })
    if (!(await group.count())) {
      await page.getByRole('group', { name: 'Lagoon', exact: true }).getByRole('button', { name: 'Duplicate' }).click()
      await group.waitFor()
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  },
}
