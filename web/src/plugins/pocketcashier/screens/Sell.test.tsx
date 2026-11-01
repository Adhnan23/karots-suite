import { cleanup, fireEvent, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import { initialState } from '@/features/till/cart/store'
import { Sell } from './Sell'

const soap = {
  id: 1, name: 'Soap', nameLocal: '', barcode: '4790000000028', sellingPrice: '100.00', wholesalePrice: '0.00', taxRate: '0', stockQty: '5',
  unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial: false, warrantyMonths: 0, lowStock: false, emoji: '',
  badges: [], packs: [], categoryId: 1, barcodes: ['5000000000011'], packCodes: [],
} as unknown as CopyProduct
const copy = { symbol: 'Rs.', priceOptions: {}, products: [soap], categories: [], customers: [] } as unknown as TillCopy

function scan(code: string) {
  const seen: unknown[] = []
  render(<Sell copy={copy} cart={initialState('retail')} dispatch={(a) => seen.push(a)} live />)
  fireEvent.change(screen.getByLabelText('Search or scan'), { target: { value: code } })
  fireEvent.submit(screen.getByLabelText('Search or scan').closest('form')!)
  return seen
}

describe('scanning on the phone', () => {
  afterEach(cleanup)

  it('asks: an existing item with a different barcode, or a new item (Quick item, code filled in)', () => {
    scan('8901234567890')
    const ask = screen.getByRole('dialog', { name: 'No product has this barcode' })
    expect(within(ask).getByRole('button', { name: /Existing item/ })).toBeTruthy()
    fireEvent.click(within(ask).getByRole('button', { name: /New item/ }))
    expect((screen.getByLabelText('Barcode') as HTMLInputElement).value).toBe('8901234567890')
  })

  it('a name nothing matches goes straight to Quick item, as the PC does', () => {
    scan('blue widget')
    expect((screen.getByLabelText('Item name') as HTMLInputElement).value).toBe('blue widget')
  })

  it('finds the existing item to give the code to', () => {
    scan('8901234567890')
    fireEvent.click(screen.getByRole('button', { name: /Existing item/ }))
    fireEvent.change(screen.getByLabelText('Find the item'), { target: { value: 'soa' } })
    expect(screen.getByRole('button', { name: /^Soap/ })).toBeTruthy()
  })

  it("adds the product by one of its extra barcodes", () => {
    const seen = scan('5000000000011')
    expect(seen).toMatchObject([{ type: 'add', product: { id: 1 } }])
    expect(screen.queryByRole('dialog', { name: 'Quick item' })).toBeNull()
  })
})
