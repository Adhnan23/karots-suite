import { describe, expect, it } from 'vitest'
import type { SaleLine } from '@/api/types.gen'
import { bumpQty } from './refund'

const line = (id: number, quantity: string, group = 0, sets = '', returnedQty = '0'): SaleLine => ({
  id, name: `L${id}`, unitAbbr: 'pc', quantity, returnedQty, unitPrice: '10.00', subtotal: '100.00', refundUnit: '10.00',
  paid: '100.00', refundLeft: '100.00', isService: false, group, groupLabel: group ? 'Combo' : '', groupSets: sets, packLabel: '' })

// RET-13: a combo comes back whole, in whole sets — its lines move together.
describe('bumpQty', () => {
  const lines = [line(1, '2', 1, '2'), line(2, '4', 1, '2'), line(3, '5')]

  it('moves a plain line one unit, within what is left', () => {
    expect(bumpQty(lines, {}, lines[2], 1)).toEqual({ 3: 1 })
    expect(bumpQty(lines, { 3: 5 }, lines[2], 1)).toEqual({ 3: 5 })
    expect(bumpQty(lines, { 3: 0 }, lines[2], -1)).toEqual({ 3: 0 })
  })

  it('moves every line of a combo one whole set', () => {
    expect(bumpQty(lines, {}, lines[0], 1)).toEqual({ 1: 1, 2: 2 })
    expect(bumpQty(lines, { 1: 1, 2: 2 }, lines[1], 1)).toEqual({ 1: 2, 2: 4 })
    expect(bumpQty(lines, { 1: 2, 2: 4 }, lines[0], 1)).toEqual({ 1: 2, 2: 4 })
    expect(bumpQty(lines, { 1: 2, 2: 4 }, lines[1], -1)).toEqual({ 1: 1, 2: 2 })
  })

  it('stops at the sets still out', () => {
    const half = [line(1, '2', 1, '2', '1'), line(2, '4', 1, '2', '2')]
    expect(bumpQty(half, { 1: 1, 2: 2 }, half[0], 1)).toEqual({ 1: 1, 2: 2 })
  })
})
