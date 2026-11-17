import { expect, it } from 'vitest'
import { cn } from './utils'

it('the till type scale is a font size, so a text colour after it keeps it', () => {
  expect(cn('text-till-xs text-muted-foreground')).toBe('text-till-xs text-muted-foreground')
  expect(cn('text-till-xs', 'text-till-body')).toBe('text-till-body')
})
