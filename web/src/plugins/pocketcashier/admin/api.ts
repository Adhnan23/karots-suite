import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import type { Addresses, PhoneRow, SendDetail, SendRow } from './types'

const base = '/x/pocketcashier'

export const when = (iso: string) => new Date(iso).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })

// A waiting phone shows up without a reload: the list is polled while open.
export const phonesQuery = queryOptions({ queryKey: ['x', 'pocketcashier', 'phones'], queryFn: () => api<PhoneRow[]>(`${base}/phones`), refetchInterval: 5_000 })
export const addressesQuery = queryOptions({ queryKey: ['x', 'pocketcashier', 'addresses'], queryFn: () => api<Addresses>(`${base}/addresses`) })
export const sendsQuery = queryOptions({ queryKey: ['x', 'pocketcashier', 'sends'], queryFn: () => api<SendRow[]>(`${base}/sends?limit=100`) })
export const sendQuery = (id: number) => queryOptions({ queryKey: ['x', 'pocketcashier', 'send', id], queryFn: () => api<SendDetail>(`${base}/sends/${id}`) })

async function write(p: Promise<unknown>) {
  try {
    await p
  } finally {
    void queryClient.invalidateQueries({ queryKey: ['x', 'pocketcashier', 'phones'] })
  }
}

export const approveCode = (code: string) => write(api(`${base}/phones/approve`, { json: { code } }))
export const approvePhone = (id: number) => write(api(`${base}/phones/${id}/approve`, { method: 'POST' }))
export const blockPhone = (id: number) => write(api(`${base}/phones/${id}/block`, { method: 'POST' }))
export const releasePhone = (id: number) => write(api(`${base}/phones/${id}/release`, { method: 'POST' }))
export const removePhone = (id: number) => write(api(`${base}/phones/${id}`, { method: 'DELETE' }))
