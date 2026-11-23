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
  render(<QueryClientProvider client={new QueryClient()}><ImportSheet entity="products" qs="&extras=fields" included={[]} onClose={() => {}} /></QueryClientProvider>)
}

describe('products import: preview first (IMP-1, 2, 4)', () => {
  afterEach(() => { cleanup(); upload.mockReset() })

  it('choosing a sheet previews it; Import sends the previewed copy', async () => {
    upload.mockResolvedValueOnce(preview).mockResolvedValueOnce({ ...preview, changes: preview.changes, applied: true })
    renderSheet()
    const picked = new File(['name\nGlucomet 500mg\n'], 'fix.csv', { type: 'text/csv' })
    fireEvent.change(document.querySelector('input[type=file]')!, { target: { files: [picked] } })

    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1))
    expect(upload.mock.calls[0][0]).toBe('/admin/products/import?dry=1&extras=fields')
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
    expect(upload.mock.calls[1][0]).toBe('/admin/products/import?extras=fields')
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

  it('the preview shows the import notes (a refused cell, a barcode in use)', async () => {
    upload.mockResolvedValueOnce({ ...preview, notes: ['Line 3: Custom fields not saved: Strength must be a number'] })
    renderSheet()
    fireEvent.change(document.querySelector('input[type=file]')!, { target: { files: [new File(['x'], 'a.csv')] } })
    expect((await screen.findAllByText(/Strength must be a number/)).length).toBeGreaterThan(0)
  })

  it('an older preview never replaces a newer one; Import sends the newer file', async () => {
    let first!: (v: unknown) => void
    upload.mockReturnValueOnce(new Promise((r) => { first = r }))
      .mockResolvedValueOnce({ ...preview, changes: [{ line: 2, product: 'Newer sheet', new: false, fields: [{ field: 'name', from: 'a', to: 'Newer sheet' }] }] })
      .mockResolvedValueOnce({ ...preview, applied: true })
    renderSheet()
    const input = document.querySelector('input[type=file]')!
    fireEvent.change(input, { target: { files: [new File(['old'], 'old.csv')] } })
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1))
    fireEvent.change(input, { target: { files: [new File(['new'], 'new.csv')] } })
    expect((await screen.findAllByText('Newer sheet')).length).toBeGreaterThan(0)
    first({ ...preview, changes: [{ line: 2, product: 'Older sheet', new: false, fields: [{ field: 'name', from: 'a', to: 'Older sheet' }] }] })
    await new Promise((r) => setTimeout(r, 20))
    expect(screen.queryAllByText('Older sheet')).toHaveLength(0)
    fireEvent.click((await screen.findAllByRole('button', { name: 'Import' }))[0])
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(3))
    expect(await (upload.mock.calls[2][2] as File).text()).toBe('new')
  })

  it('the button says Choose a file until a preview is there', async () => {
    upload.mockReturnValueOnce(new Promise(() => {}))
    renderSheet()
    fireEvent.change(document.querySelector('input[type=file]')!, { target: { files: [new File(['x'], 'a.csv')] } })
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1))
    expect(screen.queryAllByRole('button', { name: 'Import' })).toHaveLength(0)
  })

  it('Empty cells clear the field: ticking it previews again with clear=1, and Import sends it too (IMP-6)', async () => {
    upload.mockResolvedValue(preview)
    renderSheet()
    fireEvent.change(document.querySelector('input[type=file]')!, { target: { files: [new File(['x'], 'a.csv')] } })
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(1))
    fireEvent.click(screen.getAllByLabelText(/Empty cells clear/)[0])
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(2))
    expect(upload.mock.calls[1][0]).toBe('/admin/products/import?dry=1&clear=1&extras=fields')
    fireEvent.click((await screen.findAllByRole('button', { name: 'Import' }))[0])
    await waitFor(() => expect(upload).toHaveBeenCalledTimes(3))
    expect(upload.mock.calls[2][0]).toBe('/admin/products/import?clear=1&extras=fields')
  })
})

