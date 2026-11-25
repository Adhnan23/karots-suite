import { describe, expect, it } from 'vitest'
import { RouteError, type RouteCtx } from '../fetch'
import { catalogueRoutes } from './catalogue'

const ctx: RouteCtx = { parts: {}, copy: null, params: {}, query: new URLSearchParams(), body: undefined, ref: 'r1', out: [], userId: 7 }

// CTL-4: the copy is on the main PC only; a phone off the shop network types by hand, told so at once.
describe('the catalogue with the PC down', () => {
  it('says it can’t be reached, for the status and a search', () => {
    for (const path of ['/x/catalogue/status', '/x/catalogue/search']) {
      const r = catalogueRoutes.find((x) => x.method === 'GET' && x.path === path)!
      expect(r.queue).toBeFalsy()
      expect(() => r.answer(ctx)).toThrow(RouteError)
    }
  })
})
