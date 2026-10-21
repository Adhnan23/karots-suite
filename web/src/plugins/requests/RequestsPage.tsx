import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Check, MoreHorizontal, Phone } from 'lucide-react'
import { useRef, useState } from 'react'
import { toast } from 'sonner'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { DataList } from '@/components/admin/DataList'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Pager } from '@/components/admin/Pager'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { useDebounced } from '@/lib/useDebounced'
import { clearDone, linkProduct, listQuery, markCalled, removeRequest, reopen } from './api'
import { askLabel, whenLabel } from './labels'
import { DeclineSheet, EditSheet, MergePicker, ProductSheet } from './Sheets'
import type { RequestRow, RequestTab } from './types'

const CHIP = 'rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground whitespace-nowrap aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'
const TABS: { tab: RequestTab; label: string; path: string }[] = [
  { tab: 'open', label: 'Open', path: '' },
  { tab: 'call', label: 'To call', path: 'call' },
  { tab: 'done', label: 'Done', path: 'done' },
]
const PAGE = 50

type Sheet = { kind: 'product' | 'decline' | 'edit' | 'merge'; r: RequestRow } | null

async function act(p: Promise<unknown>, done: string) {
  try {
    await p
    toast.success(done)
  } catch (e) {
    toast.error(errMsg(e))
  }
}

function StageTags({ r }: { r: RequestRow }) {
  return (
    <span className="flex flex-wrap items-center gap-1.5">
      {r.stage === 'waiting' && <Tag tone="warn">Waiting for stock</Tag>}
      {r.stage === 'call' && <Tag tone="brand">In stock</Tag>}
      {r.status === 'declined' && <Tag tone="muted">{`Won't stock: ${r.reason}`}</Tag>}
      {r.productName && <Tag tone="muted">{`→ ${r.productName}`}</Tag>}
    </span>
  )
}

/** Who asked: name, a call link, when — and Called ✓ once the item is in. */
function Asks({ r }: { r: RequestRow }) {
  return (
    // On a phone DataList puts action cells in a right-aligned row beside the ⋯ menu;
    // this list takes that row's width (card minus the menu) and reads left to right.
    <span className="flex flex-col gap-1 text-[13px] phone:w-[calc(100vw-108px)]">
      {r.asks.map((a) => (
        <span key={a.id} className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
          <span className="font-medium">{a.name || 'A customer'}</span>
          {a.phone && <a href={`tel:${a.phone}`} className="inline-flex items-center gap-1 whitespace-nowrap text-brand hover:underline"><Phone size={12} />{a.phone}</a>}
          <span className="whitespace-nowrap text-faint">{whenLabel(a.at)}{a.by && ` · ${a.by}`}</span>
          {a.calledAt
            ? <span className="inline-flex items-center gap-1 text-success"><Check size={12} />called</span>
            : r.stage === 'call' && a.phone && (
              <Button size="sm" variant="secondary" className="h-7 px-2.5" onClick={() => act(markCalled(r.id, a.id), `${a.name || a.phone} marked called`)}>Called ✓</Button>
            )}
        </span>
      ))}
    </span>
  )
}

/** Apps → Customer requests: what customers asked for that the shop doesn't carry.
 * The tab is the sub-path: /admin/x/requests (open), …/call, …/done. */
