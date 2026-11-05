import { dec, num } from '@/lib/money'
import type { Bill, Job, Status } from '@/plugins/repairs/types'
import type { Action } from '../../store/outbox'
import type { CallInput } from '../../sync'
import { RouteError, type OfflineRoute, type RouteCtx } from '../fetch'

type Part = { config: unknown; suggest: unknown; labour: number; jobs: Job[]; bills: Record<string, Bill> }
type JobIn = Partial<Job> & { customerId?: number }

const partOf = (c: RouteCtx) => c.parts.repairs as Part | undefined
const JOBS = '/x/repairs/jobs'
const asId = (ref: string) => `temp:${ref}` as unknown as number
const OPEN: Status[] = ['received', 'in_progress', 'ready']

function newJob(c: RouteCtx, ref: string, b: JobIn): Job {
  const cust = c.copy?.customers.find((x) => x.id === b.customerId)
  return {
    id: asId(ref), ticketNo: `New-${ref.slice(0, 4)}`, ticketCode: '', customerName: cust?.name ?? '', customerPhone: cust?.phone ?? '', repairType: b.repairType ?? '', deviceModel: b.deviceModel ?? '',
    status: 'received', due: '', dueUrgent: false, createdAt: new Date().toISOString(), collectedAt: '', customerId: b.customerId ?? null, repairedBy: b.repairedBy ?? '', fault: b.fault ?? '', notes: b.notes ?? '',
    promisedDate: b.promisedDate ?? '', urgent: b.urgent ?? false, warrantyDays: 0, warrantyUntil: '', inWarranty: false, saleId: null, saleReceipt: '', reworkOf: null, readyAt: '', cancelledAt: '',
    parts: [], charges: [], estimate: '0.00', deposits: [], depositTotal: '0.00', legacyHeld: '0.00', outstanding: cust?.balance ?? '0.00', repairerPaid: '0.00', payouts: [], warrantyMode: 'days',
  }
}

const estimate = (j: Job) => dec(j.charges.reduce((t, x) => t + num(x.amount), 0) + j.parts.reduce((t, p) => t + num(p.qty) * num(p.price), 0))

/** One waiting call, applied to a job as the PC will. */
function applyCall(c: RouteCtx, j: Job, sub: string, call: CallInput, seq: number, out: Action[]): Job {
  const b = (call.body ?? {}) as Record<string, string>
  if (sub === '/status') return { ...j, status: b.status as Status }
  if (sub === '/cancel') return { ...j, status: 'cancelled' }
  if (sub === '/urgent') return { ...j, urgent: !!call.body && !!(call.body as { urgent?: boolean }).urgent, promisedDate: b.promisedDate ?? j.promisedDate }
  if (sub === '') return { ...j, ...(call.body as JobIn), id: j.id }
  if (sub === '/parts') {
    const p = c.copy?.products.find((x) => x.id === Number(b.productId))
    return { ...j, parts: [...j.parts, { id: -seq, productId: Number(b.productId), name: p?.name ?? '', qty: String(b.qty), price: p?.sellingPrice ?? '0.00', oldDiscount: '', warrantyDays: Number(b.warrantyDays) || 0 }] }
  }
  if (sub === '/charges') return { ...j, charges: [...j.charges, { id: -seq, label: b.label, amount: dec(num(b.amount)) }] }
  const gone = /^\/(parts|charges)\/(-?\d+)$/.exec(sub)
  if (gone && call.method === 'DELETE') return gone[1] === 'parts' ? { ...j, parts: j.parts.filter((p) => String(p.id) !== gone[2]) } : { ...j, charges: j.charges.filter((x) => String(x.id) !== gone[2]) }
  if (sub === '/deposits') {
    const ref = String(b.paymentId).replace(/^temp:/, '')
    const paid = out.find((a) => a.ref === ref)?.body as CallInput | undefined
    const amount = dec(num((paid?.body as { amount?: string } | undefined)?.amount))
    return { ...j, deposits: [...j.deposits, { paymentId: b.paymentId as unknown as number, amount, receiptNo: '', at: '' }], depositTotal: dec(num(j.depositTotal) + num(amount)) }
  }
  return j
}

