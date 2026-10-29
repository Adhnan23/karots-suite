import { keepPreviousData, useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link, useNavigate } from '@tanstack/react-router'
import { Undo2 } from 'lucide-react'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { suppliersQuery } from '@/features/counter/queries'
import { configQuery } from '@/features/till/api'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { day } from '../purchases/PurchasesPage'
import { purchaseReturnsQuery } from '../queries'

const route = getRouteApi('/_authed/admin/purchase-returns/')

/** Purchase returns: stock sent back to
 * suppliers, each a debit note that credits what we owe them. */
export function ReturnsPage() {
  const s = route.useSearch()
  const supplier = s.supplier ?? 0
  const page = s.page ?? 1
  const navigate = route.useNavigate()
  const go = useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const sups = useQuery(suppliersQuery('')).data ?? []
  const data = useQuery({ ...purchaseReturnsQuery(supplier, page), placeholderData: keepPreviousData }).data
  return (
    <>
      <PageHead title="Purchase returns" sub="Stock sent back to suppliers. Each one comes off what you owe them."
        actions={<Button className="h-10 gap-1.5 px-4 font-bold" asChild><Link to="/admin/purchase-returns/new" search={supplier ? { supplier } : {}}><Undo2 size={16} /> Return goods</Link></Button>} />
      <select aria-label="Supplier" value={supplier} onChange={(e) => void navigate({ search: { supplier: Number(e.target.value), page: 1 } })} className={cn(INPUT, 'mb-3 appearance-auto md:w-72')}>
        <option value={0}>All suppliers</option>
        {sups.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
      </select>
      <DataList label="Purchase returns" rows={data?.items ?? []} rowKey={(r) => r.id} onRow={(r) => void go({ to: '/admin/purchase-returns/$id', params: { id: String(r.id) } })}
        empty={<EmptyState title={!data ? 'Loading…' : 'No returns yet'}>{data && 'When goods go back — damaged, wrong or expired — record them here so the supplier owes you for them.'}</EmptyState>}
        footer={data && <Pager page={data.page} pageSize={data.pageSize} total={data.total} onPage={(p) => void navigate({ search: (x) => ({ ...x, page: p }) })} />}
        columns={[
          { key: 'who', header: 'Returned to', cell: (r) => (
            <span className="flex flex-col gap-0.5"><b>{r.supplierName}</b><small className="font-normal text-faint">{r.reference || `#${r.id}`} · {day(r.date)}{r.invoiceNo && ` · from ${r.invoiceNo}`}{r.reason && ` · ${r.reason}`}</small></span>
          ) },
          { key: 'total', header: 'Value', num: true, cell: (r) => <b className="text-success">↓ {money(sym, r.total)}</b> },
        ]} />
    </>
  )
}
