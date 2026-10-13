import type { Page } from './fixtures'

// M7.6d UI-gate steps (SHOTS_PATH=/admin/products?q=Samba or /admin/conversions).
export const loosePrep: Record<string, (page: Page) => Promise<void>> = {
  async looseSheet(page) {
    await page.getByText('Samba Rice 25 kg', { exact: true }).locator('visible=true').first().click()
    await page.getByRole('button', { name: /Make a loose version/ }).click()
    const d = page.getByRole('dialog', { name: /loose$/ })
    await d.getByLabel('Ratio').fill('25')
    await d.getByLabel('Loose selling price').fill('300')
    await d.getByLabel('Convert now').fill('1')
  },
  async looseRun(page) {
    const row = page.getByRole('row').or(page.getByRole('listitem')).filter({ hasText: 'Samba Rice 25 kg (loose)' }).locator('visible=true').first()
    await row.getByRole('button', { name: 'Run' }).click()
    await page.getByLabel('Got').fill('24.6')
  },
}
