import { expect, it } from 'vitest'
import { searchStr } from './search'

it('keeps a numeric barcode as text', () => {
  expect(searchStr(4790000000011)).toBe('4790000000011')
  expect(searchStr('milk')).toBe('milk')
  expect(searchStr(undefined)).toBe('')
  expect(searchStr({})).toBe('')
})
