import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import { type Range, rangeQS } from '@/features/counter/range'
import type { Combo, ComboRequest, Report, Settings } from './types'

const base = '/x/combos'

export const combosQuery = queryOptions({ queryKey: ['x', 'combos', 'all'], queryFn: () => api<Combo[]>(base) })
/** What the till can ring today (cached briefly: hints read it on every cart change). */
export const tillCombosQuery = queryOptions({ queryKey: ['x', 'combos', 'till'], queryFn: () => api<Combo[]>(`${base}/till`), staleTime: 60_000 })
export const settingsQuery = queryOptions({ queryKey: ['x', 'combos', 'settings'], queryFn: () => api<Settings>(`${base}/settings`), staleTime: 60_000 })
export const reportQuery = (range: Range) =>
  queryOptions({ queryKey: ['x', 'combos', 'report', range], queryFn: () => api<Report>(`${base}/report?${rangeQS(range).slice(1)}`) })

async function write<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } finally {
    void queryClient.invalidateQueries({ queryKey: ['x', 'combos'] })
  }
}

export const saveCombo = (id: number, body: ComboRequest) =>
  write(api<Combo>(id ? `${base}/${id}` : base, { method: id ? 'PUT' : 'POST', json: body }))
export const setActive = (id: number, active: boolean) => write(api(`${base}/${id}/active`, { json: { active } }))
export const deleteCombo = (id: number) => write(api(`${base}/${id}`, { method: 'DELETE' }))
export const recordSold = (saleId: number, combos: { comboId: number; sets: string; net: string; discount: string }[]) =>
  api(`${base}/sold`, { json: { saleId, combos } })
export const saveSettings = (v: Settings) => write(api<Settings>(`${base}/settings`, { method: 'PUT', json: v }))
