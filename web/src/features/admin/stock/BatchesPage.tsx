import { queryOptions, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ChevronRight, Search } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { BatchesView, BatchGroup, WriteOffResult } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { Seg } from '@/features/till/bits'
import { configQuery } from '@/features/till/api'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { daysLeft, ExpiryTag, LotsSheet } from './LotsSheet'

const batchesQuery = queryOptions({ queryKey: ['admin', 'batches'], queryFn: () => api<BatchesView>('/admin/stock/batches') })
type Show = 'all' | 'attention' | 'expired'

/** Live lots with expiry alerts and write-offs (karots-pos Batches). */
export function BatchesPage() {
  const v = useQuery(batchesQuery).data
  const sym = useSuspenseQuery(configQuery).data.symbol
  const qc = useQueryClient()
  const [q, setQ] = useState('')
  const [show, setShow] = useState<Show>('all')
  const [lots, setLots] = useState(0)
  const refresh = () => { void qc.invalidateQueries({ queryKey: ['admin'] }); void qc.invalidateQueries({ queryKey: ['till'] }) }
  const needle = q.trim().toLowerCase()
  const groups = (v?.groups ?? []).filter((g) => (!needle || g.name.toLowerCase().includes(needle))
    && (show === 'all' || g.expired > 0 || (show === 'attention' && g.expiringSoon > 0)))

  async function writeOffAll() {
    const r = await api<WriteOffResult>('/admin/stock/expired/write-off-all', { method: 'POST' })
    toast.success(r.lots ? `${r.lots} expired ${r.lots === 1 ? 'lot' : 'lots'} written off` : 'No expired stock to write off')
    refresh()
  }
  return (
    <>
      <PageHead title="Batches" sub="Every live lot, expiring first. Write off what’s past its date." />
      {v && (
        <div className="mb-4 grid grid-cols-2 gap-2.5 md:grid-cols-4">
          <Stat tone="danger" label="Expired" value={`${v.expired} ${v.expired === 1 ? 'lot' : 'lots'}`} sub={money(sym, v.expiredValue)}
            action={v.expired > 0 && <ConfirmButton label="Write off all" title={`Write off ${v.expired} expired lots?`} confirm="Write off"
              body={`${money(sym, v.expiredValue)} of stock leaves the shelf as damage. It shows under Losses.`} onConfirm={writeOffAll} className="mt-2 h-8 px-3 text-[12px]" />} />
          <Stat tone="warn" label="Expiring in 30 days" value={`${v.expiringSoon} ${v.expiringSoon === 1 ? 'lot' : 'lots'}`} sub={money(sym, v.expiringValue)} />
          <Stat label="Live stock at cost" value={money(sym, v.totalValue)} sub={`${v.groups.length} products`} />
          <Stat label="No expiry date" value={`${v.noExpiry} ${v.noExpiry === 1 ? 'lot' : 'lots'}`} sub="don’t go off" />
        </div>
      )}
      <div className="mb-3 grid gap-2 md:flex md:items-center">
        <div className="relative md:w-80">
          <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
          <input aria-label="Search products" placeholder="Product name" value={q} onChange={(e) => setQ(e.target.value)} className={cn(INPUT, 'pl-9')} />
        </div>
        <Seg label="Show" value={show} onChange={setShow} options={[['all', 'All'], ['attention', 'Needs attention'], ['expired', 'Expired']]} />
      </div>
      <div className={cn(SURFACE, 'overflow-hidden')}>
        {groups.length === 0 ? <EmptyState title={v ? 'Nothing here' : 'Loading…'}>{v && show !== 'all' && 'No lots need attention right now.'}</EmptyState> : (
          <ul aria-label="Products with lots" className="divide-y divide-border">
            {groups.map((g) => <Group key={g.productId} g={g} sym={sym} onLots={() => setLots(g.productId)} onDone={refresh} />)}
          </ul>
        )}
      </div>
      {lots > 0 && <LotsSheet key={lots} productId={lots} onClose={() => setLots(0)} />}
    </>
  )
}

