import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import type { Controlled, Flags, Found, Patient, RecentSale, Register, Script, ScriptInput, Unrecorded } from './types'

const base = '/x/pharmacy'
const key = (...k: unknown[]) => ['x', 'pharmacy', ...k]

export const searchQuery = (q: string, customers = false) =>
  queryOptions({ queryKey: key('search', q, customers), queryFn: () => api<Found[]>(`${base}/patients?q=${encodeURIComponent(q)}${customers ? '&customers=1' : ''}`) })
export const patientQuery = (id: number) => queryOptions({ queryKey: key('patient', id), queryFn: () => api<Patient>(`${base}/patients/${id}`), enabled: id > 0 })
export const scriptsQuery = (id: number) => queryOptions({ queryKey: key('scripts', id), queryFn: () => api<Script[]>(`${base}/patients/${id}/scripts`), enabled: id > 0 })
export const recentSalesQuery = (q: string) => queryOptions({ queryKey: key('recent', q), queryFn: () => api<RecentSale[]>(`${base}/sales/recent?q=${encodeURIComponent(q)}`) })
export const productFlagsQuery = (pid: number) =>
  queryOptions({ queryKey: key('product', pid), queryFn: () => api<Flags>(`${base}/product/${pid}`), enabled: pid > 0 })
/** The cart's controlled / Rx-only products (cached briefly; ids sorted so the key is stable). */
export const tillInfoQuery = (ids: number[]) =>
  queryOptions({ queryKey: key('till-info', ids), queryFn: () => api<Record<string, Flags>>(`${base}/till-info?ids=${ids.join(',')}`), enabled: ids.length > 0, staleTime: 60_000 })
export const controlledQuery = queryOptions({ queryKey: key('controlled'), queryFn: () => api<Controlled[]>(`${base}/controlled`) })
export const registerQuery = (pid: number, from: string) =>
  queryOptions({ queryKey: key('register', pid, from), queryFn: () => api<Register>(`${base}/controlled/${pid}${from ? `?from=${from}` : ''}`) })
export const unrecordedQuery = queryOptions({ queryKey: key('unrecorded'), queryFn: () => api<Unrecorded[]>(`${base}/controlled/missing`) })

async function write<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } finally {
    void queryClient.invalidateQueries({ queryKey: key() })
  }
}

export type NewPatient = { name: string; phone: string; allergies: string; nic?: string; dob?: string; sex?: string; notes?: string }
export const registerPatient = (p: NewPatient) => write(api<Patient>(`${base}/patients`, { json: p }))
export const patientFromCustomer = (cid: number) => write(api<Patient>(`${base}/patients/from-customer/${cid}`, { method: 'POST' }))
export const updatePatient = (id: number, p: Omit<Patient, 'id' | 'customerId' | 'name' | 'phone'>) =>
  write(api<Patient>(`${base}/patients/${id}`, { method: 'PUT', json: p }))
export const createScript = (s: ScriptInput) => write(api<{ id: number }>(`${base}/prescriptions`, { json: s }))
export const updateScript = (id: number, s: ScriptInput) => write(api(`${base}/prescriptions/${id}`, { method: 'PUT', json: s }))
export const setScriptActive = (id: number, active: boolean) => write(api(`${base}/prescriptions/${id}/active`, { json: { active } }))
export type DispenseInput = { saleId: number; patientId: number; prescriptionId?: number; prescriber?: string; lines?: { productId: number; rxItemId: number }[] }
export const recordDispense = (d: DispenseInput) => write(api<{ id: number }>(`${base}/dispenses`, { json: d }))
