import { api } from '@/api/client'
import type { AskReply } from './types'

const base = '/x/aicatalog/jobs'

export type JobInfo = { key: string; label: string; desc: string; warning: string; oneByOne: boolean; options?: unknown; noAnswer?: string[] }
export type JobRow = {
  productId: number; kind: string; key: string; label: string; old: string; new: string; reason: string
  isNew: boolean; change: boolean; ticked: boolean; needs?: string; choices?: { value: string; label: string }[]; extra?: Record<string, string>
}
export type JobPreview = { rows: JobRow[]; asked: number[]; names: Record<string, string>; remaining: number; state?: string }
export type JobAsk = { size: number; categoryId: number; filter: '' | 'skipped'; force?: boolean; ids?: number[]; options?: unknown; reply?: string }
export type JobApplied = { applied: number; skipped: { productId: number; label: string; why: string }[]; summary: string }

export const jobsQuery = { queryKey: ['x', 'aicatalog', 'jobs'], queryFn: () => api<JobInfo[]>(base) }
export const jobNext = (job: string, b: JobAsk) => api<AskReply<JobPreview> & { left?: number }>(`${base}/${job}/next`, { json: b })
export const jobRedoOver = (job: string) => api(`${base}/${job}/redo-over`, { method: 'POST' })
export const jobApply = (job: string, b: { rows: JobRow[]; asked: number[]; suggested: number[]; state?: string }) => api<JobApplied>(`${base}/${job}/apply`, { json: b })

/** A row's identity on the page. */
export const rowKey = (r: JobRow, i: number) => `${r.productId}:${r.kind}:${r.key}:${i}`

/** What the server said should start ticked. */
export function initialTicks(rows: JobRow[]): Set<string> {
  return new Set(rows.flatMap((r, i) => (r.ticked && !r.needs ? [rowKey(r, i)] : [])))
}

/** Ticks every row of a kind that can be ticked. A one-by-one job has no tick-all. */
export function tickAll(rows: JobRow[], ticks: Set<string>, kind: string, oneByOne: boolean): Set<string> {
  if (oneByOne) return ticks
  const next = new Set(ticks)
  rows.forEach((r, i) => { if (r.kind === kind && !r.needs) next.add(rowKey(r, i)) })
  return next
}

/** Rows grouped under their product, in the batch's order. */
export function byProduct(rows: JobRow[]): { productId: number; rows: { row: JobRow; key: string }[] }[] {
  const out = new Map<number, { row: JobRow; key: string }[]>()
  rows.forEach((row, i) => {
    const list = out.get(row.productId) ?? []
    list.push({ row, key: rowKey(row, i) })
    out.set(row.productId, list)
  })
  return [...out].map(([productId, list]) => ({ productId, rows: list }))
}

/** What apply needs: the ticked rows, and who had a suggestion at all. */
export function applyBody(pv: JobPreview, ticks: Set<string>) {
  return {
    rows: pv.rows.filter((r, i) => ticks.has(rowKey(r, i))),
    asked: pv.asked,
    suggested: [...new Set(pv.rows.map((r) => r.productId))],
    state: pv.state,
  }
}

/** A typed answer to what a row needs: "new" and blanks are no answer. */
export const answersNeed = (typed: string) => typed.trim() !== '' && typed.trim().toLowerCase() !== 'new'

/** Where a run of batches stands: "Batch 2 of 6: products 6–10 of 27". firstLeft is what was left when the run began. */
export function batchLabel(firstLeft: number, left: number, size: number): string {
  if (firstLeft <= 0) return ''
  const done = Math.max(0, firstLeft - left)
  const each = Math.max(1, size)
  const batches = Math.ceil(firstLeft / each)
  const from = Math.min(done + 1, firstLeft)
  const to = Math.min(done + each, firstLeft)
  return `Batch ${Math.min(Math.floor(done / each) + 1, batches)} of ${batches}: ${from === to ? `product ${from}` : `products ${from}–${to}`} of ${firstLeft}`
}
