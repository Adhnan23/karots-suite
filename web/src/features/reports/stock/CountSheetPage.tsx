import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import { ClipboardCheck, Printer } from 'lucide-react'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { suppliersQuery } from '@/features/counter/queries'
import { qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { ReportPage } from '../kit/ReportPage'
import { reportQS } from '../kit/search'
import { countSheetQuery } from '../queries'

const route = getRouteApi('/_authed/admin/reports/count-sheet')

/** Stock count sheet (karots-pos /admin/reports/count-sheet): the products to
 * count on a shelf, by category and / or supplier — print it, count, then key
 * the counts into Stock-take. */
export function CountSheetPage() {
  const s = route.useSearch()
  const { me } = route.useRouteContext()
  const f = { category: s.category ? String(s.category) : '', supplier: s.supplier ? String(s.supplier) : '' }
  const navigate = route.useNavigate()
  const set = (p: Partial<typeof s>) => void navigate({ search: (x) => ({ ...x, ...p }), replace: true })
  const sups = useQuery(suppliersQuery('')).data ?? []
  const d = useQuery({ ...countSheetQuery(f), placeholderData: keepPreviousData }).data
  return (
    <ReportPage title="Stock count sheet" sub="What to count on the shelf, grouped by category. Print it, count, then enter the counts."
      filters={(
        <>
          <div className="md:w-72"><CategoryPicker label="Category" noneLabel="Every category" value={Number(f.category) || 0} onChange={(id) => set({ category: id })} /></div>
          <select aria-label="Supplier" value={f.supplier} onChange={(e) => set({ supplier: Number(e.target.value) })} className={cn(INPUT, 'appearance-auto md:w-60')}>
            <option value="">Every supplier</option>
            {sups.map((x) => <option key={x.id} value={x.id}>{x.name}</option>)}
          </select>
          <span className="flex gap-2 md:ml-auto">
            <Button variant="outline" className="h-10 gap-1.5 px-4" asChild><a href={`/print/count-sheet?${reportQS({ preset: '', ...f })}`} target="_blank" rel="noreferrer"><Printer size={16} /> A4 sheet</a></Button>
            {me.stockTakeEnabled && <Button className="h-10 gap-1.5 px-4" asChild><Link to="/admin/stock/take"><ClipboardCheck size={16} /> Enter counts</Link></Button>}
          </span>
        </>
      )}>
      <DataList label="Products to count" rows={d?.rows ?? []} rowKey={(x) => x.id}
        empty={<EmptyState title={d ? 'No stocked products match' : 'Loading…'} />}
        columns={[
          { key: 'name', header: 'Product', cell: (x) => <span className="flex flex-col"><b>{x.name}</b><small className="font-normal text-faint">{x.category}{x.barcode && ` · ${x.barcode}`}</small></span> },
          { key: 'qty', header: 'On hand', num: true, cell: (x) => `${qtyText(x.onHand)} ${x.unit}` },
        ]} />
    </ReportPage>
  )
}
