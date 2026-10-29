import { fireEvent, render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useState } from 'react'
import { describe, expect, it } from 'vitest'
import { blankCheque, ChequeLines, chequeIssue, chequesReady, chequesTotal, toChequeLines, type ChequeDraftLine } from './ChequeLines'

function Harness({ out }: { out: (ls: ChequeDraftLine[]) => void }) {
  const [ls, setLs] = useState([blankCheque()])
  return <ChequeLines dir="in" lines={ls} onChange={(n) => { setLs(n); out(n) }} sym="Rs." want={1500} />
}

describe('ChequeLines', () => {
  it('adds lines, totals them and reports the wire shape', () => {
    let last: ChequeDraftLine[] = []
    const qc = new QueryClient({ defaultOptions: { queries: { enabled: false } } })
    render(<QueryClientProvider client={qc}><Harness out={(l) => (last = l)} /></QueryClientProvider>)
    fireEvent.change(screen.getByLabelText('Cheque 1 bank'), { target: { value: 'Bank of Ceylon' } })
    fireEvent.change(screen.getByLabelText('Cheque 1 number'), { target: { value: '100001' } })
    fireEvent.change(screen.getByLabelText('Cheque 1 amount'), { target: { value: '1,000' } })
    fireEvent.click(screen.getByText('Add a cheque'))
    fireEvent.change(screen.getByLabelText('Cheque 2 amount'), { target: { value: '500' } })
    expect(chequesTotal(last)).toBe(1500)
    expect(chequeIssue(last[1], 'in')).toBe('number')
    expect(chequesReady(last, 'in')).toBe(false)
    expect(toChequeLines(last)[0]).toEqual({ bank: 'Bank of Ceylon', lockerId: 0, number: '100001', amount: '1000', date: last[0].date, note: '' })
  })
  it('needs a bank place for a given cheque', () => {
    expect(chequeIssue({ ...blankCheque('100'), number: '1' }, 'out')).toBe('bank account')
    expect(chequeIssue({ ...blankCheque('100', '7'), number: '1' }, 'out')).toBe('')
  })
})
