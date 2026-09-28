import { describe, expect, it } from 'vitest'
import { parseEntry } from './entry'

describe('parseEntry', () => {
  it('reads the phone quick-sell handoff KQ<barcode>X<qty>', () => {
    expect(parseEntry('KQ4790001X12')).toEqual({ kind: 'quick', barcode: '4790001', qty: 12 })
    expect(parseEntry('kq4790001x3')).toEqual({ kind: 'quick', barcode: '4790001', qty: 3 })
  })
  it('digits probe the barcode first; anything else is a search', () => {
    expect(parseEntry(' 4790000000011 ')).toEqual({ kind: 'barcode', code: '4790000000011' })
    expect(parseEntry('milk')).toEqual({ kind: 'search', q: 'milk' })
    expect(parseEntry('SKU-12')).toEqual({ kind: 'search', q: 'SKU-12' })
    expect(parseEntry('   ')).toEqual({ kind: 'empty' })
  })
})
