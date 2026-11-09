import { describe, expect, it } from 'vitest'
import { optsQS } from './ImportSheet'

describe('products sheet options', () => {
  it('adds the ref column only when asked, so a fixed sheet can come back to the same products', () => {
    expect(optsQS({ skipDisabled: true, extras: [], ref: false }, [])).toBe('')
    expect(optsQS({ skipDisabled: true, extras: [], ref: true }, [])).toBe('&ref=1')
    expect(optsQS({ skipDisabled: false, extras: ['moredetails'], ref: true }, [{ key: 'moredetails', label: 'More details' }])).toBe('&disabled=1&extras=moredetails&ref=1')
  })
})
