import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { CountSheetPaper } from '@/features/reports/stock/CountSheetPaper'
import { countSheetQuery } from '@/features/reports/queries'
import { configQuery } from '@/features/till/api'
import { searchStr } from '@/lib/search'

type Search = { category: string; supplier: string }

export const Route = createFileRoute('/_authed/print/count-sheet')({
  validateSearch: (s: Record<string, unknown>): Search => ({ category: searchStr(s.category), supplier: searchStr(s.supplier) }),
  loaderDeps: ({ search }) => search,
  loader: ({ context, deps }) => Promise.all([context.queryClient.ensureQueryData(configQuery), context.queryClient.ensureQueryData(countSheetQuery(deps))]),
  component: function CountSheetPrint() {
    const s = Route.useSearch()
    const cfg = useSuspenseQuery(configQuery).data
    const d = useSuspenseQuery(countSheetQuery(s)).data
    return <PrintFrame title="Stock count sheet"><CountSheetPaper d={d} shop={cfg.shop} /></PrintFrame>
  },
})
