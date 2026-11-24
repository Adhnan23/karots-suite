import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'

/** The shop's copy of the catalogue (CTL-4: no copy yet → type by hand). */
export type Status = { version: number; count: number; checkedAt: string | null; syncedAt: string | null; error: string; hasAddress: boolean }

export const statusQuery = queryOptions({ queryKey: ['catalogue', 'status'], queryFn: () => api<Status>('/x/catalogue/status') })

/** A search result and what picking it fills (CTL-6); have = the shop product it already is (CTL-5). */
export type Pick = {
  id: number
  name: string
  unit: string
  kinds: string
  fields: Record<string, string> | null
  unitId: number
  sections?: Record<string, unknown>
  have?: { id: number; name: string }
}

export const searchQuery = (q: string) =>
  queryOptions({ queryKey: ['catalogue', 'search', q], queryFn: () => api<Pick[]>(`/x/catalogue/search?q=${encodeURIComponent(q)}`), staleTime: 60_000 })
