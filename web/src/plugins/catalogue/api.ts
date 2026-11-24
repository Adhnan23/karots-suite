import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'

/** The shop's copy of the catalogue (CTL-4: no copy yet → type by hand). */
export type Status = { version: number; count: number; checkedAt: string | null; syncedAt: string | null; error: string; hasAddress: boolean }

export const statusQuery = queryOptions({ queryKey: ['catalogue', 'status'], queryFn: () => api<Status>('/x/catalogue/status') })
