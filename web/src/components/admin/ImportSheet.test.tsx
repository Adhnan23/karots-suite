import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { ImportSheet } from './ImportSheet'

const upload = vi.fn()
vi.mock('@/api/client', async (orig) => ({ ...(await orig<object>()), apiUpload: (...a: unknown[]) => upload(...a) }))

const preview = {
  created: 1, updated: 2, skipped: 1, applied: false, notes: [],
  errors: [{ line: 5, message: 'a ref from another shop' }],
  changes: [
    { line: 2, product: 'Glucomet 500mg', new: false, fields: [{ field: 'name', from: 'GLUCOMET 500', to: 'Glucomet 500mg' }, { field: 'category', from: 'BLOOD SUGER', to: 'Medicines > Diabetes > Metformin' }] },
    { line: 3, product: 'Azee 500mg', new: false, fields: [{ field: 'Generic Name', from: '', to: 'Azithromycin' }] },
    { line: 4, product: 'Bengray cream 28g', new: true, fields: [] },
  ],
}

// The side sheet asks the screen's width; jsdom has no matchMedia.
Object.defineProperty(window, 'matchMedia', { writable: true, value: (q: string) => ({ matches: false, media: q, onchange: null, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {}, dispatchEvent: () => false }) })

function renderSheet() {
  render(<QueryClientProvider client={new QueryClient()}><ImportSheet entity="products" qs="&extras=productplus" included={[]} onClose={() => {}} /></QueryClientProvider>)
}

describe('products import: preview first (IMP-1, 2, 4)', () => {
  afterEach(() => { cleanup(); upload.mockReset() })

  it('choosing a sheet previews it; Import sends the previewed copy', async () => {
    upload.mockResolvedValueOnce(preview).mockResolvedValueOnce({ ...preview, changes: preview.changes, applied: true })
    renderSheet()
    const picked = new File(['name\nGlucomet 500mg\n'], 'fix.csv', { type: 'text/csv' })
    fireEvent.change(document.querySelector('input[type=file]')!, { target: { files: [picked] } })

    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1))
    expect(upload.mock.calls[0][0]).toBe('/admin/products/import?dry=1&extras=productplus')
    expect((await screen.findAllByText('2 to update')).length).toBeGreaterThan(0)
    expect(screen.getAllByText('1 new').length).toBeGreaterThan(0)
    expect(screen.getAllByText('1 skipped').length).toBeGreaterThan(0)
    expect(screen.getAllByText('BLOOD SUGER').length).toBeGreaterThan(0)
    expect(screen.getAllByText('Medicines > Diabetes > Metformin').length).toBeGreaterThan(0)
    expect(screen.getAllByText(/a ref from another shop/).length).toBeGreaterThan(0)

    for (const f of screen.getAllByPlaceholderText(/Find a product/)) fireEvent.change(f, { target: { value: 'azee' } })
    expect(screen.queryByText('BLOOD SUGER')).toBeNull()
    expect(screen.getAllByText('Azithromycin').length).toBeGreaterThan(0)

    fireEvent.click((await screen.findAllByRole('button', { name: 'Import' }))[0])
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(2))
    expect(upload.mock.calls[1][0]).toBe('/admin/products/import?extras=productplus')
    const sent = upload.mock.calls[1][2] as File
    expect(sent).not.toBe(picked)
    expect(await sent.text()).toBe('name\nGlucomet 500mg\n')
    expect((await screen.findAllByLabelText('Import result')).length).toBeGreaterThan(0)
  })

  it('customers import as before: no preview', async () => {
    upload.mockResolvedValueOnce({ created: 1, updated: 0, skipped: 0, errors: [], notes: [], applied: true })
    render(<QueryClientProvider client={new QueryClient()}><ImportSheet entity="customers" qs="" included={[]} onClose={() => {}} /></QueryClientProvider>)
    fireEvent.change(document.querySelector('input[type=file]')!, { target: { files: [new File(['name\nA\n'], 'c.csv')] } })
    expect(upload).not.toHaveBeenCalled()
    fireEvent.click((await screen.findAllByRole('button', { name: 'Import' }))[0])
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1))
    expect(upload.mock.calls[0][0]).toBe('/admin/customers/import')
  })
})
