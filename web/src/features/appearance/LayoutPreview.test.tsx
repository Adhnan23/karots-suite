import { fireEvent, render, screen } from '@testing-library/react'
import { expect, it, vi } from 'vitest'
import { Bar } from './LayoutPreview'

it('a cancelled drag forgets where it started, so the next move does not jump the bar', () => {
  const onDrag = vi.fn()
  render(<Bar vertical label="bar" onDrag={onDrag} />)
  const bar = screen.getByRole('separator', { name: 'bar' })
  bar.setPointerCapture = () => {}
  fireEvent.pointerDown(bar, { clientX: 100, pointerId: 1 })
  fireEvent.pointerCancel(bar, { pointerId: 1 })
  fireEvent.pointerMove(bar, { clientX: 400, pointerId: 1 })
  expect(onDrag).not.toHaveBeenCalled()
})

it('losing the pointer capture ends the drag too', () => {
  const onDrag = vi.fn()
  render(<Bar vertical label="bar2" onDrag={onDrag} />)
  const bar = screen.getByRole('separator', { name: 'bar2' })
  bar.setPointerCapture = () => {}
  fireEvent.pointerDown(bar, { clientX: 100, pointerId: 1 })
  fireEvent.lostPointerCapture(bar, { pointerId: 1 })
  fireEvent.pointerMove(bar, { clientX: 400, pointerId: 1 })
  expect(onDrag).not.toHaveBeenCalled()
})
