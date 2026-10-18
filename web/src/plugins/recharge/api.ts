import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import type { CountResult, FloatPage, LedgerRow, Package, Rate, Report, SaleLine, Setup, Source, TillData } from './types'

const base = '/x/recharge'
export const key = (...k: unknown[]) => ['x', 'recharge', ...k]

export const tillDataQuery = queryOptions({ queryKey: key('till'), queryFn: () => api<TillData>(`${base}/till`), staleTime: 15_000 })
export const saleLinesQuery = (sale: number) => queryOptions({ queryKey: key('sale-lines', sale), queryFn: () => api<SaleLine[]>(`${base}/sale-lines?sale=${sale}`), staleTime: 60_000 })
export const sourcesQuery = queryOptions({ queryKey: key('sources'), queryFn: () => api<Source[]>(`${base}/refill/sources`) })
export const setupQuery = queryOptions({ queryKey: key('setup'), queryFn: () => api<Setup>(`${base}/setup`) })
export const floatQuery = queryOptions({ queryKey: key('float'), queryFn: () => api<FloatPage>(`${base}/float`) })
export const ledgerQuery = (qs: string) => queryOptions({ queryKey: key('ledger', qs), queryFn: () => api<LedgerRow[]>(`${base}/ledger?${qs}`) })
export const reportQuery = (qs: string) => queryOptions({ queryKey: key('report', qs), queryFn: () => api<Report>(`${base}/report?${qs}`) })
export const ledgerCsv = (qs: string) => `/api/v1${base}/ledger?${qs}&format=csv`

/** Every write refreshes the recharge queries (floats, packages, setup…). */
async function write<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } finally {
    void queryClient.invalidateQueries({ queryKey: key() })
  }
}

export const quickPackage = (carrierId: number, name: string, price: string, commission: string) =>
  write(api<{ package: Package; created: boolean }>(`${base}/packages/quick`, { json: { carrierId, name, price, commission } }))
export const drawerCounts = (counts: { deviceId: number; counted: string }[]) =>
  write(api<CountResult[]>(`${base}/counts/drawer`, { json: { counts } }))
export const adminCounts = (counts: { deviceId: number; counted: string }[]) => write(api<CountResult[]>(`${base}/counts`, { json: { counts } }))
export type RefillIn = { deviceId: number; supplierId: number; amount: string; bonus: string; credit: boolean; source: string; reference: string }
export const counterRefill = (r: RefillIn) => write(api<{ receiptId: number }>(`${base}/refill/counter`, { json: r }))
export const adminRefill = (r: RefillIn) => write(api<{ receiptId: number }>(`${base}/refill`, { json: r }))
export const createCarrier = (name: string) => write(api<{ id: number }>(`${base}/carriers`, { json: { name } }))
export const renameCarrier = (id: number, name: string) => write(api(`${base}/carriers/${id}`, { method: 'PUT', json: { name } }))
export const carrierSupplier = (id: number, supplierId: number) => write(api(`${base}/carriers/${id}/supplier`, { method: 'PUT', json: { supplierId } }))
export const carrierActive = (id: number, on: boolean) => write(api(`${base}/carriers/${id}/active`, { json: { on } }))
export const setRates = (id: number, rates: Rate[]) => write(api(`${base}/carriers/${id}/rates`, { method: 'PUT', json: rates }))
export type DeviceIn = { carrierId: number; label: string; number: string; reload: boolean; money: boolean; target: string; opening: string }
export const createDevice = (d: DeviceIn) => write(api<{ id: number }>(`${base}/devices`, { json: d }))
export const updateDevice = (id: number, d: DeviceIn) => write(api(`${base}/devices/${id}`, { method: 'PUT', json: d }))
export const deviceActive = (id: number, on: boolean) => write(api(`${base}/devices/${id}/active`, { json: { on } }))
export type PackageIn = { carrierId: number; name: string; price: string; floatCost: string; sort: number; isActive?: boolean }
export const createPackage = (p: PackageIn) => write(api<Package>(`${base}/packages`, { json: p }))
export const updatePackage = (id: number, p: PackageIn) => write(api(`${base}/packages/${id}`, { method: 'PUT', json: p }))
export const reviewPackage = (id: number) => write(api(`${base}/packages/${id}/review`, { method: 'POST' }))
