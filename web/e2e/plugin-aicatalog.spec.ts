import { createServer, type Server } from 'node:http'
import type { AddressInfo } from 'node:net'
import { expect, test, type Page } from './fixtures'
import { cleanupData } from './shot-prep-aicatalog'

// AI Assistant (plugin, built in for Playwright). Needs `make e2e-seed`. Paste mode — the main
// path — for every flow, plus one API-mode flow against a local fake provider (no real key).
test.describe.configure({ mode: 'serial' })

const tag = Date.now().toString().slice(-6)
const base = '/api/v1/x/aicatalog'
const paste = { manualMode: true, provider: 'gemini', baseUrl: 'https://generativelanguage.googleapis.com/v1beta/openai', model: 'gemini-3.6-flash', webSearch: true, namingStyle: '', customPrompt: '' }
const answer = (name: string, category: string, specs = '') =>
  JSON.stringify({ confident: true, best: { name, category, specs, explanation: 'Test item.' } })

async function login(page: Page, to: string) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
  await page.goto(to)
  await page.waitForLoadState('networkidle')
}

const productId = async (page: Page, name: string) =>
  ((await (await page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(name)}`)).json()).items as { id: number; name: string }[]).find((p) => p.name === name)?.id

async function identifyInForm(page: Page, typed: string) {
  const dlg = page.getByRole('dialog')
  await dlg.locator('input[name="prod-name"]').fill(typed)
  await dlg.getByRole('button', { name: 'Identify' }).click()
  await expect(dlg.getByLabel('Chatbot answer')).toBeVisible()
  return dlg
}

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage()
  await login(page, '/admin')
  await page.request.put(`${base}/settings`, { data: paste })
  await page.close()
})

test('identify in the product form: a pasted answer in prose and a fence fills name and a new category; ⓘ after save', async ({ page }) => {
  await login(page, '/admin/products')
  await page.getByRole('button', { name: 'New product' }).click()
  const dlg = await identifyInForm(page, `6201 zz ${tag}`)
  const name = `SKF 6201-2Z Bearing ${tag}`
  await dlg.getByLabel('Chatbot answer').fill(`Sure! Here it is:\n\`\`\`json\n${answer(name, `Bearings ${tag} / Ball`, '12x32x10 mm')}\n\`\`\`\nAnything else?`)
  await dlg.getByLabel('Chatbot answer').press('Enter')
  await expect(dlg.getByText('The AI is sure')).toBeVisible()
  await expect(dlg.getByText('new category')).toBeVisible() // neither level exists yet
  await dlg.getByRole('button', { name: 'Use this' }).click()
  await expect(dlg.locator('input[name="prod-name"]')).toHaveValue(name)
  await expect(dlg.getByRole('button', { name: 'Category' })).toContainText('Ball')
  await expect(dlg.getByText('Filled from the AI')).toBeVisible()
  await dlg.getByLabel('Selling').fill('450')
  await dlg.getByRole('button', { name: 'Add product' }).click()
  await expect(dlg).toBeHidden()
  const id = await productId(page, name)
  const detail = await (await page.request.get(`/api/v1/till/products/${id}/detail`)).json()
  expect(detail.rows).toContainEqual({ label: 'AI hint', value: '12x32x10 mm' })
})

