import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { StatementSheet } from '@/features/people/StatementSheet'
import { statementQuery } from '@/features/purchasing/queries'
import { configQuery } from '@/features/till/api'

const kindOf = (k: string) => (k === 'customer' ? 'customer' : 'supplier')

export const Route = createFileRoute('/_authed/print/statement/$kind/$id')({
  loader: ({ context, params }) =>
    Promise.all([context.queryClient.ensureQueryData(configQuery), context.queryClient.ensureQueryData(statementQuery(kindOf(params.kind), Number(params.id)))]),
  component: function StatementPrint() {
    const { kind, id } = Route.useParams()
    const cfg = useSuspenseQuery(configQuery).data
    const st = useSuspenseQuery(statementQuery(kindOf(kind), Number(id))).data
    return (
      <PrintFrame title={`Statement · ${st.name}`}>
        <StatementSheet st={st} shop={cfg.shop} sym={cfg.symbol} kind={kindOf(kind)} />
      </PrintFrame>
    )
  },
})
