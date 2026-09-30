import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { PoSheet } from '@/features/purchasing/purchases/PurchaseDocs'
import { purchasesPrintQuery } from '@/features/purchasing/queries'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'

type Search = { ids: string; mode?: 'separate' }

export const Route = createFileRoute('/_authed/print/po')({
  validateSearch: (s: Record<string, unknown>): Search => ({ ids: String(s.ids ?? '').replace(/[^\d,]/g, ''), mode: s.mode === 'separate' ? 'separate' : undefined }),
  loaderDeps: ({ search }) => ({ ids: search.ids }),
  loader: ({ context, deps }) => Promise.all([context.queryClient.ensureQueryData(configQuery), context.queryClient.ensureQueryData(purchasesPrintQuery(deps.ids))]),
  component: function PoPrint() {
    const { ids, mode } = Route.useSearch()
    const navigate = Route.useNavigate()
    const cfg = useSuspenseQuery(configQuery).data
    const pos = useSuspenseQuery(purchasesPrintQuery(ids)).data
    return (
      <PrintFrame title={pos.length === 1 ? `Purchase order #${pos[0].id}` : `${pos.length} purchase orders`}>
        {pos.length > 1 && (
          <div className="mx-auto mb-3 max-w-[760px] px-4 print:hidden">
            <Seg label="Pages" value={mode ?? 'together'} onChange={(m) => void navigate({ search: { ids, mode: m === 'separate' ? 'separate' : undefined } })}
              options={[['together', 'All together'], ['separate', 'One page per supplier']]} />
          </div>
        )}
        <div className="flex flex-col gap-6 print:gap-0">
          {pos.map((p) => <div key={p.id} className={mode === 'separate' ? 'break-after-page' : 'print:mb-8'}><PoSheet p={p} shop={cfg.shop} sym={cfg.symbol} /></div>)}
        </div>
      </PrintFrame>
    )
  },
})