test('hints: corrected on the product page, found and shown at the till', async ({ page }) => {
  test.setTimeout(90_000)
  await login(page, '/admin/products')
  const cat = (await (await page.request.get('/api/v1/admin/categories')).json())[0]
  const name = `BP-114 ${tag}`
  const saved = await page.request.post('/api/v1/admin/products', { data: { name, categoryId: cat.id, unitId: 1, sellingPrice: '450' } })
  const { id } = await saved.json()
  const search = async (q: string) => (await (await page.request.get(`/api/v1/till/products?search=${encodeURIComponent(q)}`)).json()).items as { id: number; line?: string }[]
  expect((await search(`Bajaj${tag}`)).map((p) => p.id)).not.toContain(id)

  await page.getByLabel('Search products').fill(name)
  await page.getByRole('button', { name: new RegExp(`^BP-114 ${tag}`) }).first().click()
  const hint = page.getByRole('region', { name: 'AI hint' })
  await hint.getByLabel('AI hint details').fill(`Fits Bajaj${tag} CT100 <b>front</b> 100% wheels`)
  await hint.getByLabel('What it is').fill('A brake pad')
  await hint.getByRole('button', { name: 'Save hint' }).click()
  await expect(hint.getByRole('button', { name: 'Save hint' })).toBeHidden()

  const found = await search(`Bajaj${tag}`)
  expect(found.map((p) => p.id)).toContain(id)
  expect(found.find((p) => p.id === id)?.line).toContain('CT100')
  expect((await search(`ct100 Bajaj${tag}`)).map((p) => p.id)).toContain(id)
  const detail = await (await page.request.get(`/api/v1/till/products/${id}/detail`)).json()
  expect(detail.rows).toContainEqual({ label: 'AI hint', value: `Fits Bajaj${tag} CT100 <b>front</b> 100% wheels` })
  expect(detail.rows).toContainEqual({ label: 'What it is', value: 'A brake pad' })
  expect(detail.rows).toContainEqual({ label: 'Note', value: 'AI suggestion, confirm before selling' })
  expect((await page.request.put(`${base}/items/99999999`, { data: { specs: 'x', explanation: '' } })).status()).toBe(404)

  // On the till itself: a customer asks for the bike, the tile comes up and says why.
  const st = await (await page.request.get('/api/v1/till/drawer')).json()
  if (!st.summary.session) await page.request.post('/api/v1/till/drawer/open', { data: { breakdown: { '500': 2 }, sourceLockerId: 0 } })
  await page.goto('/till')
  await page.getByPlaceholder(/scan|search/i).first().fill(`Bajaj${tag}`)
  await expect(page.getByRole('button', { name: new RegExp(`BP-114 ${tag}`) }).first()).toContainText('CT100')
  await expect(page.locator('b', { hasText: 'front' })).toHaveCount(0) // shown as plain text
  if (process.env.SHOTS) await page.screenshot({ path: `shots/${process.env.SHOTS}/till-hint.png` })
})

test('jobs: identify a whole list: unknown items wait, ticks apply, the next batch moves on, undo puts it back', async ({ page }) => {
  await login(page, '/admin/x/aicatalog/jobs')
  const cat = (await (await page.request.get('/api/v1/admin/categories')).json())[0]
  const add = async (name: string) => (await (await page.request.post('/api/v1/admin/products', { data: { name, categoryId: cat.id, unitId: 1 } })).json()).id as number
  const a = await add(`pn-${tag}-a`)
  const b = await add(`pn-${tag}-b`)
  await page.getByRole('button', { name: /Identify a whole list/ }).click()
  await page.getByRole('button', { name: 'Category', exact: false }).first().click()
  await page.getByLabel('Search categories').fill(cat.name)
  await page.getByLabel('Search categories').press('Enter')
  await page.getByRole('button', { name: 'Start' }).click()
  const prompt = await page.getByLabel('Prompt to copy').inputValue()
  expect(prompt).toContain(`pn-${tag}-a`)
  expect(prompt).not.toContain('spare-part')
  const reply = JSON.stringify({ items: [
    { id: a, confident: true, name: `Brake Pad ${tag}`, category: cat.name, specs: `Fits Hint${tag} (1/2")`, explanation: 'A brake pad' },
    { id: b, confident: false, name: 'Unknown Product', category: 'Unclassified', specs: 'cannot tell', explanation: '' },
  ] })
  await page.getByLabel('Chatbot answer').fill(reply)
  await page.getByLabel('Chatbot answer').press('Enter')
  const hintBox = page.getByRole('checkbox', { name: new RegExp(`^Hint: Fits Hint${tag}`) })
  await expect(hintBox).toBeChecked()
  const nameBox = page.getByRole('checkbox', { name: new RegExp(`^Name: Brake Pad ${tag}`) })
  await expect(nameBox).not.toBeChecked()
  await expect(page.getByText(`pn-${tag}-b`)).toBeVisible() // named under "No answer for"
  await nameBox.check()
  await page.getByRole('button', { name: /^Apply 2/ }).click()
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: '2 applied' }).first()).toBeVisible()

  const found = (await (await page.request.get(`/api/v1/till/products?search=Hint${tag}`)).json()).items as { id: number; name: string }[]
  expect(found.map((p) => p.name)).toEqual([`Brake Pad ${tag}`])
  const old = (await (await page.request.get(`/api/v1/till/products?search=pn-${tag}-a`)).json()).items as { id: number }[]
  expect(old.map((p) => p.id)).toContain(a) // the old code still finds it

  await page.getByRole('button', { name: 'Undo it' }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Undo' }).click()
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: 'Undone' }).first()).toBeVisible()
  const back = await (await page.request.get(`/api/v1/admin/products/${a}`)).json()
  expect(back.name).toBe(`pn-${tag}-a`)
})

