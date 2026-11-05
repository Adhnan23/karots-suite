import 'fake-indexeddb/auto'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useReducer } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState, tillReducer, type Action } from '@/features/till/cart/store'
import { slots } from '@/plugins/slots'
import { PhoneTillProvider, phoneTillApi } from '../tillApi'
import { CartLines } from './CartBar'
import { DrawerSheet } from './DrawerSheet'
import { Menu } from './Menu'
import { Sell } from './Sell'

const product = (over: Partial<CopyProduct> = {}) => ({ id: 1, name: 'Panadol', sellingPrice: '100.00', wholesalePrice: '0', stockQty: '5', unitAbbr: 'pc', isService: false, badges: [], barcode: '', barcodes: [], packCodes: [], packs: [], categoryId: 1, ...over }) as unknown as CopyProduct
const copy = { symbol: 'Rs.', categories: [], priceOptions: {}, products: [product(), product({ id: 2, name: 'Photocopy', isService: true, sellingPrice: '0', badges: ['🖨'] })],
  places: [], sessions: [{ userId: 7, expected: '100' }], people: [{ id: 7 }], denominations: [], tenderMethods: [], defaultLockerId: 0, allowUntrackedCash: true } as unknown as TillCopy
const who = { id: 7, name: 'Nimal', role: 'cashier' }

function Shell({ children, lines = false }: { children: (cart: ReturnType<typeof initialState>, dispatch: (a: Action) => void) => React.ReactNode; lines?: boolean }) {
  const [cart, dispatch] = useReducer(tillReducer, undefined, () => {
    const s = initialState('retail')
    return lines ? tillReducer(s, { type: 'add', product: product(), lots: [] }) : s
  })
  return <PhoneTillProvider value={phoneTillApi(cart, dispatch, copy)}>{children(cart, dispatch)}</PhoneTillProvider>
}

function withSlots<K extends keyof typeof slots>(k: K, item: (typeof slots)[K][number]) {
  ;(slots[k] as unknown[]).push(item)
  cleanups.push(() => (slots[k] as unknown[]).pop())
}
const cleanups: (() => void)[] = []

