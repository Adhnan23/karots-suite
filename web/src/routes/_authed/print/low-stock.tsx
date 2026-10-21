import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { LowStockPaper } from '@/features/purchasing/reorder/LowStockPaper'
import { reorderQuery } from '@/features/purchasing/queries'
import { configQuery } from '@/features/till/api'
import { searchStr } from '@/lib/search'

type Search = { q: string; category: number; supplier: number; page: number; hideCovered: boolean; all: boolean; till: boolean }

/** Low stock on A4 (karots-pos /admin/reports/low-stock/print): the Reorder
 * list's filters, this page or everything (?all=1). */
export const Route = createFileRoute('/_authed/print/low-stock')({
  validateSearch: (s: Record<string, unknown>): Search => ({
    q: searchStr(s.q), category: Number(s.category) || 0, supplier: Number(s.supplier) || 0, page: Math.max(1, Number(s.page) || 1),
    hideCovered: s.hideCovered === true || s.hideCovered === 'true', all: s.all === 1 || s.all === '1',
    till: s.till === true || s.till === 'true', // printed from the till's Low stock tab
  }),
  loaderDeps: ({ search }) => search,
  loader: ({ context, deps }) => Promise.all([context.queryClient.ensureQueryData(configQuery), context.queryClient.ensureQueryData(reorderQuery(deps))]),
  component: function LowStockPrint() {
    const s = Route.useSearch()
    const cfg = useSuspenseQuery(configQuery).data
    const d = useSuspenseQuery(reorderQuery(s)).data
    return <PrintFrame title="Low stock"><LowStockPaper d={d} shop={cfg.shop} sym={cfg.symbol} all={s.all} /></PrintFrame>
  },
})