test('a bad paste keeps the text and says why', async ({ page }) => {
  await login(page, '/admin/products')
  await page.getByRole('button', { name: 'New product' }).click()
  const dlg = await identifyInForm(page, 'mystery part')
  await dlg.getByLabel('Chatbot answer').fill("I'm not sure what that is, sorry.")
  await dlg.getByLabel('Chatbot answer').press('Enter')
  await expect(dlg.getByRole('alert')).toContainText('paste the whole reply')
  await expect(dlg.getByLabel('Chatbot answer')).toHaveValue("I'm not sure what that is, sorry.")
  // A chat that already has the rules (and the shop's own tweaks) gets just the item.
  await dlg.getByRole('button', { name: 'Short prompt (chat already set up)' }).click()
  await expect(dlg.getByLabel('Prompt to copy')).toHaveValue(/^Next item: mystery part/)
  await expect(dlg.getByRole('button', { name: 'Full prompt (new chat)' })).toBeVisible()
})

test('identify from Items to review and from intake', async ({ page }) => {
  await login(page, '/admin/products/review')
  await page.request.post('/api/v1/till/quick-item', { data: { name: `AI review ${tag}`, price: '75', qty: '1', barcode: '', unitId: 0 } })
  await page.reload()
  await page.getByRole('button', { name: new RegExp(`^AI review ${tag}`) }).first().click()
  await expect(page.getByRole('dialog').getByRole('button', { name: 'Identify' })).toBeVisible()
  await page.keyboard.press('Escape')

  await page.goto('/admin/inventory/intake')
  await page.getByLabel('Scan or search').fill(`anchor ${tag}`)
  await page.getByLabel('Scan or search').press('Enter')
  await page.getByRole('button', { name: 'Identify' }).click()
  const name = `Anchor Milk Powder 400g ${tag}`
  await page.getByLabel('Chatbot answer').fill(answer(name, `Dairy ${tag}`))
  await page.getByLabel('Chatbot answer').press('Enter')
  await page.getByRole('button', { name: 'Use this' }).click()
  await expect(page.locator('input[name="in-name"]')).toHaveValue(name)
  await page.getByLabel('Selling').fill('980')
  await page.getByLabel('Opening quantity').fill('2')
  await page.getByRole('button', { name: /^Create(,| &) stock/ }).click()
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: `${name} added` })).toBeVisible()
  const id = await productId(page, name)
  const detail = await (await page.request.get(`/api/v1/till/products/${id}/detail`)).json()
  expect(detail.category).toBe(`Dairy ${tag}`)
  expect(detail.rows).toContainEqual({ label: 'What it is', value: 'Test item.' })
  expect(detail.rows.map((r: { label: string }) => r.label)).not.toContain('AI hint') // this answer had no specs
})

test('coming back from the chatbot reads its answer from the clipboard', async ({ page, context }) => {
  await context.grantPermissions(['clipboard-read', 'clipboard-write'])
  await login(page, '/admin/products')
  await page.getByRole('button', { name: 'New product' }).click()
  const dlg = await identifyInForm(page, `clip ${tag}`)
  await dlg.getByRole('button', { name: 'Copy prompt' }).click() // the default: copy only, no new tab
  expect(context.pages()).toHaveLength(1)
  expect(await page.evaluate(() => navigator.clipboard.readText())).toContain(`clip ${tag}`) // the prompt was copied
  await page.evaluate((t) => navigator.clipboard.writeText(t), `Here you go: ${answer(`Clip Item ${tag}`, 'Misc')}`)
  await page.evaluate(() => window.dispatchEvent(new Event('focus')))
  await expect(dlg.getByRole('button', { name: 'Use this' })).toBeVisible()
  await expect(dlg.getByText(`Clip Item ${tag}`)).toBeVisible()
})