/** The jobs as the phone should show them: the PC's, plus what waits here. */
function jobs(c: RouteCtx, extra: Action[] = []): Job[] {
  const out = [...c.out, ...extra]
  let all = [...(partOf(c)?.jobs ?? [])]
  out.forEach((a, seq) => {
    if (a.kind === 'sale') {
      const sold = new Set(((a.body as { items?: { plugin?: { repairs?: { job?: unknown } } }[] }).items ?? []).map((it) => String(it.plugin?.repairs?.job ?? '')))
      all = all.map((j) => (sold.has(String(j.id)) ? { ...j, status: 'collected' as const } : j))
    }
    if (a.kind !== 'call') return
    const call = a.body as CallInput
    const at = call.path.indexOf(JOBS)
    if (at < 0) return
    const rest = call.path.slice(at + JOBS.length)
    if (rest === '') return void all.unshift(newJob(c, a.ref, call.body as JobIn))
    const m = /^\/([^/]+)(.*)$/.exec(rest)
    if (m) all = all.map((j) => (String(j.id) === decodeURIComponent(m[1]) ? applyCall(c, j, m[2], call, seq + 1, out) : j))
  })
  return all.map((j) => ({ ...j, estimate: estimate(j) }))
}

function list(c: RouteCtx): Job[] {
  const q = (c.query.get('q') ?? '').trim().toLowerCase()
  const show = c.query.get('show') ?? 'open'
  return jobs(c).filter((j) => {
    if (q) return [j.ticketNo, j.ticketCode, j.customerName, j.customerPhone, j.deviceModel].some((f) => f.toLowerCase().includes(q))
    if (show === 'all') return true
    if (show === 'open') return OPEN.includes(j.status)
    return j.status === show
  })
}

function bill(c: RouteCtx): Bill {
  const part = partOf(c)
  const j = jobs(c).find((x) => String(x.id) === c.params.id)
  if (!j || !part) throw new RouteError("This repair isn't on this phone")
  if (!j.parts.length && !j.charges.length) throw new RouteError('add a part or labour to the job first')
  const unchanged = !c.out.some((a) => a.kind === 'call' && (a.body as CallInput).path.includes(`${JOBS}/${j.id}`))
  const stored = part.bills[c.params.id]
  if (unchanged && stored) return stored
  const free = j.reworkOf !== null
  const none = { discount: '', discountType: '' as const }
  return {
    customerId: j.customerId, free, onAccount: '0.00', deposits: j.depositTotal, legacyHeld: j.legacyHeld,
    lines: [
      ...j.parts.map((p) => ({ kind: 'part' as const, productId: p.productId, name: p.name, qty: p.qty, price: '', ...(free ? { discount: '100', discountType: 'percent' as const } : none) })),
      ...j.charges.map((x) => ({ kind: 'labour' as const, productId: part.labour, name: `${x.label} — ${j.ticketNo}`, qty: '', price: free ? '0.00' : x.amount, ...none })),
    ],
  }
}

/** A write: queues, and answers with the job as it now stands. */
const change = (method: string, sub: string, label: string): OfflineRoute => ({
  method, path: `${JOBS}/:id${sub}`, queue: true, label: () => label,
  answer: (c) => {
    const path = sub.replace(/:([a-z]+)/g, (_, k: string) => c.params[k])
    const me: Action = { ref: c.ref, kind: 'call', userId: c.userId, at: '', body: { method, path: `/api/v1${JOBS}/${c.params.id}${path}`, label, body: c.body } }
    return jobs(c, [me]).find((j) => String(j.id) === c.params.id) ?? null
  },
})

export const repairsRoutes: OfflineRoute[] = [
  { method: 'GET', path: '/x/repairs/config', answer: (c) => partOf(c)?.config ?? { warrantyMode: 'days', defaultWarrantyDays: 0 } },
  { method: 'GET', path: '/x/repairs/suggest', answer: (c) => partOf(c)?.suggest ?? { types: [], models: [], repairers: [], labour: [] } },
  { method: 'GET', path: '/x/repairs/labour', answer: (c) => ({ productId: partOf(c)?.labour ?? 0 }) },
  { method: 'GET', path: JOBS, answer: list },
  { method: 'GET', path: `${JOBS}/:id`, answer: (c) => jobs(c).find((j) => String(j.id) === c.params.id) ?? null },
  { method: 'GET', path: `${JOBS}/:id/bill`, answer: bill },
  { method: 'POST', path: JOBS, queue: true, label: (c) => `New repair: ${(c.body as JobIn).deviceModel}`, answer: (c) => newJob(c, c.ref, c.body as JobIn) },
  change('PUT', '', 'Repair details changed'),
  change('POST', '/status', 'Repair status changed'),
  change('POST', '/urgent', 'Repair marked urgent or not'),
  change('POST', '/cancel', 'Repair cancelled'),
  change('POST', '/parts', 'Part added to a repair'),
  change('DELETE', '/parts/:pid', 'Part taken off a repair'),
  change('POST', '/charges', 'Labour added to a repair'),
  change('DELETE', '/charges/:cid', 'Labour taken off a repair'),
  change('POST', '/deposits', 'Deposit linked to a repair'),
]
