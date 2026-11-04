import { expect, type Page } from './fixtures'

/** A receipt on screen is a picture, so its words are read from the same slip's
 * lines. Call before the slip shows; the result gives the text of the latest one. */
export function slipText(page: Page) {
  let last = ''
  page.on('request', (r) => {
    if (r.method() === 'POST' && r.url().includes('data=1')) last = r.url()
  })
  return async () => {
    await expect.poll(() => last, { message: 'no slip was drawn' }).not.toBe('')
    const u = new URL(last)
    u.searchParams.delete('data')
    u.searchParams.set('preview', '1')
    const res = await page.request.post(u.pathname + u.search)
    const { lines } = (await res.json()) as { lines: { text: string }[] }
    return lines.map((l) => l.text).join('\n')
  }
}

export const receiptPicture = (scope: Page | ReturnType<Page['locator']>) => scope.getByRole('img', { name: 'Receipt preview' })
