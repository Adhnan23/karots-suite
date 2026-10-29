import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { MovementList, Option, ProductLots } from '@/api/types.gen'
import { type Range, rangeQS } from '@/features/counter/range'

export type MoveFilter = { product: number; type: string; user: number; range: Range; page: number }

export const movementsQuery = (f: MoveFilter) =>
  queryOptions({
    queryKey: ['admin', 'movements', f],
    queryFn: () => api<MovementList>(`/admin/stock/movements?product=${f.product}&type=${f.type}&user=${f.user}&page=${f.page}${rangeQS(f.range)}`),
  })
export const lotsQuery = (id: number) => queryOptions({ queryKey: ['admin', 'lots', id], queryFn: () => api<ProductLots>(`/admin/stock/lots/${id}`) })
export const staffQuery = queryOptions({ queryKey: ['admin', 'staff'], queryFn: () => api<Option[]>('/admin/staff-options'), staleTime: 60_000 })

/** Movement types as the shop says them. */
export const MOVE_TYPES: [string, string][] = [
  ['sale', 'Sale'], ['purchase', 'Delivery'], ['adjust', 'Adjustment'], ['return', 'Customer return'], ['damage', 'Damage / expired'],
  ['conversion', 'Conversion'], ['purchase_return', 'Returned to supplier'], ['warranty_replacement', 'Warranty swap'],
  ['recovery', 'Recovered'], ['own_use', 'Shop use'], ['staff', 'Staff'],
]
export const moveLabel = (t: string) => MOVE_TYPES.find(([v]) => v === t)?.[1] ?? t.replace(/_/g, ' ')
