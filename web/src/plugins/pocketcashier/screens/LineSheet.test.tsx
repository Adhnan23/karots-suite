import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { useReducer } from 'react'
import { afterEach, describe, expect, it } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState, tillReducer } from '@/features/till/cart/store'
import { lineNet } from '@/features/till/cart/totals'
import type { TillState } from '@/features/till/cart/types'
import { LineSheet } from './LineSheet'

const product = (over: Partial<CopyProduct>) => ({
  id: 1, name: 'Rice', nameLocal: '', barcode: '', sellingPrice: '100.00', wholesalePrice: '0.00', taxRate: '0', stockQty: '50',
  unitAbbr: 'kg', unitAllowDecimal: true, isService: false, trackSerial: false, warrantyMonths: 0, lowStock: false, emoji: '',
  badges: [], packs: [], categoryId: 1, barcodes: [], packCodes: [], ...over,
}) as unknown as CopyProduct

const copy = { symbol: 'Rs.', priceOptions: {}, products: [], customers: [] } as unknown as TillCopy

/** The real reducer behind the sheet; `seen` gets every state. */
function Harness({ p, seen }: { p: CopyProduct; seen: (s: TillState) => void }) {
  const [cart, dispatch] = useReducer(tillReducer, tillReducer(initialState('retail'), { type: 'add', product: p, lots: [] }))
  seen(cart)
  const line = cart.lines[0]
  return line ? <LineSheet line={line} cart={cart} dispatch={dispatch} copy={copy} onClose={() => undefined} /> : null
}

function open(p: CopyProduct) {
  let state = initialState('retail')
  render(<Harness p={p} seen={(s) => { state = s }} />)
  return () => state
}

const keys = (s: string) => {
  fireEvent.click(screen.getByRole('button', { name: 'Clear' }))
  for (const k of s) fireEvent.click(screen.getByRole('button', { name: k }))
  fireEvent.click(screen.getByRole('button', { name: 'Set quantity' }))
}

describe('the line sheet', () => {
  afterEach(cleanup)

  it('types a weight on a product sold by the kg', () => {
    const state = open(product({}))
    keys('1.25')
    expect(state().lines[0].qty).toBe(1.25)
  })

  it('rounds a typed 1.5 to whole on a whole-only product, as the till does', () => {
    const state = open(product({ unitAllowDecimal: false, unitAbbr: 'pcs' }))
    keys('1.5')
    expect(state().lines[0].qty).toBe(2)
  })

  it('takes 10% off the line', () => {
    const state = open(product({}))
    fireEvent.change(screen.getByLabelText('Discount'), { target: { value: '10' } })
    fireEvent.click(screen.getByRole('button', { name: '%' }))
    expect(lineNet(state().lines[0])).toBeCloseTo(90)
  })

  it('asks one serial per unit on a serial-tracked product', () => {
    const state = open(product({ trackSerial: true, unitAllowDecimal: false }))
    keys('2')
    expect(screen.getAllByLabelText(/^Serial \d/)).toHaveLength(2)
    fireEvent.change(screen.getByLabelText('Serial 2'), { target: { value: 'SN-2' } })
    expect(state().lines[0].serials).toEqual(['', 'SN-2'])
  })
})
