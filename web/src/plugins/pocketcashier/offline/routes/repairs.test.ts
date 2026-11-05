import { describe, expect, it } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import type { Job } from '@/plugins/repairs/types'
import type { Action } from '../../store/outbox'
import { RouteError, type RouteCtx } from '../fetch'
import { tempNumber } from '../fetch'
import { repairsRoutes } from './repairs'

const job = { id: 3, ticketNo: 'R-0003', ticketCode: 'RPR000003', customerName: 'Sunil', customerPhone: '0771', repairType: 'Screen', deviceModel: 'Nokia', status: 'received', due: '', dueUrgent: false, createdAt: '', collectedAt: '', customerId: 1, repairedBy: '', fault: '', notes: '', promisedDate: '', urgent: false, warrantyDays: 0, warrantyUntil: '', inWarranty: false, saleId: null, saleReceipt: '', reworkOf: null, readyAt: '', cancelledAt: '', parts: [], charges: [{ id: 1, label: 'Labour', amount: '500.00' }], estimate: '500.00', deposits: [], depositTotal: '0.00', legacyHeld: '0.00', outstanding: '0.00', repairerPaid: '0.00', payouts: [], warrantyMode: 'days' } as unknown as Job
const part = { config: { warrantyMode: 'days', defaultWarrantyDays: 30 }, suggest: { types: ['Screen'] }, labour: 77, jobs: [job], bills: { 3: { customerId: 1, lines: [{ kind: 'labour', productId: 77, name: 'Labour — R-0003', qty: '', price: '500.00' }], free: false, onAccount: '0.00', deposits: '0.00', legacyHeld: '0.00' } } }
const copy = { customers: [{ id: 1, name: 'Sunil', phone: '0771', balance: '0', creditLimit: '0' }], products: [{ id: 9, name: 'Screen part', sellingPrice: '1200.00' }] } as unknown as TillCopy
const ctx = (over: Partial<RouteCtx> = {}): RouteCtx => ({ parts: { repairs: part }, copy, params: {}, query: new URLSearchParams(), body: undefined, ref: 'r1abcd', out: [], userId: 7, ...over })
const route = (m: string, p: string) => repairsRoutes.find((r) => r.method === m && r.path === p)!
const call = (ref: string, method: string, path: string, body?: unknown): Action => ({ ref, kind: 'call', userId: 7, at: '', body: { method, path: `/api/v1/x/repairs${path}`, label: '', body } })
const read = (id: string, out: Action[]) => route('GET', '/x/repairs/jobs/:id').answer(ctx({ params: { id }, out })) as Job

