import { expect, test, type Page } from './fixtures'

// Custom palettes (docs/specs/2026-10-21-palettes-design.md). Every test puts Studio back.
const SYSTEM_PIN = process.env.POS_SYSTEM_PIN ?? '2273'

async function systemLogin(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0000000001')
  await page.getByLabel('PIN').fill(SYSTEM_PIN)
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/admin/)
}

const BLUE = '#123456'
const css = (id: number) =>
  `/* karots-palette theme=dock name=E2E Blue base=studio */\n` +
  `:root[data-ui="dock"][data-palette="custom-${id}"] { --background: ${BLUE}; }\n` +
  `:root[data-ui="dock"][data-palette="custom-${id}"][data-theme="dark"] { --background: ${BLUE}; }\n`

test('the active custom palette shows on the login page, before and after sign-in', async ({ page, browser }) => {
  await systemLogin(page)
  const made = await (await page.request.post('/api/v1/system/palettes', { data: { name: 'E2E Blue', base: 'studio', css: '' } })).json()
  await page.request.put(`/api/v1/system/palettes/${made.id}`, { data: { name: 'E2E Blue', base: 'studio', css: css(made.id) } })
  await page.request.put('/api/v1/system/appearance', { data: { palette: `custom-${made.id}`, receiptStyle: 'classic' } })
  try {
    // This device signed in before: the custom palette paints with the first frame.
    await page.goto('/admin')
    await expect(page.locator('html')).toHaveAttribute('data-palette', `custom-${made.id}`)
    await page.request.post('/api/v1/auth/logout')
    await page.goto('/login', { waitUntil: 'domcontentloaded' })
    expect(await page.evaluate(() => getComputedStyle(document.body).backgroundColor)).toBe('rgb(18, 52, 86)')

    // A device that never signed in learns it from login-info.
    const fresh = await browser.newPage()
    await fresh.goto('/login')
    await expect.poll(() => fresh.evaluate(() => getComputedStyle(document.body).backgroundColor)).toBe('rgb(18, 52, 86)')
    await fresh.close()
  } finally {
    await systemLogin(page)
    await page.request.put('/api/v1/system/appearance', { data: { palette: 'studio', receiptStyle: 'classic' } })
    await page.request.delete(`/api/v1/system/palettes/${made.id}`)
  }
})

