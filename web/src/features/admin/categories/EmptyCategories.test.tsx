import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { EmptyCategories } from './EmptyCategories'

const api = vi.fn()
vi.mock('@/api/client', async (orig) => ({ ...(await orig<object>()), api: (...a: unknown[]) => api(...a) }))
Object.defineProperty(window, 'matchMedia', { writable: true, value: (q: string) => ({ matches: false, media: q, onchange: null, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, dispatchEvent: () => false }) })

describe('Delete empty categories (CAT-1, 2)', () => {
  afterEach(() => { cleanup(); api.mockReset() })

  it('lists the empty ones ticked; an unticked one stays; only the ticked go', async () => {
    api.mockImplementation((path: string) => path === '/admin/categories/empty'
      ? Promise.resolve([{ id: 1, path: 'BLOOD SUGER' }, { id: 2, path: 'Medical Devices & First Aid > Plasters & Bandages' }, { id: 3, path: 'Wound' }])
      : Promise.resolve({ deleted: 2, kept: [] }))
    render(<QueryClientProvider client={new QueryClient()}><EmptyCategories onClose={() => {}} /></QueryClientProvider>)
    const plasters = (await screen.findAllByLabelText(/Plasters & Bandages/))[0] as HTMLInputElement
    expect(plasters.checked).toBe(true)
    fireEvent.click(plasters)
    fireEvent.click((await screen.findAllByRole('button', { name: 'Delete 2' }))[0])
    await waitFor(() => expect(api).toHaveBeenCalledWith('/admin/categories/delete-empty', { method: 'POST', json: { ids: [1, 3] } }))
  })
})
