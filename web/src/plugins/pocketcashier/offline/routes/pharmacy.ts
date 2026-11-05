import type { CopyCustomer } from '@/api/types.gen'
import type { Patient, Script } from '@/plugins/pharmacy/types'
import { findSales } from '../../store/recent'
import type { CallInput } from '../../sync'
import { search } from '../../catalog'
import { tempNumber, type OfflineRoute, type RouteCtx } from '../fetch'

type Part = { patients: Patient[]; scripts: Record<string, Script[]>; flags: Record<string, unknown>; doses: unknown[]; notes: string[] }
type NewPatient = Partial<Patient>
type ScriptIn = Partial<Omit<Script, 'items'>> & { patientId?: number | string; items?: { drugText?: string; productId?: number | null; groupId?: number | null; dosage?: string; note?: string; quantity?: string; perDay?: string }[] }

const partOf = (c: RouteCtx) => c.parts.pharmacy as Part | undefined
const calls = (c: RouteCtx) => c.out.filter((a) => a.kind === 'call').map((a) => ({ ref: a.ref, call: a.body as CallInput }))
const blank = { customerId: 0, name: '', phone: '', dob: '', sex: '', nic: '', allergies: '', notes: '' }

function fromCustomer(c: RouteCtx, cid: string, ref: string): Patient {
  const x: CopyCustomer | undefined = c.copy?.customers.find((y) => String(y.id) === cid)
  return { ...blank, id: tempNumber(ref), customerId: Number(cid), name: x?.name ?? '', phone: x?.phone ?? '' }
}

/** A new patient is a new customer too: the sale goes on that customer, so it needs
 * its own temporary number, read from the PC's reply. */
const customerNumber = (ref: string) => tempNumber(ref) + 1
const newPatient = (b: NewPatient, ref: string): Patient => ({ ...blank, ...b, id: tempNumber(ref), customerId: customerNumber(ref) })

/** The patients as the phone should show them: the PC's, plus those made or changed here. */
function patients(c: RouteCtx): Patient[] {
  let all = [...(partOf(c)?.patients ?? [])]
  for (const { ref, call } of calls(c)) {
    if (call.path.endsWith('/x/pharmacy/patients')) all.push(newPatient(call.body as NewPatient, ref))
    const made = /\/x\/pharmacy\/patients\/from-customer\/(\d+)$/.exec(call.path)
    if (made) all.push(fromCustomer(c, made[1], ref))
    const edit = call.method === 'PUT' && /\/x\/pharmacy\/patients\/([^/]+)$/.exec(call.path)
    if (edit) all = all.map((p) => (String(p.id) === edit[1] ? { ...p, ...(call.body as NewPatient) } : p))
  }
  return all
}

function find(c: RouteCtx) {
  const q = (c.query.get('q') ?? '').trim().toLowerCase()
  const has = (...fields: string[]) => !q || fields.some((f) => f.toLowerCase().includes(q))
  const all = patients(c)
  const out = all.filter((p) => has(p.name, p.phone, p.nic))
    .map((p) => ({ patientId: p.id, customerId: p.customerId, name: p.name, phone: p.phone, nic: p.nic, allergies: p.allergies }))
  if (c.query.get('customers') !== '1') return out
  const taken = new Set(all.map((p) => p.customerId))
  const rest = (c.copy?.customers ?? []).filter((x) => !taken.has(x.id) && has(x.name, x.phone))
    .map((x) => ({ patientId: 0, customerId: x.id, name: x.name, phone: x.phone, nic: '', allergies: '' }))
  return [...out, ...rest]
}

/** What Add would put in the cart: a script written offline names its product. */
function suggestOf(c: RouteCtx, productId?: number | null) {
  const p = c.copy?.products.find((x) => x.id === productId)
  return p ? { productId: p.id, name: p.name, stock: p.stockQty } : null
}

