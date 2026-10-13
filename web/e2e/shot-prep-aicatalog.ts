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

type Cat = { id: number; name: string }
/** Fixed shot data: a near-duplicate category pair, an uncategorized item and a look-alike pair. */
export async function cleanupData(page: Page, pre = 'AI shot') {
  const cats = (await (await page.request.get('/api/v1/admin/categories')).json()) as Cat[]
  const cat = async (name: string) => cats.find((c) => c.name === name)?.id
    ?? (await (await page.request.post('/api/v1/admin/categories/quick', { data: { name, parentId: 0 } })).json()).id as number
  const prods = (await (await page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(pre)}&pageSize=50`)).json()).items as { id: number; name: string }[]
  const prod = async (name: string, categoryId: number) => prods.find((p) => p.name === name)?.id
    ?? (await (await page.request.post('/api/v1/admin/products', { data: {
      name, nameLocal: '', barcode: '', categoryId, unitId: 1, costPrice: '40', sellingPrice: '60', wholesalePrice: '', taxRate: '',
      reorderLevel: 0, trackSerial: false, warrantyMonths: 0, supplierId: 0, repriceStock: false, dupSourceName: '',
    } })).json()).id as number
  const drinks = await cat(`${pre} Drinks`), drink = await cat(`${pre} drink`), uncat = await cat('Uncategorized')
  const ids = {
    drinks, drink,
    coke: await prod(`${pre} Coke 1L`, drink), cola: await prod(`${pre} Coca Cola 1 Ltr`, drinks),
    soap: await prod(`${pre} Sunlight soap`, uncat), sprite: await prod(`${pre} sprite 500ml`, drinks),
  }
  return JSON.stringify({
    renames: [], reparents: [],
    merges: [{ from: ids.drink, into: ids.drinks, reason: 'Same category — one is singular and lower case.' }],
    moves: [],
    uncategorized: [{ product_id: ids.soap, category: 'Household > Soap', reason: 'A bar soap for washing clothes.' }],
    names: [{ product_id: ids.sprite, to: `${pre} Sprite 500ml`, reason: 'Brand names start with a capital.' }],
    duplicates: [{ a: ids.coke, b: ids.cola, reason: 'Both are a 1 litre bottle of Coca-Cola.' }],
  })
}

/** SHOTS_PATH=/admin/x/aicatalog: ai-cleanup (a pasted plan, one change unticked).
 *  SHOTS_PATH=/admin/x/aicatalog/settings: ai-settings-api (the API fields). */
Object.assign(aicatalogPrep, {
  'ai-cleanup': async (page: Page) => {
    const reply = await cleanupData(page)
    await page.getByRole('button', { name: 'Start a clean-up' }).click()
    await page.getByLabel('Chatbot answer').fill(reply)
    await page.getByLabel('Chatbot answer').press('Enter')
    await page.getByRole('region', { name: 'Names' }).getByRole('checkbox').first().uncheck()
  },
  'ai-settings-api': async (page: Page) => {
    await page.getByRole('button', { name: /^API/ }).click()
    await page.getByLabel('Provider').selectOption('gemini')
  },
})
