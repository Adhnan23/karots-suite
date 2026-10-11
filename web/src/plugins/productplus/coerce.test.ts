import { describe, expect, it } from 'vitest'
import { coerce } from './coerce'

const f = (type: string, options: string[] = []) => ({ label: 'X', type, options })

describe('coerce', () => {
  it('reads yes/no words', () => {
    for (const w of ['yes', 'Y', 'true', '1']) expect(coerce(f('bool'), w)).toEqual({ value: '1' })
    for (const w of ['no', 'N', 'false', '0', '']) expect(coerce(f('bool'), w)).toEqual({ value: '' })
    expect(coerce(f('bool'), 'maybe')).toEqual({ error: 'X: yes or no' })
  })
  it('checks numbers and dates', () => {
    expect(coerce(f('number'), ' 12.5 ')).toEqual({ value: '12.5' })
    expect(coerce(f('number'), '1,5')).toEqual({ error: 'X: not a number' })
    expect(coerce(f('date'), '2026-02-28')).toEqual({ value: '2026-02-28' })
    expect(coerce(f('date'), '2026-13-01')).toEqual({ error: 'X: not a date (YYYY-MM-DD)' })
    expect(coerce(f('date'), '2026-02-30')).toEqual({ error: 'X: not a date (YYYY-MM-DD)' })
  })
  it('matches a dropdown option, keeps the current value', () => {
    expect(coerce(f('select', ['Small', 'Large']), 'small')).toEqual({ value: 'Small' })
    expect(coerce(f('select', ['Small']), 'XL')).toEqual({ error: 'X: XL is not an option' })
    expect(coerce(f('select', ['Small']), 'Old', 'Old')).toEqual({ value: 'Old' })
    expect(coerce(f('text'), '')).toEqual({ value: '' })
  })
})
