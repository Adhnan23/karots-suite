import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { PinSheet } from './PinSheet'

describe('changing my PIN on the phone', () => {
  afterEach(() => { cleanup(); vi.unstubAllGlobals() })

  const fill = (cur: string, next: string, again: string) => {
    fireEvent.change(screen.getByLabelText('Current PIN'), { target: { value: cur } })
    fireEvent.change(screen.getByLabelText('New PIN'), { target: { value: next } })
    fireEvent.change(screen.getByLabelText('New PIN again'), { target: { value: again } })
    fireEvent.click(screen.getByRole('button', { name: 'Change PIN' }))
  }

  it('sends it to the PC, then says it changed', async () => {
    const fetch = vi.fn(async () => new Response(null, { status: 204 }))
    vi.stubGlobal('fetch', fetch)
    const onDone = vi.fn()
    render(<PinSheet live onDone={onDone} onClose={() => {}} />)
    fill('1234', '5678', '5678')
    await waitFor(() => expect(onDone).toHaveBeenCalled())
    expect(JSON.parse(String((fetch.mock.calls[0] as unknown[])[1] && ((fetch.mock.calls[0] as unknown[])[1] as RequestInit).body))).toEqual({ currentPin: '1234', newPin: '5678', confirmPin: '5678' })
  })

  it('needs the PC', () => {
    render(<PinSheet live={false} onDone={() => {}} onClose={() => {}} />)
    expect(screen.getByRole('alert').textContent).toMatch(/PC/)
  })
})