test('system user duplicates, exports, imports, uses and deletes palettes', async ({ page }) => {
  await systemLogin(page)
  await page.goto('/admin/system/appearance')
  const studio = page.getByRole('group', { name: 'Studio', exact: true })
  await expect(studio.getByText('In use')).toBeVisible()
  await expect(studio.getByRole('button', { name: /delete/i })).toHaveCount(0) // built-ins never delete or export
  await expect(studio.getByRole('button', { name: /export/i })).toHaveCount(0)

  await studio.getByRole('button', { name: 'Duplicate' }).click()
  await page.getByRole('button', { name: 'Back to palettes' }).click() // Duplicate opens the editor
  const copy = page.getByRole('group', { name: 'Studio copy', exact: true })
  await expect(copy).toBeVisible()
  const download = page.waitForEvent('download')
  await copy.getByRole('button', { name: 'Export' }).click()
  const file = await (await download).path()
  const fs = await import('node:fs')
  const text = fs.readFileSync(file, 'utf8')
  expect(text).toContain('name=Studio copy')

  await copy.getByRole('button', { name: 'Delete' }).click()
  await page.getByRole('button', { name: 'Delete palette' }).click()
  await expect(copy).toHaveCount(0)
  await page.getByLabel('Import a palette').setInputFiles({ name: 'studio-copy.css', mimeType: 'text/css', buffer: Buffer.from(text) })
  await expect(copy).toBeVisible()

  // Another theme's file is refused.
  await page.getByLabel('Import a palette').setInputFiles({ name: 'x.css', mimeType: 'text/css', buffer: Buffer.from(text.replace('theme=dock', 'theme=other').replace('name=Studio copy', 'name=Other')) })
  await expect(page.getByText('This palette is for another theme')).toBeVisible()

  await page.getByRole('group', { name: 'Bazaar', exact: true }).getByRole('button', { name: 'Use' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-palette', 'bazaar')
  await copy.getByRole('button', { name: 'Use' }).click()
  await expect(copy.getByText('In use')).toBeVisible()
  await expect(copy.getByRole('button', { name: 'Delete' })).toHaveCount(0) // not the active one

  await studio.getByRole('button', { name: 'Use' }).click()
  await copy.getByRole('button', { name: 'Delete' }).click()
  await page.getByRole('button', { name: 'Delete palette' }).click()
  await expect(copy).toHaveCount(0)
})

test('palette editor previews live, asks before dropping changes, and saves', async ({ page }) => {
  await systemLogin(page)
  await page.addInitScript(() => localStorage.setItem('ks-mode', 'light'))
  await page.goto('/admin/system/appearance')
  await page.getByRole('group', { name: 'Night', exact: true }).getByRole('button', { name: 'Duplicate' }).click()
  const brand = page.getByLabel('Brand', { exact: true })
  const sale = page.getByRole('button', { name: 'Complete sale' })
  await brand.fill('#ff0000')
  await expect(sale).toHaveCSS('background-color', 'rgb(255, 0, 0)')

  // Back with unsaved changes asks; leaving drops them.
  await page.getByRole('button', { name: 'Back to palettes' }).click()
  await page.getByRole('button', { name: 'Leave' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-palette', 'studio')
  const copy = page.getByRole('group', { name: 'Night copy', exact: true })
  await copy.getByRole('button', { name: 'Edit' }).click()
  await expect(brand).toHaveValue('#6E5BFF')

  // Dark mode edits the dark block; leaving puts this device's mode back.
  await page.getByRole('group', { name: 'Edit mode' }).getByRole('button', { name: 'dark' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark')
  await expect(brand).toHaveValue('#8B7BFF')
  await page.getByRole('group', { name: 'Edit mode' }).getByRole('button', { name: 'light' }).click()
  // Undo steps back; the readability check flags text the same colour as the page.
  const fg = page.getByLabel('Foreground', { exact: true })
  const fgWas = await fg.inputValue()
  await fg.fill('#E6F4F4')
  await page.getByLabel('Background', { exact: true }).first().fill('#E6F4F4')
  await expect(page.getByRole('status', { name: 'Readability' })).toContainText('Text on the page')
  await page.getByRole('button', { name: 'Undo' }).click() // quick edits in a row are one step
  await expect(fg).toHaveValue(fgWas)
  await expect(page.getByRole('status', { name: 'Readability' })).not.toContainText('Text on the page')
  await brand.fill('#00aa00')
  await page.getByRole('button', { name: 'Save', exact: true }).click()
  await expect(page.getByText('Saved', { exact: true })).toBeVisible()
  await page.reload()
  await copy.getByRole('button', { name: 'Edit' }).click()
  await expect(brand).toHaveValue('#00aa00')
  await page.getByRole('button', { name: 'Back to palettes' }).click()
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'light')

  await copy.getByRole('button', { name: 'Delete' }).click()
  await page.getByRole('button', { name: 'Delete palette' }).click()
  await expect(copy).toHaveCount(0)
})

// The built-ins (docs/specs/2026-10-27-more-palettes-design.md): one grid, the first
// five then the 17 added, and a new one saves and paints the login page.
test('every built-in palette is in the picker; a new one saves and shows on the login page', async ({ page }) => {
  await systemLogin(page)
  try {
    await page.goto('/admin/system/appearance')
    const builtIns = page.getByRole('group').filter({ hasText: 'Built-in' })
    await expect(builtIns).toHaveCount(22)
    await expect(builtIns.first()).toHaveAttribute('aria-label', 'Studio')
    await expect(builtIns.last()).toHaveAttribute('aria-label', 'Neumorph')

    await page.getByRole('group', { name: 'Kanagawa', exact: true }).getByRole('button', { name: 'Use' }).click()
    await expect(page.getByRole('group', { name: 'Kanagawa', exact: true })).toHaveAttribute('data-active', 'true')
    await page.reload()
    await expect(page.locator('html')).toHaveAttribute('data-palette', 'kanagawa')
    await page.request.post('/api/v1/auth/logout')
    await page.goto('/login')
    await expect(page.locator('html')).toHaveAttribute('data-palette', 'kanagawa')
  } finally {
    await systemLogin(page)
    await page.request.put('/api/v1/system/appearance', { data: { palette: 'studio', receiptStyle: 'classic' } })
  }
})
