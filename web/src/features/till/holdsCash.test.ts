import { describe, expect, it } from 'vitest'
import { holdsCash } from './api'

// A till's cash can be banked into a safe, a pocket or another cash place — never a
// bank or card account, and never Cheques in hand, which holds only uncleared cheques
// (the close picker offered it first, and the server then refused the close).
describe('holdsCash', () => {
  it('takes cash places only', () => {
    expect(['safe', 'pocket', 'other'].map((kind) => holdsCash({ kind }))).toEqual([true, true, true])
    expect(['bank', 'card', 'cheque'].map((kind) => holdsCash({ kind }))).toEqual([false, false, false])
  })
})
