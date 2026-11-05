import { describe, expect, it } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import type { Action } from '../../store/outbox'
import type { RouteCtx } from '../fetch'
import { tempNumber } from '../fetch'
import { pharmacyRoutes } from './pharmacy'

const part = {
  patients: [{ id: 1, customerId: 0, name: 'Sunil Perera', phone: '0771', nic: '901', allergies: 'Penicillin', dob: '', sex: '', notes: '' }],
  scripts: { 1: [{ id: 5, active: true, items: [{ id: 50, drug: 'Panadol' }] }] },
  flags: { 4: { controlled: true, schedule: 'S', rxOnly: true, dose: '', note: '' } },
  doses: [{ id: 1, dose: '1x3' }], notes: ['After food'],
}
const copy = { customers: [{ id: 7, name: 'Sunil Silva', phone: '0772', balance: '0', creditLimit: '0' }, { id: 8, name: 'Other', phone: '', balance: '0', creditLimit: '0' }],
  products: [{ id: 4, name: 'Panadol', stockQty: '5', isService: false, barcode: '', barcodes: [], categoryId: 1, badges: [], sellingPrice: '10' }, { id: 5, name: 'Pan service', isService: true, barcode: '', barcodes: [], categoryId: 1, badges: [], sellingPrice: '10' }] } as unknown as TillCopy
const ctx = (over: Partial<RouteCtx> = {}): RouteCtx => ({ parts: { pharmacy: part }, copy, params: {}, query: new URLSearchParams(), body: undefined, ref: 'r1', out: [], userId: 7, ...over })
const route = (m: string, p: string) => pharmacyRoutes.find((r) => r.method === m && r.path === p)!
const call = (ref: string, path: string, body: unknown): Action => ({ ref, kind: 'call', userId: 7, at: '', body: { method: 'POST', path: `/api/v1${path}`, label: '', body } })

describe('Pharmacy with the PC down', () => {
  it('finds a patient by name, phone or NIC', () => {
    const find = (q: string) => route('GET', '/x/pharmacy/patients').answer(ctx({ query: new URLSearchParams({ q }) })) as { patientId: number }[]
    expect(find('sunil').map((f) => f.patientId)).toEqual([1])
    expect(find('0771').map((f) => f.patientId)).toEqual([1])
    expect(find('901').map((f) => f.patientId)).toEqual([1])
  })
  it('also offers customers who aren\'t patients yet', () => {
    const r = route('GET', '/x/pharmacy/patients').answer(ctx({ query: new URLSearchParams('q=sunil&customers=1') })) as { patientId: number; customerId: number }[]
    expect(r).toEqual([expect.objectContaining({ patientId: 1 }), expect.objectContaining({ patientId: 0, customerId: 7 })])
  })
  it('reads a patient and their scripts', () => {
    expect(route('GET', '/x/pharmacy/patients/:id').answer(ctx({ params: { id: '1' } }))).toMatchObject({ allergies: 'Penicillin' })
    expect(route('GET', '/x/pharmacy/patients/:id/scripts').answer(ctx({ params: { id: '1' } }))).toEqual(part.scripts[1])
    expect(route('GET', '/x/pharmacy/patients/:id/scripts').answer(ctx({ params: { id: '9' } }))).toEqual([])
  })
  it('a new patient has a temporary id, and shows up at once', () => {
    const r = route('POST', '/x/pharmacy/patients')
    expect(r.queue).toBe(true)
    const c = ctx({ body: { name: 'New Person', phone: '0779', allergies: 'None' } })
    expect(r.answer(c)).toMatchObject({ id: tempNumber('r1'), customerId: tempNumber('r1') + 1, name: 'New Person' })
    expect(r.call!(c)).toEqual({ tempFields: { [tempNumber('r1') + 1]: 'customerId' } })
    const out = [call('p1', '/x/pharmacy/patients', { name: 'New Person', phone: '0779', allergies: 'None' })]
    expect(route('GET', '/x/pharmacy/patients').answer(ctx({ out, query: new URLSearchParams('q=new') }))).toEqual([expect.objectContaining({ patientId: tempNumber('p1'), name: 'New Person' })])
    expect(route('GET', '/x/pharmacy/patients/:id').answer(ctx({ out, params: { id: String(tempNumber('p1')) } }))).toMatchObject({ allergies: 'None' })
  })
  it('a customer made a patient: temporary id from their details', () => {
    const r = route('POST', '/x/pharmacy/patients/from-customer/:cid')
    expect(r.answer(ctx({ params: { cid: '7' } }))).toMatchObject({ id: tempNumber('r1'), name: 'Sunil Silva', customerId: 7 })
    const out = [call('p2', '/x/pharmacy/patients/from-customer/7', undefined)]
    expect(route('GET', '/x/pharmacy/patients/:id').answer(ctx({ out, params: { id: String(tempNumber('p2')) } }))).toMatchObject({ name: 'Sunil Silva', customerId: 7 })
  })
  it('an edit to a patient shows at once', () => {
    const out: Action[] = [{ ref: 'e', kind: 'call', userId: 7, at: '', body: { method: 'PUT', path: '/api/v1/x/pharmacy/patients/1', label: '', body: { allergies: 'Aspirin' } } }]
    expect(route('GET', '/x/pharmacy/patients/:id').answer(ctx({ out, params: { id: '1' } }))).toMatchObject({ allergies: 'Aspirin' })
  })
  it('a new script on a patient shows at once', () => {
    const out = [call('s1', '/x/pharmacy/prescriptions', { patientId: 1, prescriberName: 'Dr A', items: [{ drugText: 'Amoxil', productId: 4, dosage: '1x3', quantity: '10' }] })]
    const got = route('GET', '/x/pharmacy/patients/:id/scripts').answer(ctx({ out, params: { id: '1' } })) as { id: unknown; items: { drug: string; owed: string }[] }[]
    expect(got).toHaveLength(2)
    expect(got[0]).toMatchObject({ id: tempNumber('s1'), items: [expect.objectContaining({ drug: 'Amoxil', owed: '10', suggest: expect.objectContaining({ productId: 4, name: 'Panadol' }) })] })
    expect(route('POST', '/x/pharmacy/prescriptions').answer(ctx({ body: {} }))).toEqual({ id: tempNumber('r1') })
  })
  it('flags, doses and notes come from the part', () => {
    expect(route('GET', '/x/pharmacy/till-info').answer(ctx({ query: new URLSearchParams('ids=4,9') }))).toEqual({ 4: part.flags[4] })
    expect(route('GET', '/x/pharmacy/doses').answer(ctx())).toEqual(part.doses)
    expect(route('GET', '/x/pharmacy/notes').answer(ctx())).toEqual(part.notes)
  })
  it('finding a medicine uses the products on the phone', () => {
    expect(route('GET', '/x/pharmacy/medicines').answer(ctx({ query: new URLSearchParams('q=pan') }))).toEqual([{ kind: 'product', id: 4, name: 'Panadol', members: 0, dose: '', note: '' }])
  })
  it('a dispense queues, naming what it depends on through its temporary references', () => {
    const r = route('POST', '/x/pharmacy/dispenses')
    expect(r.queue).toBe(true)
    expect(r.answer(ctx({ body: { saleId: 'temp:s', patientId: 'temp:p' } }))).toEqual({ id: tempNumber('r1') })
  })
  it('with nothing stored yet the screen stays empty, not broken', () => {
    const empty = ctx({ parts: {} })
    expect(route('GET', '/x/pharmacy/patients').answer({ ...empty, copy: null })).toEqual([])
    expect(route('GET', '/x/pharmacy/doses').answer(empty)).toEqual([])
  })
})
