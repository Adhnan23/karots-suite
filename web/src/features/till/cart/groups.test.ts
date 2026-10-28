import { describe, expect, it } from 'vitest'
import type { TillProduct } from '@/api/types.gen'
import { fromHeld, toHeldBlob } from './held'
import { grouped } from './groups'
import { initialState, tillReducer } from './store'
import type { TillState } from './types'

const prod = (id: number, name: string, price: string, trackSerial = false): TillProduct => ({ id, name, barcode: String(id), sellingPrice: price,
  wholesalePrice: '0.00', taxRate: '0', stockQty: '9', unitAbbr: 'pcs', unitAllowDecimal: false, isService: false, trackSerial, warrantyMonths: 0, lowStock: false, emoji: '', badges: [], packs: [] })
const milk = prod(1, 'Milk', '380.00')
const cookies = prod(2, 'Cookies', '220.00', true)
const pair = { label: 'Milk + Cookies', count: 1, items: [{ product: milk, per: 1, discount: '25.34' }, { product: cookies, per: 1, discount: '14.66' }], plugin: { combos: { comboId: 7 } } }

const withPair = (s: TillState = initialState('retail')) => tillReducer(s, { type: 'addGroup', group: pair })
const gid = (s: TillState) => s.lines.find((l) => l.group)!.group!.id

describe('cart line groups', () => {
  it('addGroup adds one line per member with a fixed per-unit discount and the group', () => {
    const s = withPair()
    expect(s.lines).toHaveLength(2)
    expect(s.lines[0]).toMatchObject({ productId: 1, qty: 1, discount: '25.34', discountType: 'fixed', group: { label: 'Milk + Cookies', per: 1, count: 1 }, plugin: { combos: { comboId: 7 } } })
    expect(s.lines[1].serials).toEqual(['']) // serial boxes follow qty as on a normal add
    expect(new Set(s.lines.map((l) => l.key)).size).toBe(2)
  })
  it('the same group again adds sets to the one in the cart', () => {
    let s = withPair()
    s = tillReducer(s, { type: 'addGroup', group: { ...pair, count: 2 } })
    expect(s.lines).toHaveLength(2)
    expect(s.lines.map((l) => l.qty)).toEqual([3, 3])
    expect(s.lines[0].group!.count).toBe(3)
  })
  it('take moves units off plain lines in the same step', () => {
    let s = initialState('retail')
    s = tillReducer(s, { type: 'quickSet', product: milk, qty: 3 })
    s = tillReducer(s, { type: 'quickSet', product: cookies, qty: 1 })
    const [m, c] = s.lines
    s = tillReducer(s, { type: 'addGroup', group: { ...pair, take: [{ key: m.key, qty: 1 }, { key: c.key, qty: 1 }] } })
    expect(s.lines.filter((l) => !l.group).map((l) => [l.productId, l.qty])).toEqual([[1, 2]])
    expect(s.lines.filter((l) => l.group)).toHaveLength(2)
  })
  it('groupCount scales every line; 0 removes the group', () => {
    let s = withPair()
    s = tillReducer(s, { type: 'groupCount', id: gid(s), count: 3 })
    expect(s.lines.map((l) => l.qty)).toEqual([3, 3])
    expect(s.lines[1].serials).toHaveLength(3)
    expect(s.lines[0].group!.count).toBe(3)
    s = tillReducer(s, { type: 'groupCount', id: gid(s), count: 0 })
    expect(s.lines).toEqual([])
  })
  it('breakGroup removes that line and returns the rest to full price', () => {
    let s = withPair()
    s = tillReducer(s, { type: 'breakGroup', id: gid(s), key: s.lines[0].key })
    expect(s.lines).toHaveLength(1)
    expect(s.lines[0]).toMatchObject({ productId: 2, discount: '' })
    expect(s.lines[0].group).toBeUndefined()
    expect(s.lines[0].plugin).toBeUndefined()
  })
  it('removeGroup drops every member and nothing else', () => {
    let s = tillReducer(initialState('retail'), { type: 'add', product: milk, lots: [] })
    s = withPair(s)
    s = tillReducer(s, { type: 'removeGroup', id: gid(s) })
    expect(s.lines).toHaveLength(1)
    expect(s.lines[0].group).toBeUndefined()
  })
  it('scanning a member never merges into the group', () => {
    let s = withPair()
    s = tillReducer(s, { type: 'add', product: milk, lots: [] })
    s = tillReducer(s, { type: 'quickSet', product: milk, qty: 2 })
    const plain = s.lines.filter((l) => !l.group)
    expect(plain).toHaveLength(1)
    expect(plain[0].qty).toBe(2)
    expect(s.lines.find((l) => l.group && l.productId === 1)!.qty).toBe(1)
  })
  it('hold / resume keeps the group', () => {
    const s = withPair()
    const back = fromHeld({ cart: toHeldBlob(s) } as never, 100)
    expect(back.lines.map((l) => l.group?.label)).toEqual(['Milk + Cookies', 'Milk + Cookies'])
  })
  it('grouped gathers a group under its first line', () => {
    let s = withPair()
    s = tillReducer(s, { type: 'add', product: prod(3, 'Bread', '180.00'), lots: [] })
    expect(grouped(s.lines).map((x) => x.kind)).toEqual(['group', 'line'])
  })
})

describe('service line groups (a job and its add-ons)', () => {
  const job = { productId: 70, name: 'Photocopy A4', price: 40, qty: 4, unitPrice: 10, description: 'Photocopy A4', fixedQty: true, plugin: { x: 1 } }
  const urgent = { productId: 70, name: 'Urgent', price: 100, qty: 1, unitPrice: 100, description: 'Urgent', components: [{ productId: 9, quantity: '1' }] }
  const add = () => tillReducer(initialState('retail'), { type: 'addServiceGroup', group: { label: 'Photocopy', lines: [job, urgent] } })

  it('adds each line as priced, together, one fixed set', () => {
    const s = add()
    expect(s.lines.map((l) => [l.name, l.qty, l.unitPrice, l.group?.label, l.group?.count, l.group?.fixed])).toEqual([
      ['Photocopy A4', 4, 10, 'Photocopy', 1, true], ['Urgent', 1, 100, 'Photocopy', 1, true]])
    expect(s.lines[0].plugin).toEqual({ x: 1 })
    expect(s.lines[1].components).toEqual(urgent.components)
    expect(grouped(s.lines)).toHaveLength(1)
  })
  it('has no − / + and is never broken, only removed whole', () => {
    const s = add()
    const id = gid(s)
    expect(tillReducer(s, { type: 'groupCount', id, count: 3 }).lines).toEqual(s.lines)
    expect(tillReducer(s, { type: 'breakGroup', id, key: s.lines[1].key }).lines).toEqual(s.lines)
    expect(tillReducer(s, { type: 'removeGroup', id }).lines).toEqual([])
  })
  it('survives hold and resume', () => {
    const back = fromHeld({ cart: toHeldBlob(add()) } as never, 0)
    expect(back.lines.every((l) => l.group?.fixed && l.group.label === 'Photocopy')).toBe(true)
  })
})
