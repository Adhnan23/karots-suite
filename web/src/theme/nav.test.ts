import { describe, expect, it } from 'vitest'
import type { Me } from '@/api/types.gen'
import { navFor, whereIs, type NavArea } from './nav'

const areas: NavArea[] = [
  { key: 'stock', label: 'Inventory', icon: (() => null) as never, pages: [
    { to: '/admin/stock', label: 'Stock', description: '' },
    { to: '/admin/stock/take', label: 'Stock-take', description: '', gate: 'stockTake' },
  ] },
]

describe('whereIs', () => {
  it('prefers the longest matching page', () => {
    expect(whereIs(areas, '/admin/stock/take')?.page.label).toBe('Stock-take')
    expect(whereIs(areas, '/admin/stock/lots/4')?.page.label).toBe('Stock')
    expect(whereIs(areas, '/admin/stocks')).toBeUndefined()
  })
})

describe('navFor', () => {
  it('hides stock-take pages when the toggle is off', () => {
    const me = { stockTakeEnabled: false, user: { id: 1, name: 'A', role: 'admin' } } as Me
    expect(navFor(me).flatMap((a) => a.pages).some((p) => p.gate === 'stockTake')).toBe(false)
  })
  it('shows Users to admins only', () => {
    const as = (role: string) => navFor({ stockTakeEnabled: true, user: { id: 1, name: 'A', role } } as Me).flatMap((a) => a.pages).some((p) => p.to === '/admin/users')
    expect(as('admin')).toBe(true)
    expect(as('manager')).toBe(false)
  })
})
