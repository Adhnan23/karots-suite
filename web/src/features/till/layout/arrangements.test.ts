import { describe, expect, it } from 'vitest'
import type { SellLayout } from '@/api/types.gen'
import { ARRANGEMENTS, gridOf, pickLayout, sizeKey, todayLayout } from './arrangements'

const L = (arr: string, fr: number[], split = 0): SellLayout => ({ arr, fr, split, card: 'L', text: 100 })

describe('pickLayout', () => {
  const saved = { '1920x1080': L('CPY', [0.3, 0.45, 0.25]), '1366x768': L('P_CY', [0.6, 0.4], 0.5) }
  it('takes the nearest saved size within 10%', () => {
    expect(pickLayout(saved, 1900, 1050)).toBe(saved['1920x1080'])
    expect(pickLayout(saved, 1366, 740)).toBe(saved['1366x768'])
  })
  it('nothing near enough, or nothing saved: today', () => {
    expect(pickLayout(saved, 1600, 900)).toBeNull()
    expect(pickLayout({}, 1366, 768)).toBeNull()
  })
})

describe('gridOf', () => {
  it('a stacked column spans both rows of the other', () => {
    expect(gridOf(L('PY_C', [0.6, 0.4], 0.55))).toEqual({
      gridTemplateColumns: 'minmax(0,0.6fr) minmax(0,0.4fr)',
      gridTemplateRows: 'minmax(0,0.55fr) minmax(0,0.45fr)',
      gridTemplateAreas: "'cat cart' 'pay cart'",
    })
  })
  it('three columns are one row', () => {
    expect(gridOf(L('CPY', [0.3, 0.45, 0.25]))).toEqual({
      gridTemplateColumns: 'minmax(0,0.3fr) minmax(0,0.45fr) minmax(0,0.25fr)',
      gridTemplateRows: 'minmax(0,1fr)',
      gridTemplateAreas: "'cart cat pay'",
    })
  })
  it('an unknown arrangement or the wrong share count: null (today)', () => {
    expect(gridOf(L('ZZZ', [1]))).toBeNull()
    expect(gridOf(L('PCY', [0.5, 0.5]))).toBeNull()
  })
})

describe('todayLayout', () => {
  it('wide: three columns with the cart and pay at their clamped widths', () => {
    const t = todayLayout(1366)
    expect(t.arr).toBe('PCY')
    expect(t.fr.reduce((a, b) => a + b)).toBeCloseTo(1)
    expect(t.fr[1] * (1366 - 56)).toBeCloseTo(392)
    expect(t.fr[2] * (1366 - 56)).toBeCloseTo(356)
  })
  it('narrow: cart over pay, 2 to 3', () => {
    const t = todayLayout(1100)
    expect(t.arr).toBe('P_CY')
    expect(t.split).toBeCloseTo(0.4)
  })
})

it('every arrangement has its three sections once', () => {
  expect(ARRANGEMENTS).toHaveLength(10)
  for (const a of ARRANGEMENTS) expect(a.cols.flat().sort()).toEqual(['cart', 'cat', 'pay'])
  expect(sizeKey(1366.4, 767.6)).toBe('1366x768')
})

it('a text size other than 90, 100 or 110 is Normal (never 0 px fonts)', async () => {
  const { textOf } = await import('./arrangements')
  expect(textOf({ ...L('PCY', [0.4, 0.3, 0.3]), text: 90 })).toBe(90)
  expect(textOf({ ...L('PCY', [0.4, 0.3, 0.3]), text: 0 })).toBe(100)
  expect(textOf({ ...L('PCY', [0.4, 0.3, 0.3]), text: 150 })).toBe(100)
  expect(textOf(null)).toBe(100)
})