describe('Repairs with the PC down', () => {
  it('reads config, suggestions and labour', () => {
    expect(route('GET', '/x/repairs/config').answer(ctx())).toMatchObject({ defaultWarrantyDays: 30 })
    expect(route('GET', '/x/repairs/suggest').answer(ctx())).toMatchObject({ types: ['Screen'] })
    expect(route('GET', '/x/repairs/labour').answer(ctx())).toEqual({ productId: 77 })
  })
  it('lists jobs, finds by ticket, name, phone or device', () => {
    const list = (qs: string) => (route('GET', '/x/repairs/jobs').answer(ctx({ query: new URLSearchParams(qs) })) as Job[]).map((j) => j.id)
    expect(list('show=open')).toEqual([3])
    for (const q of ['R-0003', 'RPR000003', 'sunil', '0771', 'nokia']) expect(list(`show=all&q=${q}`)).toEqual([3])
    expect(list('show=all&q=none')).toEqual([])
    expect(list('show=collected')).toEqual([])
  })
  it('a new job offline shows a temporary number, and shows at once', () => {
    const r = route('POST', '/x/repairs/jobs')
    expect(r.queue).toBe(true)
    const body = { customerId: 1, repairType: 'Battery', deviceModel: 'Samsung', fault: 'Dead', notes: '', repairedBy: '', promisedDate: '', urgent: false }
    const made = r.answer(ctx({ body })) as Job
    expect(made).toMatchObject({ id: tempNumber('r1abcd'), ticketNo: 'New-r1ab', customerName: 'Sunil', status: 'received', deviceModel: 'Samsung', parts: [] })
    expect(read(String(tempNumber('j1')), [call('j1', 'POST', '/jobs', body)])).toMatchObject({ deviceModel: 'Samsung' })
    const list = route('GET', '/x/repairs/jobs').answer(ctx({ out: [call('j1', 'POST', '/jobs', body)], query: new URLSearchParams('show=open') })) as Job[]
    expect(list.map((j) => j.deviceModel)).toEqual(['Samsung', 'Nokia'])
  })
  it('status, parts and charges waiting on the phone show on the job', () => {
    const j = read('3', [
      call('a', 'POST', '/jobs/3/status', { status: 'ready' }),
      call('b', 'POST', '/jobs/3/parts', { productId: 9, qty: '2', warrantyDays: 0 }),
      call('c', 'POST', '/jobs/3/charges', { label: 'Rush', amount: '100' }),
    ])
    expect(j.status).toBe('ready')
    expect(j.parts).toEqual([expect.objectContaining({ productId: 9, name: 'Screen part', qty: '2', price: '1200.00' })])
    expect(j.charges.map((x) => x.label)).toEqual(['Labour', 'Rush'])
    expect(j.estimate).toBe('3000.00')
  })
  it('removing a part or charge shows too', () => {
    expect(read('3', [call('d', 'DELETE', '/jobs/3/charges/1')]).charges).toEqual([])
  })
  it('a deposit waiting on the phone shows, with its amount', () => {
    const pay: Action = { ref: 'p1', kind: 'call', userId: 7, at: '', body: { method: 'POST', path: '/api/v1/till/customers/1/payment', label: '', body: { amount: '200' } } }
    const j = read('3', [pay, call('l', 'POST', '/jobs/3/deposits', { paymentId: tempNumber('p1') })])
    expect(j.depositTotal).toBe('200.00')
    expect(j.deposits[0]).toMatchObject({ amount: '200.00' })
  })
  it('a job cancelled on the phone reads as cancelled', () => {
    expect(read('3', [call('x', 'POST', '/jobs/3/cancel')]).status).toBe('cancelled')
  })
  it('a job already sold on this phone isn\'t open any more', () => {
    const sale: Action = { ref: 's', kind: 'sale', userId: 7, at: '', body: { items: [{ plugin: { repairs: { job: 3, ticket: 'R-0003' } } }] } }
    expect(read('3', [sale]).status).toBe('collected')
  })
  it('the bill comes from the part while the job is unchanged', () => {
    expect(route('GET', '/x/repairs/jobs/:id/bill').answer(ctx({ params: { id: '3' } }))).toEqual(part.bills[3])
  })
  it('a job changed on the phone gets a bill worked out here', () => {
    const b = route('GET', '/x/repairs/jobs/:id/bill').answer(ctx({ params: { id: '3' }, out: [call('b', 'POST', '/jobs/3/parts', { productId: 9, qty: '2' })] })) as { lines: { kind: string; productId: number; qty: string; price: string }[] }
    expect(b.lines).toEqual([expect.objectContaining({ kind: 'part', productId: 9, qty: '2' }), expect.objectContaining({ kind: 'labour', productId: 77, price: '500.00' })])
  })
  it('nothing to bill says so, as the PC does', () => {
    expect(() => route('GET', '/x/repairs/jobs/:id/bill').answer(ctx({ params: { id: '3' }, parts: { repairs: { ...part, jobs: [{ ...job, charges: [] }], bills: {} } } }))).toThrow(RouteError)
  })
  it('a write answers with the job as it now stands', () => {
    const r = route('POST', '/x/repairs/jobs/:id/status')
    expect(r.queue).toBe(true)
    expect(r.answer(ctx({ params: { id: '3' }, body: { status: 'in_progress' } }))).toMatchObject({ status: 'in_progress' })
  })
  it('with nothing stored yet the screen stays empty, not broken', () => {
    expect(route('GET', '/x/repairs/jobs').answer(ctx({ parts: {} }))).toEqual([])
  })
})
