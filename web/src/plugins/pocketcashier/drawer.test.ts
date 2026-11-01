import { describe, expect, it } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import { drawerOpen, expectedCash, heldList } from './drawer'
import type { Action } from './store/outbox'

const copy = {
  sessions: [{ id: 1, userId: 7, openedAt: '', openingCash: '500.00', expected: '1000.00' }],
  held: [
    { id: 30, userId: 7, label: 'Ravi', saleType: 'retail', customerId: 0, discount: '0', cart: '{}', itemCount: 1, total: '90.00', createdAt: '' },
    { id: 31, userId: 7, label: 'Gone', saleType: 'retail', customerId: 0, discount: '0', cart: '{}', itemCount: 1, total: '10.00', createdAt: '' },
    { id: 32, userId: 8, label: 'Not mine', saleType: 'retail', customerId: 0, discount: '0', cart: '{}', itemCount: 1, total: '10.00', createdAt: '' },
  ],
} as unknown as TillCopy

const act = (kind: Action['kind'], body: unknown, userId = 7): Action => ({ ref: kind + Math.random(), kind, userId, at: '2026-11-01T10:00:00Z', body })

describe('the phone drawer', () => {
  it('adds what has not been sent yet to the copy’s expected cash', () => {
    const out = [
      act('sale', { total: '250', payments: [{ method: 'cash', amount: '500' }] }),
      act('withdraw', { amount: '100' }),
      act('deposit', { amount: '50' }),
      act('deposit', { amount: '999' }, 8), // someone else's
    ]
    expect(expectedCash(copy, 7, out)).toBe('1200.00')
  })

  it('counts kept change as cash that stayed in the drawer', () => {
    // 640 paid with 1000 cash; the customer let the shop keep 5 → 355 handed back.
    const out = [act('sale', { total: '640', changeGiven: '355', payments: [{ method: 'cash', amount: '1000' }] })]
    expect(expectedCash(copy, 7, out)).toBe('1645.00')
  })

  it('counts a card sale as no cash, and starts again from an unsent open', () => {
    const out = [act('open', { openingCash: '300' }), act('sale', { total: '80', payments: [{ method: 'card', amount: '80' }] })]
    expect(expectedCash(copy, 7, out)).toBe('300.00')
    expect(drawerOpen(copy, 7, [act('close', { closingCash: '0' })])).toBe(false)
    expect(drawerOpen(copy, 9, [act('open', { openingCash: '0' }, 9)])).toBe(true)
  })

  it('lists my holds, without ones resumed, plus holds not sent yet', () => {
    const out = [act('unhold', { heldId: 31 }), act('hold', { label: 'Saman', total: '40.00', itemCount: 2, cart: '{}' })]
    expect(heldList(copy, 7, out).map((h) => h.label)).toEqual(['Ravi', 'Saman'])
  })
})
