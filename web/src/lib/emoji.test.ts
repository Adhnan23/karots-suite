import { describe, expect, it } from 'vitest'
import { searchEmoji } from './emoji'

describe('searchEmoji', () => {
  it('matches word starts', () => {
    expect(searchEmoji('milk')).toContain('🥛')
    expect(searchEmoji('mil')).toContain('🥛')
    expect(searchEmoji('soap')).toEqual(['🧼'])
  })
  it('every word must match', () => expect(searchEmoji('milk tea')).toEqual(['🧋']))
  it('searches by aisle too', () => expect(searchEmoji('drinks')).toContain('🥤'))
})
