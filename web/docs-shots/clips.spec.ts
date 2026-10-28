import { execFileSync } from 'node:child_process'
import { mkdirSync, rmSync, writeFileSync } from 'node:fs'
import { test, type Page } from '../e2e/fixtures'
import { cashier, code, desk, only, owner, scan, session, superStore, type Who } from './demo'

// Short screen recordings for the promo videos (videos/), from a running Demo
// Room's Super Store: `make videos-clips`. Each clip starts once its screen has
// loaded (the loading part is cut off) and runs about CLIP seconds.
// Output: videos/public/clips/<name>.webm — the names in videos/src/clips.ts.
test.skip(!code, 'set DOCS_CODE to the Demo Room hourly code')
test.describe.configure({ mode: 'serial' })

const out = '../videos/public/clips'
const CLIP = 7
// A phone-shaped page: the admin's phone layout, 9:16 like the video.
const tall = { width: 540, height: 960 }

type Clip = { name: string; who: Who; on: 'phone' | 'desk'; path: string; act?: (p: Page) => Promise<void> }

const glide = (px: number) => async (p: Page) => {
  for (let i = 0; i < 25; i++) {
    await p.mouse.wheel(0, px / 25)
    await p.waitForTimeout(160)
  }
}
const typeSlow = (field: string, ...words: string[]) => async (p: Page) => {
  for (const w of words) {
    await p.getByLabel(field).fill('')
    await p.getByLabel(field).pressSequentially(w, { delay: 260 })
    await p.waitForLoadState('networkidle')
    await p.waitForTimeout(1800)
  }
}
const sell = scan(['4790001007493', '4790001002030', '4790001011766'], 'Scan a barcode or search products')

const CLIPS: Clip[] = [
  { name: 'dashboard-phone', who: owner, on: 'phone', path: '/admin', act: glide(900) },
  { name: 'sales-phone', who: owner, on: 'phone', path: '/admin/sales', act: glide(700) },
  { name: 'whos-on', who: owner, on: 'phone', path: '/admin/users' },
  { name: 'lowstock', who: cashier, on: 'desk', path: '/till/low-stock', act: glide(500) },
  { name: 'reorder', who: owner, on: 'phone', path: '/admin/reorder', act: glide(700) },
  { name: 'credit-list', who: owner, on: 'phone', path: '/admin/customers', act: glide(600) },
  { name: 'statement', who: owner, on: 'phone', path: 'statement', act: glide(500) },
  { name: 'cashflow-phone', who: owner, on: 'phone', path: '/admin/cashflow', act: glide(700) },
  { name: 'activity', who: owner, on: 'phone', path: '/admin/activity', act: glide(700) },
  { name: 'till-close', who: cashier, on: 'desk', path: '/till', act: closeTill },
  { name: 'supplier-dues', who: owner, on: 'phone', path: '/admin/reports/supplier-dues', act: glide(600) },
  { name: 'profit', who: owner, on: 'phone', path: '/admin/reports/profit-by-category', act: glide(600) },
  { name: 'sell', who: cashier, on: 'desk', path: '/till', act: sell },
  { name: 'backup', who: owner, on: 'phone', path: '/admin/data', act: glide(600) },
  { name: 'requests', who: owner, on: 'phone', path: '/admin/x/requests', act: glide(500) },
  { name: 'similar', who: owner, on: 'phone', path: '/admin/x/alternatives', act: glide(500) },
  { name: 'search-local', who: cashier, on: 'desk', path: '/till', act: typeSlow('Scan a barcode or search products', 'සම්බා', 'அரிசி') },
  { name: 'hold', who: cashier, on: 'desk', path: '/till', act: hold },
  ...['pharmacy', 'repairs', 'recharge', 'documents', 'clearance', 'combos'].map(
    (k): Clip => ({ name: k, who: owner, on: 'phone', path: `/admin/x/${k}`, act: glide(600) }),
  ),
  { name: 'till-lock', who: cashier, on: 'desk', path: 'lock' }, // last: it leaves the cashier locked
]

