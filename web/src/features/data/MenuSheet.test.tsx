import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { MenuSheet } from './MenuSheet'

const upload = vi.fn()
vi.mock('@/api/client', async (orig) => ({ ...(await orig<object>()), apiUpload: (...a: unknown[]) => upload(...a) }))
const toast = { success: vi.fn(), warning: vi.fn(), error: vi.fn() }
vi.mock('sonner', () => ({ toast: { success: (...a: unknown[]) => toast.success(...a), warning: (...a: unknown[]) => toast.warning(...a), error: (...a: unknown[]) => toast.error(...a) } }))

const result = (over = {}) => ({ groups: 1, items: 0, removedGroups: 0, removedItems: 0, skipped: [], applied: false, ...over })

describe('the cashier menu sheet (MENU-3)', () => {
  afterEach(() => { cleanup(); upload.mockReset(); Object.values(toast).forEach((f) => f.mockReset()) })

  it('Replace sends the bytes that were previewed, and says when Apply skipped lines the preview did not', async () => {
    upload.mockResolvedValueOnce(result()).mockResolvedValueOnce(result({ applied: true, skipped: ['line 3: two products have this name'] }))
    render(<QueryClientProvider client={new QueryClient()}><MenuSheet /></QueryClientProvider>)
    const picked = new File(['Group\nDrinks\n'], 'menu.csv', { type: 'text/csv' })
    fireEvent.change(document.querySelector('input[data-sheet="menu"]')!, { target: { files: [picked] } })
    fireEvent.click(await screen.findByRole('button', { name: 'Replace the menu' }))
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(2))
    const sent = upload.mock.calls[1][2] as File
    expect(sent).not.toBe(picked) // a copy taken at preview time: editing the file on disk afterwards changes nothing
    expect(await sent.text()).toBe('Group\nDrinks\n')
    await waitFor(() => expect(toast.warning).toHaveBeenCalled())
    expect(String(toast.warning.mock.calls[0][0])).toMatch(/1 line skipped/)
  })
})
