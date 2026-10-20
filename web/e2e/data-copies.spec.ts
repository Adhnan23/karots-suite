import { mkdtemp, readdir, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'
import { expect, test, type Page } from './fixtures'

// M8c2 backup copies: the owner's copy folder on the Data page. The server runs
// on this machine, so a temp folder here is a folder it can copy into.
test.describe.configure({ mode: 'serial' })

async function signIn(page: Page) {
  await page.goto('/login')
  await page.getByLabel('Phone').fill('0770000001')
  await page.getByLabel('PIN').fill('1234')
  await page.getByRole('button', { name: /sign in/i }).click()
  await page.waitForURL(/\/(till|admin)/)
}

test('data: copy folder — refuse the backups folder, save, copy now, list it', async ({ page }) => {
  await signIn(page)
  const data = await (await page.request.get('/api/v1/admin/data')).json()
  const backups = resolve('..', data.folder as string) // the server runs from the repo root
  const seed = join(backups, `e2e-copy-${Date.now()}.json.gz`)
  await writeFile(seed, 'e2e copy test') // something to copy
  const dir = await mkdtemp(join(tmpdir(), 'ks-copies-'))
  try {
    await page.goto('/admin/data')
    const card = page.getByRole('region', { name: 'Backup copies' })
    await card.getByRole('button', { name: /Set a folder|Change folder/ }).click()
    const sheet = page.getByRole('dialog')
    const input = sheet.getByLabel('Folder')

    await input.fill(backups)
    await sheet.getByRole('button', { name: 'Save' }).click()
    await expect(sheet.getByRole('alert')).toHaveText(/that is where the backups already are/)

    // Choose…: the server's folders — make one inside the temp folder and use it.
    await input.fill(dir)
    await sheet.getByRole('button', { name: 'Choose…' }).click()
    const picker = page.getByRole('dialog', { name: 'Choose the copy folder' })
    await picker.getByPlaceholder(/New folder/).fill('Karots copies')
    await picker.getByRole('button', { name: 'Make' }).click()
    await expect(picker.getByText(join(dir, 'Karots copies'), { exact: true })).toBeVisible()
    await picker.getByRole('button', { name: 'Use this folder' }).click()
    await expect(input).toHaveValue(join(dir, 'Karots copies'))
    await input.fill(dir)
    await sheet.getByRole('button', { name: 'Save' }).click()
    await expect(sheet).toBeHidden()
    const place = card.getByRole('region', { name: 'Copy folder' })
    await expect(place).toContainText(dir)

    await place.getByRole('button', { name: 'Copy now' }).click()
    await expect(place).toContainText(/Last copied just now/)
    expect((await readdir(dir)).some((f) => f.endsWith('.json.gz'))).toBe(true)

    await place.getByRole('button', { name: 'Backups here…' }).click()
    await expect(page.getByRole('list', { name: 'Copied backups' })).toContainText(seed.split('/').pop()!)
  } finally {
    await page.request.delete('/api/v1/admin/data/copies/folder')
    await rm(seed, { force: true })
    await rm(dir, { recursive: true, force: true })
  }
})
