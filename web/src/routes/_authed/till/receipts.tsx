import { createFileRoute } from '@tanstack/react-router'
import { ReceiptsTab } from '@/features/counter/receipts/ReceiptsTab'

/** ?no= opens that slip (a scanned receipt number) in its list. */
export const Route = createFileRoute('/_authed/till/receipts')({
  validateSearch: (s: Record<string, unknown>): { no?: string } => (typeof s.no === 'string' && s.no ? { no: s.no } : {}),
  component: function Receipts() {
    const { no } = Route.useSearch()
    return <ReceiptsTab key={no} no={no} />
  },
})