function scripts(c: RouteCtx): Script[] {
  const mine = partOf(c)?.scripts[c.params.id] ?? []
  const made: Script[] = calls(c).filter(({ call }) => call.path.endsWith('/x/pharmacy/prescriptions') && String((call.body as ScriptIn).patientId) === c.params.id)
    .map(({ ref, call }) => {
      const b = call.body as ScriptIn
      return {
        id: tempNumber(ref), prescriberName: b.prescriberName ?? '', prescriberReg: b.prescriberReg ?? '', prescribedOn: b.prescribedOn ?? '', notes: b.notes ?? '',
        isRepeat: b.isRepeat ?? false, repeatIntervalDays: b.repeatIntervalDays ?? 0, active: true,
        items: (b.items ?? []).map((it, i) => ({ id: -(i + 1), productId: it.productId ?? null, groupId: it.groupId ?? null, drug: it.drugText ?? '', productName: it.drugText ?? '', groupName: '',
          dosage: it.dosage ?? '', note: it.note ?? '', quantity: it.quantity ?? '0', perDay: it.perDay ?? '0', dispensed: '0', owed: it.quantity ?? '0', lastAt: null, nextDue: null, status: '' as const, suggest: suggestOf(c, it.productId) })),
      }
    })
  return [...made, ...mine]
}

async function recent(c: RouteCtx) {
  const rows = await findSales(c.query.get('q') ?? '', 20).catch(() => [])
  return rows.map((s) => ({ id: s.id, receipt: s.receiptNo, at: s.createdAt, total: s.total, customer: s.customerName, items: s.lines.map((l) => l.name).join(', '), recorded: '', medicines: s.lines.length }))
}

export const pharmacyRoutes: OfflineRoute[] = [
  { method: 'GET', path: '/x/pharmacy/patients', answer: find },
  { method: 'GET', path: '/x/pharmacy/patients/:id', answer: (c) => patients(c).find((p) => String(p.id) === c.params.id) ?? null },
  { method: 'GET', path: '/x/pharmacy/patients/:id/scripts', answer: scripts },
  { method: 'GET', path: '/x/pharmacy/till-info', answer: (c) => {
    const flags = partOf(c)?.flags ?? {}
    return Object.fromEntries((c.query.get('ids') ?? '').split(',').filter((id) => flags[id]).map((id) => [id, flags[id]]))
  } },
  // Only products: medicine groups need the PC. A script can still name any medicine by typing it.
  { method: 'GET', path: '/x/pharmacy/medicines', answer: (c) => search(c.copy, c.query.get('q') ?? '').filter((p) => !p.isService).slice(0, 15)
    .map((p) => ({ kind: 'product', id: p.id, name: p.name, members: 0, dose: '', note: '' })) },
  { method: 'GET', path: '/x/pharmacy/doses', answer: (c) => partOf(c)?.doses ?? [] },
  { method: 'GET', path: '/x/pharmacy/notes', answer: (c) => partOf(c)?.notes ?? [] },
  { method: 'GET', path: '/x/pharmacy/sales/recent', answer: recent },
  { method: 'POST', path: '/x/pharmacy/patients', queue: true, label: (c) => `New patient: ${(c.body as NewPatient).name}`,
    call: (c) => ({ tempFields: { [customerNumber(c.ref)]: 'customerId' } }),
    answer: (c) => newPatient(c.body as NewPatient, c.ref) },
  { method: 'POST', path: '/x/pharmacy/patients/from-customer/:cid', queue: true, label: (c) => `New patient: ${fromCustomer(c, c.params.cid, '').name}`,
    answer: (c) => fromCustomer(c, c.params.cid, c.ref) },
  { method: 'PUT', path: '/x/pharmacy/patients/:id', queue: true, label: () => 'Patient details changed',
    answer: (c) => ({ ...patients(c).find((p) => String(p.id) === c.params.id), ...(c.body as NewPatient) }) },
  { method: 'POST', path: '/x/pharmacy/prescriptions', queue: true, label: () => 'New prescription', answer: (c) => ({ id: tempNumber(c.ref) }) },
  { method: 'PUT', path: '/x/pharmacy/prescriptions/:id', queue: true, label: () => 'Prescription changed', answer: () => ({}) },
  { method: 'POST', path: '/x/pharmacy/prescriptions/:id/active', queue: true, label: () => 'Prescription turned on or off', answer: () => ({}) },
  { method: 'POST', path: '/x/pharmacy/dispenses', queue: true, label: () => 'Medicines given to a patient', answer: (c) => ({ id: tempNumber(c.ref) }) },
]