async function closeTill(p: Page) {
  await p.getByRole('button', { name: /Close till/ }).click()
  const counts = p.getByLabel(/^Count of /)
  for (let i = 0; i < 3; i++) {
    await counts.nth(i).pressSequentially(String(3 - i), { delay: 200 })
    await p.waitForTimeout(300)
  }
  await p.waitForTimeout(1200)
}

async function hold(p: Page) {
  await sell(p)
  await p.keyboard.press('F4')
  await p.waitForTimeout(1500)
}

// Pages that aren't a fixed path: a statement for the customer who owes most,
// and the lock screen (locked by the same call auto-lock makes).
async function resolve(p: Page, path: string): Promise<string> {
  if (path === 'statement') {
    const list = await (await p.request.get('/api/v1/admin/customers?limit=200')).json()
    const top = [...list.items].sort((a, b) => Number(b.owed) - Number(a.owed))[0]
    return `/print/statement/customer/${top.id}`
  }
  if (path === 'lock') {
    await p.request.post('/api/v1/auth/lock')
    return '/lock'
  }
  return path
}

test('Super Store is up', async ({ browser }) => {
  const page = await session(browser, null)
  await superStore(page)
  await page.context().close()
})

// Chrome's own screencast, not Playwright's recordVideo: that one draws a 2×
// phone page at 1× in a corner. (Frames come at CSS pixels, so a phone clip is
// 540×960; the video scales it up.) Frames arrive only when the screen changes, each
// with its time; ffmpeg turns them into a steady 30 fps clip.
async function record(page: Page, name: string, fn: () => Promise<void>) {
  const dir = `test-results/clips/${name}`
  rmSync(dir, { recursive: true, force: true })
  mkdirSync(dir, { recursive: true })
  const cdp = await page.context().newCDPSession(page)
  const frames: { file: string; t: number }[] = []
  cdp.on('Page.screencastFrame', (f) => {
    const file = `${dir}/${String(frames.length).padStart(5, '0')}.jpg`
    writeFileSync(file, Buffer.from(f.data, 'base64'))
    frames.push({ file, t: f.metadata.timestamp ?? Date.now() / 1000 })
    cdp.send('Page.screencastFrameAck', { sessionId: f.sessionId }).catch(() => {})
  })
  const vp = page.viewportSize()!
  await cdp.send('Page.startScreencast', { format: 'jpeg', quality: 92, maxWidth: vp.width, maxHeight: vp.height })
  await page.waitForTimeout(300)
  const start = Date.now() / 1000
  await fn()
  const end = Date.now() / 1000
  await cdp.send('Page.stopScreencast')
  if (!frames.length) throw new Error(`${name}: no frames`)
  const list = frames.map((f, i) => {
    const next = i + 1 < frames.length ? frames[i + 1].t : end
    return `file '${f.file.split('/').pop()}'\nduration ${Math.max(0.001, next - Math.max(f.t, i ? 0 : start)).toFixed(3)}`
  })
  writeFileSync(`${dir}/list.txt`, list.join('\n') + `\nfile '${frames.at(-1)!.file.split('/').pop()}'\n`)
  execFileSync('ffmpeg', ['-y', '-v', 'error', '-f', 'concat', '-safe', '0', '-i', `${dir}/list.txt`, '-vf', 'fps=30,format=yuv420p',
    '-c:v', 'libvpx-vp9', '-b:v', '0', '-crf', '28', '-an', `${out}/${name}.webm`])
}
for (const c of CLIPS) {
  test(`clip ${c.name}`, async ({ browser }) => {
    test.skip(!!only && !c.name.includes(only))
    mkdirSync(out, { recursive: true })
    const page = await session(browser, c.who, c.on === 'phone' ? tall : desk)
    await page.goto(await resolve(page, c.path))
    await page.waitForLoadState('networkidle')
    await page.waitForTimeout(600) // charts and Motion settle
    await record(page, c.name, async () => {
      const acted = Date.now()
      await c.act?.(page)
      await page.waitForTimeout(Math.max(500, CLIP * 1000 - (Date.now() - acted)))
    })
    await page.context().close()
  })
}
