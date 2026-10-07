import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { Tag } from '@/features/admin/products/ProductsPage'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { AgingBar } from '../kit/AgingBar'
import { BarCell } from '../kit/BarCell'
import { Kpi, KpiStrip } from '../kit/Kpi'
import { ReportPage } from '../kit/ReportPage'
import { duesQuery } from '../queries'

const TEXT = {
  customer: {
    title: 'Customer dues', sub: 'Who owes you, how much, and for how long. Advances aren’t dues.', who: 'Customers owing', flag: 'Over their limit', tag: 'over limit',
    terms: 'Limit', empty: 'Nobody owes you anything', to: '/admin/customers/$id' as const,
  },
  supplier: {
    title: 'Supplier dues', sub: 'Who you owe, how much, and what is past its credit days.', who: 'Suppliers owed', flag: 'Overdue', tag: 'overdue',
    terms: 'Credit days', empty: 'You owe no supplier anything', to: '/admin/suppliers/$id' as const,
  },
}

/** Customer / Supplier dues (karots-pos): the money owed either way, aged by
 * the oldest unpaid credit sale or bill. A live report — no period. */
export function DuesPage({ kind }: { kind: 'customer' | 'supplier' }) {
  const t = TEXT[kind]
  const navigate = useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const d = useQuery(duesQuery(kind)).data
  const top = num(d?.rows[0]?.balance ?? '0')
  return (
    <ReportPage title={t.title} sub={t.sub} exportUrl={`/api/v1/admin/reports/${kind}-dues`}>
      <KpiStrip label="Headline" cols={4}>
        <Kpi label={kind === 'customer' ? '↓ They owe you' : '↑ You owe'} amount={d?.total} tone={kind === 'customer' ? undefined : 'out'} v={d ? money(sym, d.total) : '…'} />
        <Kpi label={t.who} v={d ? String(d.count) : '…'} />
        <Kpi label={t.flag} amount={d?.flaggedSum} tone="out" v={d ? money(sym, d.flaggedSum) : '…'} sub={d && `${d.flagged} ${kind}${d.flagged === 1 ? '' : 's'}`} />
        <Kpi label="Oldest" v={d ? `${d.oldestDays} days` : '…'} />
      </KpiStrip>
      {d && d.count > 0 && <AgingBar buckets={d.buckets} total={d.total} sym={sym} />}
      <DataList label={t.who} rows={d?.rows ?? []} rowKey={(x) => x.id}
        onRow={(x) => void navigate({ to: t.to, params: { id: String(x.id) } })}
        empty={<EmptyState title={d ? t.empty : 'Loading…'} />}
        columns={[
          { key: 'name', header: 'Name', cell: (x) => (
            <span className="flex flex-col">
              <span className="flex items-center gap-1.5"><b>{x.name}</b>{x.flag && <Tag tone="danger">{t.tag}</Tag>}{x.disabled && <Tag tone="muted">disabled</Tag>}</span>
              {x.phone && <small className="font-normal text-faint">{x.phone}</small>}
            </span>
          ) },
          { key: 'bal', header: 'Owed', num: true, cell: (x) => <BarCell pct={top > 0 ? (num(x.balance) / top) * 100 : null} tone={kind === 'customer' ? 'brand' : 'danger'}>{money(sym, x.balance)}</BarCell> },
          { key: 'terms', header: t.terms, num: true, wide: true, cell: (x) => <span className="text-muted-foreground">{kind === 'customer' ? (num(x.terms) > 0 ? money(sym, x.terms) : '—') : (x.terms === '0' ? '—' : x.terms)}</span> },
          { key: 'age', header: 'Oldest unpaid', num: true, cell: (x) => (x.oldest ? <span className="flex flex-col items-end"><span>{x.ageDays} days</span><small className="text-faint">{x.oldest}</small></span> : <span className="text-faint">—</span>) },
        ]} />
    </ReportPage>
  )
}