function Stat({ label, value, sub, tone, action }: { label: string; value: string; sub: string; tone?: 'danger' | 'warn'; action?: React.ReactNode }) {
  return (
    <div className={cn(SURFACE, 'flex flex-col p-3.5', tone === 'danger' && 'bg-[color-mix(in_srgb,var(--danger)_10%,var(--card))]', tone === 'warn' && 'bg-[color-mix(in_srgb,var(--warning)_10%,var(--card))]')}>
      <small className={cn('text-[12px] font-semibold', tone === 'danger' ? 'text-danger' : tone === 'warn' ? 'text-warning' : 'text-muted-foreground')}>{label}</small>
      <b className="font-display text-xl tabular-nums">{value}</b>
      <small className="text-faint tabular-nums">{sub}</small>
      {action}
    </div>
  )
}

function Group({ g, sym, onLots, onDone }: { g: BatchGroup; sym: string; onLots: () => void; onDone: () => void }) {
  const [open, setOpen] = useState(g.expired > 0 || g.expiringSoon > 0)
  return (
    <li>
      <button type="button" aria-expanded={open} onClick={() => setOpen(!open)} className="flex w-full items-center gap-3 px-4 py-3 text-left hover:bg-muted">
        <ChevronRight size={16} className={cn('flex-none text-faint transition-transform', open && 'rotate-90')} />
        <span className="min-w-0 flex-1">
          <span className="flex flex-wrap items-center gap-1.5">
            <b className="truncate">{g.name}</b>
            {g.expired > 0 && <Tag tone="danger">{`${g.expired} expired`}</Tag>}
            {g.expiringSoon > 0 && <Tag tone="warn">{`${g.expiringSoon} expiring`}</Tag>}
          </span>
          <small className="text-faint">{g.lots.length} {g.lots.length === 1 ? 'lot' : 'lots'}</small>
        </span>
        <span className="text-right tabular-nums">
          <b className="block">{qtyText(g.onHand)} <small className="font-normal text-faint">{g.unitAbbr}</small></b>
          <small className="text-faint">{money(sym, g.value)}</small>
        </span>
      </button>
      {open && (
        <div className="px-4 pb-3 md:pl-11">
          <ul className="flex flex-col gap-1.5">
            {g.lots.map((l, i) => {
              const expired = !!l.expiryDate && daysLeft(l.expiryDate) < 0
              return (
                <li key={l.id} className={cn('flex flex-wrap items-center gap-x-3 gap-y-1 rounded-[var(--radius-sm)] border border-border px-3 py-2 text-sm', expired && 'border-danger bg-[color-mix(in_srgb,var(--danger)_6%,transparent)]')}>
                  <b className="tabular-nums">{qtyText(l.qtyRemaining)} {g.unitAbbr}</b>
                  {i === 0 && <Tag tone="brand">next to sell</Tag>}
                  <ExpiryTag ymd={l.expiryDate} />
                  <span className="text-faint">{l.expiryDate ? `exp ${l.expiryDate}` : 'no expiry'} · cost {money(sym, l.costPrice)} · sells {money(sym, l.sellingPrice || g.productPrice)}</span>
                  {expired && (
                    <span className="ml-auto">
                      <ConfirmButton label="Write off" title={`Write off ${qtyText(l.qtyRemaining)} ${g.unitAbbr} of ${g.name}?`} confirm="Write off" className="h-8 px-3 text-[12px]"
                        body={`Expired ${l.expiryDate}. It leaves the shelf as damage.`}
                        onConfirm={async () => {
                          await api('/admin/stock/damage', { json: { productId: g.productId, quantity: l.qtyRemaining, reason: 'damage', note: `expired ${l.expiryDate}`, batchId: l.id } })
                          toast.success(`${g.name}: expired lot written off`)
                          onDone()
                        }} />
                    </span>
                  )}
                </li>
              )
            })}
          </ul>
          <Button variant="ghost" className="mt-1.5 h-8 px-2 text-[13px]" onClick={onLots}>Lot prices…</Button>
        </div>
      )}
    </li>
  )
}
