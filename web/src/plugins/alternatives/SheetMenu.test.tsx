import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { SheetMenu } from './SheetMenu'

const upload = vi.fn()
vi.mock('@/api/client', async (orig) => ({ ...(await orig<object>()), apiUpload: (...a: unknown[]) => upload(...a) }))
const toast = { success: vi.fn(), warning: vi.fn(), error: vi.fn() }
vi.mock('sonner', () => ({ toast: { success: (...a: unknown[]) => toast.success(...a), warning: (...a: unknown[]) => toast.warning(...a), error: (...a: unknown[]) => toast.error(...a) } }))

const result = (over = {}) => ({ groups: 2, members: 5, groupsAdded: 1, groupsRemoved: 1, placed: 2, moved: 1, takenOut: 1, removed: ['Cable 1m'], skipped: [], applied: false, ...over })

function show(role: 'admin' | 'manager') {
  const qc = new QueryClient()
  qc.setQueryData(['me'], { user: { role } })
  render(<QueryClientProvider client={qc}><SheetMenu /></QueryClientProvider>)
}

describe('the Alternatives sheet', () => {
  afterEach(() => { cleanup(); upload.mockReset(); Object.values(toast).forEach((f) => f.mockReset()) })

  it('ALT-11: Replace sends the previewed bytes, lists removed groups, and warns of new skips', async () => {
    upload.mockResolvedValueOnce(result()).mockResolvedValueOnce(result({ applied: true, skipped: ['line 3: no product with this barcode or name'] }))
    show('admin')
    const picked = new File(['Group,Product\nRice,Anchor\n'], 'alternatives.csv', { type: 'text/csv' })
    fireEvent.change(document.querySelector('input[data-sheet="alternatives"]')!, { target: { files: [picked] } })
    expect(await screen.findByText('Cable 1m')).toBeTruthy()
    fireEvent.click(screen.getByRole('button', { name: 'Replace' }))
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(2))
    expect(upload.mock.calls[0][0]).toBe('/x/alternatives/sheet/import?dry=1')
    const sent = upload.mock.calls[1][2] as File
    expect(sent).not.toBe(picked)
    expect(await sent.text()).toBe('Group,Product\nRice,Anchor\n')
    await waitFor(() => expect(toast.warning).toHaveBeenCalled())
    expect(String(toast.success.mock.calls[0][0])).toBe('Alternatives replaced: 2 groups, 5 products')
  })

  it('ALT-16: a manager can export but has no import', () => {
    show('manager')
    expect(document.querySelector('input[data-sheet="alternatives"]')).toBeNull()
    expect(screen.getByRole('button', { name: /sheet/i })).toBeTruthy()
  })
})
