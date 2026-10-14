import { describe, expect, it } from 'vitest'
import { doseLabel } from './label'

const on = new Date('2026-10-14T10:00:00+05:30')

describe('doseLabel', () => {
  it('with a patient: name and date, medicine, dose in words, days and instruction', () => {
    expect(doseLabel({ patient: 'Nimal Perera', drug: 'Metfor 500mg', qty: 60, dose: '1-0-1', words: 'Morning · Night', perDay: 2, note: 'After meals', on })).toEqual([
      'Nimal Perera  14 Oct', 'Metfor 500mg × 60', '1-0-1  Morning · Night', '30 days · After meals',
    ])
  })
  it('without a patient: medicine and date first', () => {
    expect(doseLabel({ drug: 'Panadol 500mg', qty: 6, dose: '1-1-1', words: 'Morning · Noon · Night', perDay: 3, note: '', on })).toEqual([
      'Panadol 500mg × 6  14 Oct', '1-1-1  Morning · Noon · Night', '2 days',
    ])
  })
  it('a dose not named yet prints as written; when-needed has no days', () => {
    expect(doseLabel({ drug: 'Salbutamol', qty: 1, dose: 'sos', words: '', perDay: 0, note: 'Shake well', on })).toEqual([
      'Salbutamol × 1  14 Oct', 'sos', 'Shake well',
    ])
  })
})