test('clean-up: one change unticked, apply, then undo; a duplicate opens the merge confirm', async ({ page }) => {
  await login(page, '/admin/x/aicatalog')
  const pre = `AI clean ${tag}`
  const reply = await cleanupData(page, pre)
  const names = async () => ((await (await page.request.get(`/api/v1/admin/products?q=${encodeURIComponent(pre)}`)).json()).items as { name: string; categoryName: string }[])
  await page.getByRole('button', { name: 'Start a clean-up' }).click()
  await page.getByLabel('Chatbot answer').fill(reply)
  await page.getByLabel('Chatbot answer').press('Enter')
  await page.getByRole('region', { name: 'Names' }).getByRole('checkbox').first().uncheck()
  await page.getByRole('button', { name: 'Apply 2 ticked' }).click()
  await expect(page.getByText('1 category merged, 1 uncategorized placed').first()).toBeVisible()
  let rows = await names()
  expect(rows.find((p) => p.name === `${pre} Sunlight soap`)?.categoryName).toBe('Soap')
  expect(rows.find((p) => p.name === `${pre} sprite 500ml`)).toBeTruthy() // unticked: not renamed

  await page.getByRole('region', { name: 'Duplicates' }).getByRole('button', { name: 'Merge…' }).click()
  await expect(page.getByRole('dialog')).toContainText(`Merge into ${pre}`)
  await page.keyboard.press('Escape')

  await page.getByRole('button', { name: /^(Done|Stop here)$/ }).click() // Stop here when the shop has more parts
  await page.getByRole('button', { name: /Undo last clean-up/ }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Undo' }).click()
  await expect(page.locator('[data-sonner-toast]').filter({ hasText: 'Undone' })).toBeVisible()
  rows = await names()
  expect(rows.find((p) => p.name === `${pre} Sunlight soap`)?.categoryName).toBe('Uncategorized')
  expect(rows.find((p) => p.name === `${pre} Coke 1L`)?.categoryName).toBe(`${pre} drink`)
})

test('your own chatbot: added in Settings, the Copy button opens it; removed, back to Copy only', async ({ page }) => {
  await login(page, '/admin/x/aicatalog/settings')
  await page.getByRole('button', { name: 'Add chatbot' }).click()
  await page.getByLabel('Chatbot name').fill('My Gem')
  await page.getByLabel('Chatbot address').fill('javascript:alert(1)')
  await expect(page.getByText('Use a web address starting with https://')).toBeVisible()
  await page.getByLabel('Chatbot address').fill('https://gemini.google.com/gem/shop-namer')
  await page.getByRole('button', { name: 'Add', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Copy & open My Gem' })).toHaveAttribute('aria-pressed', 'true')
  await page.goto('/admin/products')
  await page.getByRole('button', { name: 'New product' }).click()
  const dlg = await identifyInForm(page, 'gem test')
  await expect(dlg.getByRole('button', { name: 'Copy & open My Gem' })).toBeVisible()
  await page.goto('/admin/x/aicatalog/settings')
  await page.getByRole('button', { name: 'Remove My Gem' }).click()
  await expect(page.getByRole('button', { name: 'Copy only' })).toHaveAttribute('aria-pressed', 'true')
})

test('settings and clean-up fit a phone', async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 })
  for (const to of ['/admin/x/aicatalog', '/admin/x/aicatalog/settings']) {
    await login(page, to)
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(360)
  }
})

test.describe('API mode', () => {
  let server: Server
  let fail = false
  test.beforeAll(async () => {
    server = createServer((req, res) => {
      req.resume()
      req.on('end', () => {
        res.setHeader('Content-Type', 'application/json')
        if (fail) {
          res.statusCode = 500
          res.end(JSON.stringify({ error: { message: 'model overloaded' } }))
          return
        }
        res.end(JSON.stringify({ choices: [{ message: { role: 'assistant', content: answer(`API Item ${tag}`, 'Misc') } }] }))
      })
    })
    await new Promise<void>((ok) => server.listen(0, '127.0.0.1', ok))
  })
  test.afterAll(async ({ browser }) => {
    server.close()
    const page = await browser.newPage()
    await login(page, '/admin')
    await page.request.put(`${base}/settings`, { data: paste })
    await page.close()
  })

  test('the AI answers directly; when it fails, the same panel offers the prompt', async ({ page }) => {
    await login(page, '/admin/products')
    const port = (server.address() as AddressInfo).port
    expect((await page.request.put(`${base}/settings`, { data: { ...paste, manualMode: false, provider: 'openai', baseUrl: `http://127.0.0.1:${port}/v1`, model: 'fake' } })).ok()).toBe(true)
    await page.getByRole('button', { name: 'New product' }).click()
    const dlg = page.getByRole('dialog')
    await dlg.locator('input[name="prod-name"]').fill('api thing')
    await dlg.getByRole('button', { name: 'Identify' }).click()
    await expect(dlg.getByText(`API Item ${tag}`)).toBeVisible()
    await dlg.getByRole('button', { name: 'Close identify' }).click()

    fail = true
    await dlg.getByRole('button', { name: 'Identify' }).click()
    await expect(dlg.getByRole('alert')).toContainText('model overloaded')
    await expect(dlg.getByRole('button', { name: 'Copy prompt' })).toBeVisible()
  })
})
