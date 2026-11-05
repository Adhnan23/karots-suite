import { describe, expect, it } from 'vitest'
import type { RouteCtx } from '../fetch'
import { documentsRoutes, offlineQuote, type DocPart } from './documents'
import fixture from './documents.fixture.json'

const part: DocPart = {
  services: [{ id: 1, productId: 50, name: 'Photocopy', kind: 'metered', category: 'copy', offers: [], extras: [] }],
  prices: { 1: fixture.prices }, paper: { 1: fixture.paper }, recipes: { 50: fixture.recipe },
}
const ctx = (over: Partial<RouteCtx> = {}): RouteCtx => ({ parts: { documents: part }, copy: null, params: {}, query: new URLSearchParams(), body: undefined, ref: 'r1', out: [], userId: 7, ...over })
const route = (m: string, p: string) => documentsRoutes.find((r) => r.method === m && r.path === p)!
const num = (s: string) => Number(s)

describe('a job priced on the phone', () => {
  for (const c of fixture.cases) {
    it(`matches the PC: ${c.name}`, () => {
      const q = offlineQuote(part, 1, { size: c.size, material: c.material, color: c.color, double: c.double, copies: c.copies })
      if (!c.want) return expect(q).toBeNull()
      expect(q).toMatchObject({ unitPrice: c.want.unitPrice, total: c.want.total, minQty: c.want.minQty, sheets: c.want.sheets })
      expect(q!.components.map((x) => [x.productId, num(x.quantity)])).toEqual(c.want.components.map((x) => [x.productId, num(x.quantity)]))
    })
  }
})

describe('Print & Copy with the PC down', () => {
  it('answers the menu from the part', () => {
    expect(route('GET', '/x/documents/till').answer(ctx())).toEqual(part.services)
  })
  it('quote reads the query, and describes the job as the PC does', () => {
    const q = route('GET', '/x/documents/quote').answer(ctx({ query: new URLSearchParams('service=1&size=A4&material=&color=false&double=true&copies=3') }))
    expect(q).toMatchObject({ unitPrice: '14.00', total: '42.00', description: 'Photocopy A4 · B&W · 2-side', cost: '0.00' })
  })
  it('a plain job has no ink or sides in its description', () => {
    const plain = { ...part, services: [{ ...part.services[0], category: 'laminate' }] }
    const q = route('GET', '/x/documents/quote').answer(ctx({ parts: { documents: plain }, query: new URLSearchParams('service=1&size=A4&material=&color=false&double=false&copies=1') }))
    expect(q).toMatchObject({ description: 'Photocopy A4' })
  })
  it('recording jobs and shop use queues', () => {
    expect(route('POST', '/x/documents/jobs').queue).toBe(true)
    expect(route('POST', '/x/documents/own-use').queue).toBe(true)
  })
})
