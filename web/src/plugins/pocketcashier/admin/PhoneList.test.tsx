import { cleanup, render, screen, within } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { PhoneList } from './PhoneList'
import type { PhoneRow } from './types'

const now = Date.parse('2026-11-01T10:00:00Z')
const row = (p: Partial<PhoneRow>): PhoneRow => ({ id: 1, name: 'Counter phone', status: 'active', userName: '', copyAt: '', lastSeen: '', pairCode: '', ...p })
const noop = () => {}
const list = (rows: PhoneRow[]) => render(<PhoneList rows={rows} now={now} onApprove={noop} onBlock={noop} onRelease={noop} onRemove={noop} />)

describe('PhoneList', () => {
  afterEach(cleanup)

  it('shows a person on a phone not heard from as not reachable, with Release', () => {
    list([row({ userName: 'Nimal', lastSeen: '2026-11-01T09:57:00Z' })])
    const card = screen.getByRole('listitem')
    expect(within(card).getByText('Nimal · not reachable')).toBeTruthy()
    expect(within(card).getByRole('button', { name: 'Release' })).toBeTruthy()
  })

  it('shows a blocked phone as Blocked with Remove and no Unblock', () => {
    list([row({ status: 'blocked' })])
    const card = screen.getByRole('listitem')
    expect(within(card).getByText('Blocked')).toBeTruthy()
    expect(within(card).getByRole('button', { name: 'Remove' })).toBeTruthy()
    expect(within(card).queryByRole('button', { name: /unblock/i })).toBeNull()
    expect(within(card).queryByRole('button', { name: 'Block' })).toBeNull()
  })

  it('offers Approve on a waiting phone, with its code', () => {
    list([row({ status: 'pending', pairCode: '482913' })])
    const card = screen.getByRole('listitem')
    expect(within(card).getByText('482913')).toBeTruthy()
    expect(within(card).getByRole('button', { name: 'Approve' })).toBeTruthy()
  })
})
