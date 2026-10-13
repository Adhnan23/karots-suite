import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AskReply, CleanupPlan, CleanupPreview, EmptyCategory, IdentifyResult, Run, Settings } from './types'

const base = '/x/aicatalog'

export const settingsQuery = queryOptions({ queryKey: ['x', 'aicatalog', 'settings'], queryFn: () => api<Settings>(`${base}/settings`) })
export const lastRunQuery = queryOptions({ queryKey: ['x', 'aicatalog', 'last'], queryFn: () => api<{ run: Run | null }>(`${base}/cleanup/last`) })
export const emptyQuery = queryOptions({ queryKey: ['x', 'aicatalog', 'empty'], queryFn: () => api<{ categories: EmptyCategory[] }>(`${base}/cleanup/empty-categories`) })

export type IdentifyAsk = { query: string; barcode: string; hint: string; short: boolean; reply?: string }
export const identify = (b: IdentifyAsk) => api<AskReply<IdentifyResult>>(`${base}/identify`, { json: b })
export const findCategory = (path: string) => api<{ id: number }>(`${base}/category`, { json: { path } })
export const saveSettings = (s: Settings) => api<Settings>(`${base}/settings`, { method: 'PUT', json: s })
export const loadModels = (s: Settings) => api<{ models: string[] }>(`${base}/settings/models`, { json: s })
export const testAI = (s: Settings) => api(`${base}/settings/test`, { json: s })
export const cleanup = (offset: number, reply?: string) => api<AskReply<CleanupPreview>>(`${base}/cleanup`, { json: { offset, reply } })
export const applyCleanup = (plan: CleanupPlan, runId: number) => api<{ summary: string; runId: number }>(`${base}/cleanup/apply`, { json: { plan, runId } })
export const undoCleanup = () => api<{ summary: string }>(`${base}/cleanup/undo`, { method: 'POST' })
export const deleteEmpty = (ids: number[]) => api<{ deleted: number }>(`${base}/cleanup/empty-categories`, { json: { ids } })
