import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, openCheats, pollPrints } from './overlay'
import css from './overlay.css?raw'

const state = {
  installId: 'DEMO1234', phase: 'ready', progress: 100, step: '', paused: false, shops: [],
  owner: { name: 'Karots', phone: '0771234567' },
  shop: { key: 'grocery', name: 'Sunrise Mini Mart', blurb: '', plugins: [], palette: 'studio', receiptStyle: 'classic', catalogue: 'grocery',
    logins: [{ role: 'admin', name: 'Nimal Perera', phone: '0771000001', pin: '1111' }] },
  cheats: [{ name: 'busy', label: 'Make it busy (20 sales now)', group: 'Till' }],
}
let prints: unknown = { receipt: [], label: [] }

beforeEach(() => {
  document.body.innerHTML = ''
  vi.stubGlobal('fetch', vi.fn(async (url: string) => {
    const body = url.startsWith('/__demo/state') ? state : url.startsWith('/__demo/prints') ? prints : {}
    return new Response(JSON.stringify(body), { status: 200 })
  }))
})
afterEach(() => vi.unstubAllGlobals())

describe('the demo chrome', () => {
  it('puts Demo moves, Prints and Exit beside Pause and Switch shop', async () => {
    await mount()
    const fly = [...document.querySelectorAll('.ksd-fly button')].map((b) => b.textContent)
    expect(fly).toEqual(['Demo moves & logins', 'Prints', 'Pause', 'Switch shop', 'Exit demo'])
  })

  it('lists every print so far, newest first', async () => {
    prints = { receipt: [{ id: 1, kind: 'receipt', text: ['first'], kick: false, copies: 1, at: '2026-10-28T10:00:00Z' },
      { id: 2, kind: 'receipt', text: ['second'], kick: false, copies: 1, at: '2026-10-28T10:05:00Z' }], label: [] }
    await mount()
    const { openPrints } = await import('./overlay')
    await openPrints()
    const slips = [...document.querySelectorAll('.ksd-shelf pre')].map((p) => p.textContent)
    expect(slips).toEqual(['second', 'first'])
    prints = { receipt: [], label: [] }
  })

  it('shows the owner on the watermark', async () => {
    await mount()
    expect(document.querySelector('.ksd-mark')?.textContent).toContain('0771234567')
  })

  it('opens the cheat menu on Ctrl+Shift+D and on five taps of the watermark', async () => {
    await mount()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'D', ctrlKey: true, shiftKey: true }))
    expect(document.querySelector('.ksd-cheats')?.textContent).toContain('Make it busy')
    document.querySelector('.ksd-cheats')!.remove()
    const mark = document.querySelector<HTMLButtonElement>('.ksd-mark')!
    for (let i = 0; i < 5; i++) mark.click()
    expect(document.querySelector('.ksd-cheats')).not.toBeNull()
    expect(document.querySelector('.ksd-cheats')?.textContent).toContain('Sign in as Nimal Perera')
  })

  it('slides a new print onto the strip once, and not what printed before the page loaded', async () => {
    prints = { receipt: [{ id: 2, kind: 'receipt', kick: false, copies: 1, at: '' }], label: [] }
    await mount()
    await pollPrints()
    expect(document.querySelectorAll('.ksd-paper')).toHaveLength(0)
    prints = { receipt: [{ id: 3, kind: 'receipt', kick: true, copies: 1, at: '' }], label: [] }
    await pollPrints()
    await pollPrints()
    const slips = document.querySelectorAll('.ksd-paper')
    expect(slips).toHaveLength(1)
    expect(slips[0].querySelector('img')?.getAttribute('src')).toBe('/__demo/print/receipt/3')
    expect(slips[0].textContent).toContain('Cash drawer opened')
    openCheats()
  })

  it('marks printed pages DEMO', () => {
    expect(css).toMatch(/@media print[\s\S]*content: "DEMO"/)
  })
})
