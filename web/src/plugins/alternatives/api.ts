import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import type { Alt, Group, Membership } from './types'

const base = '/x/alternatives'

export const groupsQuery = (disabled = false) =>
  queryOptions({ queryKey: ['x', 'alternatives', 'groups', disabled], queryFn: () => api<Group[]>(`${base}${disabled ? '?disabled=1' : ''}`) })
export const groupQuery = (id: number) => queryOptions({ queryKey: ['x', 'alternatives', 'group', id], queryFn: () => api<Group>(`${base}/${id}`) })
/** Every grouped product → where it sits (the picker's "in …" hints). */
export const membershipQuery = queryOptions({ queryKey: ['x', 'alternatives', 'membership'], queryFn: () => api<Record<string, Membership>>(`${base}/membership`) })
export const productTierQuery = (pid: number) =>
  queryOptions({ queryKey: ['x', 'alternatives', 'product', pid], queryFn: () => api<Membership>(`${base}/product/${pid}`), enabled: pid > 0 })
/** In-stock alternatives for a short cart line (cached briefly). */
export const tillAltsQuery = (pid: number) =>
  queryOptions({ queryKey: ['x', 'alternatives', 'till', pid], queryFn: () => api<Alt[]>(`${base}/till/${pid}`), staleTime: 30_000 })

async function write<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } finally {
    void queryClient.invalidateQueries({ queryKey: ['x', 'alternatives'] })
  }
}

export const createGroup = (name: string) => write(api<Group>(base, { json: { name } }))
export const renameGroup = (id: number, name: string) => write(api<Group>(`${base}/${id}`, { method: 'PUT', json: { name } }))
export const setGroupActive = (id: number, active: boolean) => write(api(`${base}/${id}/active`, { json: { active } }))
export const deleteGroup = (id: number) => write(api(`${base}/${id}`, { method: 'DELETE' }))
export const createTier = (gid: number, name: string, reorderLevel: number) => write(api<Group>(`${base}/${gid}/tiers`, { json: { name, reorderLevel } }))
export const updateTier = (tid: number, name: string, reorderLevel: number) => write(api<Group>(`${base}/tiers/${tid}`, { method: 'PUT', json: { name, reorderLevel } }))
export const setTierActive = (tid: number, active: boolean) => write(api<Group>(`${base}/tiers/${tid}/active`, { json: { active } }))
export const moveTier = (tid: number, up: boolean) => write(api<Group>(`${base}/tiers/${tid}/move`, { json: { up } }))
export const deleteTier = (tid: number) => write(api(`${base}/tiers/${tid}`, { method: 'DELETE' }))
export const addMember = (tid: number, productId: number, move = false) => write(api<Group>(`${base}/tiers/${tid}/members`, { json: { productId, move } }))
export const removeMember = (pid: number) => write(api(`${base}/members/${pid}`, { method: 'DELETE' }))
