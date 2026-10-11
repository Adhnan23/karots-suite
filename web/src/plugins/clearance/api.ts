import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import type { ClearancePageData, ClearanceSettings } from './types'

const base = '/x/clearance'

export const clearanceQuery = queryOptions({ queryKey: ['x', 'clearance'], queryFn: () => api<ClearancePageData>(base) })

/** Every write refreshes the page and the till's catalogue (badges, prompts) —
 * also when it fails, since "Disable all" may have disabled some before an error. */
async function write<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } finally {
    await queryClient.invalidateQueries({ queryKey: ['x', 'clearance'] })
    void queryClient.invalidateQueries({ queryKey: ['till'] })
  }
}

export const approve = (id: number, percent: string) => write(api(`${base}/${id}/approve`, { json: { percent } }))
export const snooze = (id: number) => write(api(`${base}/${id}/snooze`, { method: 'POST' }))
export const unsnooze = (id: number) => write(api(`${base}/${id}/unsnooze`, { method: 'POST' }))
export const disable = (productIds: number[]) => write(api<{ disabled: number }>(`${base}/disable`, { json: { productIds } }))
export const saveRules = (s: ClearanceSettings) => write(api<ClearanceSettings>(`${base}/settings`, { method: 'PUT', json: s }))
