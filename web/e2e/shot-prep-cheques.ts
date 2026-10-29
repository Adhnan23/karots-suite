import type { Page } from './fixtures'
import { customerWithHistory, lockersWithHistory, supplierWithHistory } from './shot-data'

const day = (d: number) => new Date(Date.now() + d * 864e5).toISOString().slice(0, 10)

/** Cheques in every state, both directions: a few pending received (one due
 * today, one post-dated), one cleared, one bounced, one cancelled; pending
 * given cheques on a small bank (it goes below zero after them), one cleared.
 * Built once — later shots reuse it. */
export async function chequesWithHistory(page: Page) {
  const get = async (url: string) => (await page.request.get(url)).json()
  const post = async (url: string, data: unknown) => {
    const r = await page.request.post(url, { data })
    if (!r.ok()) throw new Error(`${url}: ${r.status()} ${await r.text()}`)
    return r.status() === 204 ? null : r.json()
  }
  if ((await get('/api/v1/admin/cheques?status=')).length >= 6) return
  await lockersWithHistory(page)
  const lockers = (await get('/api/v1/admin/lockers')).items as { id: number; name: string; kind: string }[]
  const boc = lockers.find((l) => l.name === 'BOC current account')!.id
  const small = lockers.find((l) => l.name === 'HNB savings')?.id ??
    (await post('/api/v1/admin/lockers', { name: 'HNB savings', kind: 'bank', openingBalance: '6000', overdraft: false, cashierAccess: true })).id
  const cust = await customerWithHistory(page)
  const sup = await supplierWithHistory(page)
  const n = String(Date.now()).slice(-4)
  const rc = (no: string, bank: string, amount: string, date: string) => ({ bank, lockerId: 0, number: `${no}${n}`, amount, date, note: '' })
  await post(`/api/v1/admin/customers/${cust}/payment`, { amount: '', method: 'cheque', reference: '', note: '', source: '', oldDebtFirst: false,
    cheques: [rc('31', 'Bank of Ceylon', '12500', day(0)), rc('32', 'Sampath Bank', '8000', day(-3)), rc('33', 'Commercial Bank', '15000', day(9)), rc('34', 'Hatton National Bank', '4200', day(-12)), rc('35', "People's Bank", '2750', day(-20))] })
  const gc = (no: string, lockerId: number, amount: string, date: string) => ({ bank: '', lockerId, number: `${no}${n}`, amount, date, note: '' })
  await post(`/api/v1/admin/suppliers/${sup}/payment`, { amount: '14500', mode: 'pay', allocations: [], advance: '', oldDebt: '', method: 'cheque', reference: '', note: '', source: '',
    cheques: [gc('71', small, '9500', day(2)), gc('72', boc, '5000', day(-1))] })
  const rows = (await get('/api/v1/admin/cheques?status=&dir=in')) as { id: number; number: string }[]
  const id = (no: string) => rows.find((r) => r.number === `${no}${n}`)!.id
  await post(`/api/v1/admin/cheques/${id('34')}/clear`, { date: day(-10), lockerId: boc })
  await post(`/api/v1/admin/cheques/${id('35')}/return`, { status: 'bounced' })
  await post(`/api/v1/admin/cheques/${id('32')}/return`, { status: 'cancelled' })
  const given = (await get('/api/v1/admin/cheques?status=&dir=out')) as { id: number; number: string }[]
  await post(`/api/v1/admin/cheques/${given.find((r) => r.number === `72${n}`)!.id}/clear`, { date: day(0), lockerId: 0 })
}
