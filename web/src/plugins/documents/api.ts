import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import { type Range, rangeQS } from '@/features/counter/range'
import type { Grid, Quote, Report, Service, ServiceRequest, TillService } from './types'

const base = '/x/documents'

export const servicesQuery = queryOptions({ queryKey: ['x', 'documents', 'services'], queryFn: () => api<Service[]>(`${base}/services`) })
export const gridQuery = (id: number) => queryOptions({ queryKey: ['x', 'documents', 'grid', id], queryFn: () => api<Grid>(`${base}/services/${id}/grid`) })
export const tillServicesQuery = queryOptions({ queryKey: ['x', 'documents', 'till'], queryFn: () => api<TillService[]>(`${base}/till`), staleTime: 60_000 })
export const reportQuery = (range: Range) =>
  queryOptions({ queryKey: ['x', 'documents', 'report', range], queryFn: () => api<Report>(`${base}/report?${rangeQS(range).slice(1)}`) })

export type JobAsk = { service: number; size: string; color: boolean; double: boolean; copies: number }
export const quote = (j: JobAsk) =>
  api<Quote>(`${base}/quote?service=${j.service}&size=${encodeURIComponent(j.size)}&color=${j.color ? 1 : 0}&double=${j.double ? 1 : 0}&copies=${j.copies}`)

async function write<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } finally {
    void queryClient.invalidateQueries({ queryKey: ['x', 'documents'] })
    void queryClient.invalidateQueries({ queryKey: ['admin'] })
  }
}

export const createService = (body: ServiceRequest) => write(api<{ id: number; productId: number }>(`${base}/services`, { json: body }))
export const updateService = (id: number, body: ServiceRequest) => write(api(`${base}/services/${id}`, { method: 'PUT', json: body }))
export const setActive = (id: number, active: boolean) => write(api(`${base}/services/${id}/active`, { json: { active } }))
export const saveGrid = (id: number, g: Grid) => write(api<Grid>(`${base}/services/${id}/grid`, { method: 'PUT', json: g }))
export const ownUse = (j: JobAsk) =>
  api(`${base}/own-use`, { json: { serviceId: j.service, size: j.size, color: j.color, double: j.double, copies: j.copies } })
export const recordJobs = (saleId: number, jobs: unknown[]) => api(`${base}/jobs`, { json: { saleId, jobs } })
