import { keepPreviousData, queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import type { MakeProduct, RequestList, RequestRow, RequestTab } from './types'

const base = '/x/requests'

export const listQuery = (tab: RequestTab, q: string, page: number) =>
  queryOptions({
    queryKey: ['x', 'requests', 'list', tab, q, page],
    queryFn: () => api<RequestList>(`${base}?${new URLSearchParams({ tab, q, page: String(page) })}`),
    placeholderData: keepPreviousData,
  })

export const similarQuery = (q: string) =>
  queryOptions({
    queryKey: ['x', 'requests', 'similar', q],
    queryFn: () => api<RequestRow[]>(`${base}/similar?${new URLSearchParams({ q })}`),
    enabled: q.trim().length >= 2,
  })

/** Every write refreshes the lists — also when it fails (the row may have moved on). */
async function write<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } finally {
    void queryClient.invalidateQueries({ queryKey: ['x', 'requests'] })
  }
}

export type Contact = { name: string; phone: string }

// the till
export const addRequest = (note: string, c: Contact) => write(api<RequestRow>(base, { json: { note, ...c } }))
export const askAgain = (id: number, c: Contact) => write(api(`${base}/${id}/asks`, { json: c }))

// admin
export const updateNote = (id: number, note: string) => write(api(`${base}/${id}`, { method: 'PUT', json: { note } }))
export const updateAsk = (id: number, ask: number, c: Contact) => write(api(`${base}/${id}/asks/${ask}`, { method: 'PUT', json: c }))
export const deleteAsk = (id: number, ask: number) => write(api(`${base}/${id}/asks/${ask}`, { method: 'DELETE' }))
export const markCalled = (id: number, ask: number) => write(api(`${base}/${id}/asks/${ask}/called`, { method: 'POST' }))
export const removeRequest = (id: number) => write(api(`${base}/${id}`, { method: 'DELETE' }))
export const makeProduct = (id: number, p: MakeProduct) => write(api<{ productId: number }>(`${base}/${id}/product`, { json: p }))
export const linkProduct = (id: number, productId: number) => write(api(`${base}/${id}/link`, { json: { productId } }))
export const decline = (id: number, reason: string) => write(api(`${base}/${id}/decline`, { json: { reason } }))
export const reopen = (id: number) => write(api(`${base}/${id}/reopen`, { method: 'POST' }))
export const merge = (id: number, into: number) => write(api(`${base}/${id}/merge`, { json: { into } }))
export const clearDone = () => write(api<{ cleared: number }>(`${base}/clear-done`, { method: 'POST' }))
