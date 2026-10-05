import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { CashflowSheet } from '@/features/money/cashflow/CashflowSheet'
import { cashflowPrintQuery, cashflowQuery } from '@/features/money/queries'
import { configQuery } from '@/features/till/api'
import { searchStr } from '@/lib/search'

type Search = { preset: string; from: string; to: string }

export const Route = createFileRoute('/_authed/print/cashflow')({
  validateSearch: (s: Record<string, unknown>): Search => ({ preset: searchStr(s.preset), from: searchStr(s.from), to: searchStr(s.to) }),
  loaderDeps: ({ search }) => search,
  loader: ({ context, deps }) => Promise.all([context.queryClient.ensureQueryData(configQuery), context.queryClient.ensureQueryData(cashflowQuery(deps)), context.queryClient.ensureQueryData(cashflowPrintQuery(deps))]),
  component: function CashflowPrint() {
    const r = Route.useSearch()
    const cfg = useSuspenseQuery(configQuery).data
    const d = useSuspenseQuery(cashflowQuery(r)).data
    const rows = useSuspenseQuery(cashflowPrintQuery(r)).data
    return <PrintFrame title="Cash flow"><CashflowSheet d={d} rows={rows} shop={cfg.shop} sym={cfg.symbol} /></PrintFrame>
  },
})
