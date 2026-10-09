import { queryOptions, useQueryClient } from '@tanstack/react-query'
import { api, ApiError } from '@/api/client'
import type {
  DrawerState, GroupView, HeldSale, Lot, ProductPage, TillConfig, TillCustomer, TillGroup, TillProduct, Unit,
} from '@/api/types.gen'

export const configQuery = queryOptions({ queryKey: ['till', 'config'], queryFn: () => api<TillConfig>('/till/config'), staleTime: Infinity })
export const drawerQuery = queryOptions({ queryKey: ['till', 'drawer'], queryFn: () => api<DrawerState>('/till/drawer') })
export const groupsQuery = queryOptions({ queryKey: ['till', 'groups'], queryFn: () => api<TillGroup[]>('/till/groups'), staleTime: 60_000 })
export const groupQuery = (id: number) =>
  queryOptions({ queryKey: ['till', 'group', id], queryFn: () => api<GroupView>(`/till/groups/${id}`) })
export const searchQuery = (q: string) =>
  queryOptions({
    queryKey: ['till', 'search', q],
    queryFn: ({ signal }) => api<ProductPage>(`/till/products?search=${encodeURIComponent(q)}`, { signal }),
    enabled: q.trim() !== '',
  })
/** Which products ask "which price?". Best-effort, refreshed every 90s and on focus
 * (another lane receiving stock changes the answer mid-shift). */
export const priceOptionsQuery = queryOptions({
  queryKey: ['till', 'price-options'],
  queryFn: () => api<Record<string, Lot[]>>('/till/price-options'),
  refetchInterval: 90_000, refetchOnWindowFocus: true, retry: false,
})
export const quickPicksQuery = queryOptions({ queryKey: ['till', 'quick-picks'], queryFn: () => api<TillProduct[]>('/till/quick-picks') })
export const unitsQuery = queryOptions({ queryKey: ['till', 'units'], queryFn: () => api<Unit[]>('/till/units'), staleTime: Infinity })
export const customersQuery = queryOptions({ queryKey: ['till', 'customers'], queryFn: () => api<TillCustomer[]>('/till/customers') })
export const heldQuery = queryOptions({ queryKey: ['till', 'held'], queryFn: () => api<HeldSale[]>('/till/held') })

/** Card tenders land in 'card' lockers, online ones in 'bank' lockers. */
export const lockersOf = (d: DrawerState | undefined, kind: 'card' | 'bank') => (d?.lockers ?? []).filter((l) => l.kind === kind)
/** Lockers that hold notes and coins — the only places till cash may go to or come from. */
export const holdsCash = (l: { kind: string }) => l.kind !== 'bank' && l.kind !== 'card'

/** Everything the till caches except its config: refetch after a sale or stock change. */
export function useRefreshTill() {
  const qc = useQueryClient()
  return () => qc.invalidateQueries({ predicate: (q) => q.queryKey[0] === 'till' && q.queryKey[1] !== 'config' })
}

export const errMsg = (e: unknown, fallback = 'Something went wrong') => (e instanceof ApiError ? e.message : fallback)
