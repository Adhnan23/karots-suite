import type { Page } from './fixtures'

// AI Assistant (aicatalog) UI-gate preps. Paste mode (the default).
const options = JSON.stringify({ confident: false, options: [
  { name: 'SKF 6201-2Z Deep Groove Ball Bearing', category: 'Bearings > Ball Bearings', specs: '12 × 32 × 10 mm · metal shields', explanation: 'A small sealed bearing used in motorbike wheels, fans and pumps.' },
  { name: 'NTN 6201ZZ Ball Bearing', category: 'Bearings > Ball Bearings', specs: '12 × 32 × 10 mm', explanation: 'The same size from another maker; ZZ means shields on both sides.' },
  { name: '6201-2RS Sealed Bearing', category: 'Bearings', specs: '12 × 32 × 10 mm · rubber seals', explanation: 'Rubber-sealed version that keeps water out.' },
] })
const sure = JSON.stringify({ confident: true, best: { name: 'Anchor Full Cream Milk Powder 400g', category: 'Dairy > Milk Powder', specs: '400 g pack', explanation: 'Powdered full cream milk, mixed with water to drink.' } })

async function newProduct(page: Page, name: string) {
  await page.getByRole('button', { name: 'New product' }).click()
  await page.getByRole('dialog').locator('input[name="prod-name"]').fill(name)
  await page.getByRole('button', { name: 'Identify' }).click()
  await page.getByLabel('Chatbot answer').waitFor()
}

/** SHOTS_PATH=/admin/products: ai-paste (the prompt step), ai-options (a pasted reply's options).
 *  SHOTS_PATH=/admin/inventory/intake: ai-intake (a sure answer on New item). */
export const aicatalogPrep: Record<string, (page: Page) => Promise<void>> = {
  'ai-paste': async (page) => newProduct(page, '6201 ZZ'),
  'ai-options': async (page) => {
    await newProduct(page, '6201 ZZ')
    await page.getByLabel('Chatbot answer').fill(`Here is what I found:\n\`\`\`json\n${options}\n\`\`\``)
    await page.getByLabel('Chatbot answer').press('Enter')
    await page.getByRole('button', { name: 'Use this' }).first().waitFor()
  },
  'ai-intake': async (page) => {
    await page.getByLabel('Scan or search').fill('anchor 400')
    await page.getByLabel('Scan or search').press('Enter')
    await page.getByRole('button', { name: 'Identify' }).click()
    await page.getByLabel('Chatbot answer').fill(sure)
    await page.getByLabel('Chatbot answer').press('Enter')
    await page.getByRole('button', { name: 'Use this' }).waitFor()
  },
}
