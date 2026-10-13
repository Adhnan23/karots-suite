import { describe, expect, it } from 'vitest'
import { hasBarcode, scannedLabel } from './barcode'

describe('hasBarcode', () => {
  const p = { barcode: '4791234', barcodes: ['8901111', '8902222'] }
  it('matches the main barcode', () => expect(hasBarcode(p, '4791234')).toBe(true))
  it('matches an extra barcode', () => expect(hasBarcode(p, '8902222')).toBe(true))
  it('ignores surrounding spaces from a scanner', () => expect(hasBarcode(p, ' 8901111 ')).toBe(true))
  it('does not match a different code or an empty one', () => {
    expect(hasBarcode(p, '0000')).toBe(false)
    expect(hasBarcode({ barcode: '', barcodes: [] }, '')).toBe(false)
  })
  it('works for rows without extras', () => expect(hasBarcode({ barcode: '4791234' }, '4791234')).toBe(true))
})

describe('scannedLabel', () => {
  const now = new Date(2026, 9, 13, 15, 0)
  it('says never for a code not scanned since it was added', () => expect(scannedLabel('', now)).toBe('never scanned'))
  it('counts calendar days, then months', () => {
    expect(scannedLabel(new Date(2026, 9, 13, 8, 0).toISOString(), now)).toBe('scanned today')
    expect(scannedLabel(new Date(2026, 9, 12, 23, 0).toISOString(), now)).toBe('scanned yesterday')
    expect(scannedLabel(new Date(2026, 9, 1).toISOString(), now)).toBe('scanned 12 days ago')
    expect(scannedLabel(new Date(2026, 5, 10).toISOString(), now)).toBe('scanned 4 months ago')
  })
})