export function RequestsPage({ path }: { path: string }) {
  const tab = TABS.find((t) => t.path === path)?.tab ?? 'open'
  const go = useNavigate()
  const [q, setQ] = useState('')
  const [page, setPage] = useState(1)
  const [sheet, setSheet] = useState<Sheet>(null)
  const dq = useDebounced(q.trim(), 250)
  const data = useQuery(listQuery(tab, dq, page)).data
  const close = () => setSheet(null)
  // One product picker for the page; the menu item opens it for its row.
  const linkFor = useRef<RequestRow | null>(null)
  const openPicker = useRef<(() => void) | null>(null)
  const open = (kind: NonNullable<Sheet>['kind'], r: RequestRow) => setSheet({ kind, r })

  return (
    <>
      <PageHead title="Customer requests"
        sub={data ? `${data.counts.open} open · ${data.counts.call} to call` : 'What customers asked for that you don’t stock yet.'}
        actions={tab === 'done' && data?.rows.some((r) => r.status === 'converted') && (
          <ConfirmButton plain label="Clear done" title="Clear done requests?" confirm="Clear done" className="h-10 px-4 font-bold"
            body="Removes requests already made into products with nobody left to call. Won’t-stock ones stay — the till still shows why."
            onConfirm={() => act(clearDone(), 'Done requests cleared')} />
        )} />
      <div className="mb-3 flex flex-wrap items-center gap-1.5">
        {TABS.map((t) => (
          <button key={t.tab} type="button" aria-pressed={tab === t.tab} className={CHIP}
            onClick={() => { setPage(1); void go({ to: '/admin/x/$key/$', params: { key: 'requests', _splat: t.path } }) }}>
            {t.label}{data && data.counts[t.tab] > 0 ? ` · ${data.counts[t.tab]}` : ''}
          </button>
        ))}
        <input aria-label="Search requests" placeholder="Search" value={q} onChange={(e) => { setQ(e.target.value); setPage(1) }}
          className={`${INPUT} h-10 w-full md:ml-auto md:w-64`} />
      </div>
      <DataList label="Customer requests" rows={data?.rows ?? []} rowKey={(r) => r.id}
        empty={<EmptyState title={!data ? 'Loading…' : dq ? 'Nothing matches' : tab === 'call' ? 'Nobody to call' : tab === 'done' ? 'Nothing done yet' : 'No requests yet'}>
          {data && !dq && tab === 'open' && 'Cashiers add one from the till with Note request — or when a search finds nothing.'}
          {data && !dq && tab === 'call' && 'A request shows here once its product is in stock and someone left a phone number.'}
        </EmptyState>}
        footer={data && <Pager page={page} pageSize={PAGE} total={data.total} onPage={setPage} />}
        columns={[
          { key: 'note', header: 'Request', cell: (r) => (
            <span className="flex flex-col gap-1">
              <b className="break-words">{r.note}</b>
              <span className="text-[13px] font-normal text-muted-foreground">{askLabel(r.askCount, r.lastAsked)}</span>
              <StageTags r={r} />
            </span>
          ) },
          { key: 'asks', header: 'Customers', interactive: true, cell: (r) => <Asks r={r} /> },
          { key: 'act', header: '', interactive: true, className: 'w-[1%]', cell: (r) => (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" className="h-9 w-9 p-0" aria-label={`Actions for ${r.note}`}><MoreHorizontal size={18} /></Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                {r.status === 'open' && <>
                  <DropdownMenuItem onSelect={() => open('product', r)}>Make product</DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => { linkFor.current = r; openPicker.current?.() }}>Link to a product</DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => open('decline', r)}>Won’t stock</DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => open('merge', r)}>Merge into…</DropdownMenuItem>
                </>}
                {r.status !== 'open' && <DropdownMenuItem onSelect={() => void act(reopen(r.id), 'Reopened')}>Reopen</DropdownMenuItem>}
                <DropdownMenuItem onSelect={() => open('edit', r)}>Edit</DropdownMenuItem>
                <DropdownMenuItem className="text-danger" onSelect={() => void act(removeRequest(r.id), 'Request removed')}>Remove</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) },
        ]} />
      <ProductPicker title="Link to a product"
        onPick={(p) => { const r = linkFor.current; if (r) void act(linkProduct(r.id, p.id), `“${r.note}” linked to ${p.name}`) }}
        trigger={(o) => { openPicker.current = o; return null }} />
      {sheet?.kind === 'product' && <ProductSheet r={sheet.r} onClose={close} />}
      {sheet?.kind === 'decline' && <DeclineSheet r={sheet.r} onClose={close} />}
      {sheet?.kind === 'edit' && <EditSheet r={sheet.r} onClose={close} />}
      {sheet?.kind === 'merge' && <MergePicker r={sheet.r} onClose={close} />}
    </>
  )
}
