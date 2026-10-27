import { expect, type Page } from './fixtures'

// Appearance → Receipt style: wait until every thumbnail and the preview are drawn.
export const appearancePrep: Record<string, (page: Page) => Promise<void>> = {
  'appearance-receipts': async (page) => {
    await page.locator('summary', { hasText: 'Receipt style' }).click()
    await expect(page.getByRole('img', { name: 'Receipt preview' })).toBeVisible({ timeout: 20_000 })
    await expect(page.getByRole('group', { name: 'Receipt style' }).getByRole('img')).toHaveCount(10, { timeout: 20_000 })
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  },
  // Appearance → Labels: every price-style thumbnail and the preview drawn, scrolled into view.
  'appearance-labels': async (page) => {
    await page.locator('summary', { hasText: 'Label styles' }).click()
    const labels = page.getByRole('region', { name: 'Label styles' })
    await labels.evaluate((el) => el.scrollIntoView({ block: 'start' }))
    await expect(labels.getByRole('img', { name: 'Label preview' })).toBeVisible({ timeout: 20_000 })
    await expect(labels.getByRole('group', { name: 'Label style' }).locator('[data-label-style]')).toHaveCount(8, { timeout: 20_000 })
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
    await labels.evaluate((el) => el.scrollIntoView({ block: 'start' }))
  },
  // Settings → Printers & labels with Image picked (unsaved): the label preview is the picture.
  'settings-labels': async (page) => {
    const sec = page.getByRole('region', { name: 'Printers & labels' })
    await sec.getByRole('radiogroup', { name: 'Print labels as' }).getByRole('radio', { name: /Image/ }).click()
    await expect(sec.getByRole('img', { name: 'Label as it prints' })).toHaveAttribute('src', /^blob:/, { timeout: 20_000 })
    await sec.getByRole('radiogroup', { name: 'Print labels as' }).scrollIntoViewIfNeeded()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  },
  // Appearance → Palette with one custom palette (made once, kept for the other shots).
  'appearance-palettes': async (page) => {
    const group = page.getByRole('group', { name: 'Lagoon copy', exact: true })
    if (!(await group.count())) {
      await page.getByRole('group', { name: 'Lagoon', exact: true }).getByRole('button', { name: 'Duplicate' }).click()
      await page.getByRole('button', { name: 'Back to palettes' }).click() // Duplicate opens the editor
      await group.waitFor()
    }
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  },
  // The palette editor on a copy of Lagoon.
  'palette-editor': async (page) => {
    await appearancePrep['appearance-palettes'](page)
    await page.getByRole('group', { name: 'Lagoon copy', exact: true }).getByRole('button', { name: 'Edit' }).click()
    await page.getByRole('button', { name: 'Complete sale' }).waitFor()
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(page.viewportSize()!.width)
  },
}