describe('plugin slots on the phone', () => {
  afterEach(() => { cleanup(); while (cleanups.length) cleanups.pop()!() })

  it('Sell shows a plugin\'s till menu, and its panel can add to the cart', async () => {
    withSlots('tillMenu', { key: 't', label: 'Reloads', emoji: '📲', Panel: ({ till, onDone }) => <button onClick={() => { till.addServiceLine({ productId: 2, name: 'Reload', price: '50' } as never); onDone() }}>Add reload</button> })
    render(<Shell>{(cart, dispatch) => <><Sell copy={copy} cart={cart} dispatch={dispatch} live={false} /><output data-testid="n">{cart.lines.length}</output></>}</Shell>)
    fireEvent.click(screen.getByRole('button', { name: /Reloads/ }))
    fireEvent.click(await screen.findByRole('button', { name: 'Add reload' }))
    await waitFor(() => expect(screen.getByTestId('n').textContent).toBe('1'))
    expect(screen.queryByRole('button', { name: 'Add reload' })).toBeNull()
  })

  it('tapping a product a plugin claims opens that plugin\'s panel', async () => {
    withSlots('tillMenu', { key: 't', label: 'Print & Copy', claims: (p) => p.badges.includes('🖨'), Panel: ({ product: p }) => <p>Job for {p?.name}</p> })
    render(<Shell>{(cart, dispatch) => <Sell copy={copy} cart={cart} dispatch={dispatch} live={false} />}</Shell>)
    fireEvent.click(screen.getAllByRole('button', { name: /Photocopy/ })[0])
    expect(await screen.findByText('Job for Photocopy')).toBeTruthy()
  })

  it('a scan a plugin claims opens its panel with the text', async () => {
    withSlots('tillActions', { key: 'r', label: 'Repairs', claimsText: (t) => t.startsWith('JOB'), Panel: ({ text }) => <p>Job {text}</p> })
    render(<Shell>{(cart, dispatch) => <Sell copy={copy} cart={cart} dispatch={dispatch} live={false} />}</Shell>)
    fireEvent.change(screen.getByLabelText('Search or scan'), { target: { value: 'JOB-5' } })
    fireEvent.submit(screen.getByLabelText('Search or scan').closest('form')!)
    expect(await screen.findByText('Job JOB-5')).toBeTruthy()
  })

  it('Menu shows a plugin\'s till actions, but not customer requests', async () => {
    withSlots('tillActions', { key: 'pharmacy', label: '💊 Patient', Panel: () => <p>Patient panel</p> })
    withSlots('tillActions', { key: 'requests', label: 'Note request', Panel: () => <p>no</p> })
    render(<Shell>{() => <Menu who={who} copy={copy} out={[]} live={false} onSent={() => {}} onSwitch={() => {}} onPage={() => {}} />}</Shell>)
    expect(screen.queryByRole('button', { name: 'Note request' })).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: '💊 Patient' }))
    expect(await screen.findByText('Patient panel')).toBeTruthy()
  })

  it('a panel that fails says so, instead of taking the phone down', async () => {
    vi.spyOn(console, 'error').mockImplementation(() => {})
    withSlots('tillActions', { key: 'x', label: 'Broken', Panel: () => { throw new Error('no data') } })
    render(<Shell>{() => <Menu who={who} copy={copy} out={[]} live={false} onSent={() => {}} onSwitch={() => {}} onPage={() => {}} />}</Shell>)
    fireEvent.click(screen.getByRole('button', { name: 'Broken' }))
    expect(await screen.findByText(/needs the PC/)).toBeTruthy()
    vi.restoreAllMocks()
  })

  it('the cart shows plugin hints', async () => {
    withSlots('tillCartHints', { key: 'c', Hint: ({ lines }) => <p>{lines.length} make a deal</p> })
    render(<Shell lines>{(cart, dispatch) => <CartLines cart={cart} dispatch={dispatch} copy={copy} />}</Shell>)
    expect(await screen.findByText('1 make a deal')).toBeTruthy()
  })

  it('a cart line shows plugin hints when opened', async () => {
    withSlots('tillLineHints', { key: 'l', Hint: ({ line }) => <p>Swap {line.name}</p> })
    render(<Shell lines>{(cart, dispatch) => <CartLines cart={cart} dispatch={dispatch} copy={copy} />}</Shell>)
    fireEvent.click(screen.getByRole('button', { name: /Edit Panadol/ }))
    expect(await screen.findByText('Swap Panadol')).toBeTruthy()
  })

  it('closing the drawer runs a plugin\'s count first; false stops the close', async () => {
    const save = vi.fn(async () => false)
    withSlots('drawerSections', { key: 'd', on: ['close'], Section: ({ register }) => { register(save); return <p>Float counts</p> } })
    const onSend = vi.fn(async () => '')
    render(<DrawerSheet kind="close" copy={copy} expected="100" onSend={onSend} onClose={() => {}} />)
    expect(screen.getByText('Float counts')).toBeTruthy()
    fireEvent.click(screen.getAllByRole('button', { name: 'Close' }).find((b) => b.dataset.slot !== 'sheet-close')!)
    await waitFor(() => expect(save).toHaveBeenCalled())
    expect(onSend).not.toHaveBeenCalled()
    save.mockResolvedValue(true)
    fireEvent.click(screen.getAllByRole('button', { name: 'Close' }).find((b) => b.dataset.slot !== 'sheet-close')!)
    await waitFor(() => expect(onSend).toHaveBeenCalled())
  })

  it('opening the drawer doesn\'t show a section meant for closing', () => {
    withSlots('drawerSections', { key: 'd', on: ['close'], Section: () => <p>Float counts</p> })
    render(<DrawerSheet kind="open" copy={copy} expected="0" onSend={async () => ''} onClose={() => {}} />)
    expect(screen.queryByText('Float counts')).toBeNull()
  })

  it('with no plugin compiled in, nothing extra shows', () => {
    render(<Shell>{(cart, dispatch) => <Sell copy={copy} cart={cart} dispatch={dispatch} live={false} />}</Shell>)
    expect(screen.queryByRole('group', { name: 'Plugin menus' })).toBeNull()
  })
})
