import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import type { ChequeLine } from '@/api/types.gen'
import { printLabel } from '@/features/print/labels/print'
import { ticketRequest, type TicketLabel } from './labels'
import type { Bill, Config, Job, JobInput, Report, Row, Suggestions } from './types'

const base = '/x/repairs'
const key = (...k: unknown[]) => ['x', 'repairs', ...k]

export type Show = 'open' | 'ready' | 'collected' | 'cancelled' | 'all'
export const jobsQuery = (show: Show, q = '', range = '') =>
  queryOptions({ queryKey: key('jobs', show, q, range), queryFn: () => api<Row[]>(`${base}/jobs?show=${show}&q=${encodeURIComponent(q)}${range}`) })
export const jobQuery = (id: number) => queryOptions({ queryKey: key('job', id), queryFn: () => api<Job>(`${base}/jobs/${id}`), enabled: id > 0 })
export const suggestQuery = queryOptions({ queryKey: key('suggest'), queryFn: () => api<Suggestions>(`${base}/suggest`), staleTime: 60_000 })
export const configQuery = queryOptions({ queryKey: key('config'), queryFn: () => api<Config>(`${base}/config`), staleTime: 60_000 })
export const labourQuery = queryOptions({ queryKey: key('labour'), queryFn: () => api<{ productId: number }>(`${base}/labour`), staleTime: Infinity })
export const reportQuery = (range: string) => queryOptions({ queryKey: key('report', range), queryFn: () => api<Report>(`${base}/report?${range}`) })

/** Every write refreshes the repairs queries (lists, the job, suggestions). */
async function write<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } finally {
    void queryClient.invalidateQueries({ queryKey: key() })
  }
}

const job = (id: number, path = '', init?: Parameters<typeof api>[1]) => write(api<Job>(`${base}/jobs/${id}${path}`, init))

export const createJob = (j: JobInput) => write(api<Job>(`${base}/jobs`, { json: j }))
export const updateJob = (id: number, j: JobInput) => job(id, '', { method: 'PUT', json: j })
export const setStatus = (id: number, status: string) => job(id, '/status', { json: { status } })
export const setUrgent = (id: number, urgent: boolean, promisedDate = '') => job(id, '/urgent', { json: { urgent, promisedDate } })
export const cancelJob = (id: number) => job(id, '/cancel', { method: 'POST' })
export const addPart = (id: number, productId: number, qty: string, warrantyDays = 0) => job(id, '/parts', { json: { productId, qty, warrantyDays } })
export const removePart = (id: number, pid: number) => job(id, `/parts/${pid}`, { method: 'DELETE' })
export const addCharge = (id: number, label: string, amount: string) => job(id, '/charges', { json: { label, amount } })
export const removeCharge = (id: number, cid: number) => job(id, `/charges/${cid}`, { method: 'DELETE' })
export const linkDeposit = (id: number, paymentId: number) => job(id, '/deposits', { json: { paymentId } })
export const startRework = (id: number) => job(id, '/rework', { method: 'POST' })
export const payRepairer = (id: number, amount: string, source: string, note: string, cheques?: ChequeLine[]) =>
  job(id, '/payouts', { json: { amount, source, note, ...(cheques ? { method: 'cheque', cheques } : {}) } })
export const refundHeld = (id: number) => write(api<{ receiptId: number; amount: string }>(`${base}/jobs/${id}/refund-held`, { method: 'POST' }))
export const getBill = (id: number) => api<Bill>(`${base}/jobs/${id}/bill`)
export const saveConfig = (c: Config) => write(api(`${base}/config`, { method: 'PUT', json: c }))
export const slipPath = (id: number) => `${base}/jobs/${id}/print`
/** The ticket label: the picture in Image mode, else core's barcode label (/till/labels). */
export const printTicketLabel = (t: TicketLabel) =>
  printLabel({ kind: 'repairs.ticket', data: t, size: '', count: 1, classic: { path: '/till/labels', body: ticketRequest(t) } })
