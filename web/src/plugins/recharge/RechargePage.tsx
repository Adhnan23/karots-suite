import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { Calculator, ShoppingCart } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { DataList } from '@/components/admin/DataList'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { configQuery as tillConfigQuery, errMsg } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { floatQuery, setRates, setupQuery } from './api'
import { CarriersTab } from './CarriersTab'
import { BuySheet, CountSheet } from './FloatSheets'
import { LedgerTab, ReportTab } from './LedgerTab'
import type { Float, Suggestion } from './types'

const CHIP = 'rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground whitespace-nowrap aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'
const WEEK = 7 * 24 * 3600 * 1000

/** Apps → Reload & Bills: Float (balances, what the counts suggest, earnings),
 * Carriers (rates, devices, packages), Ledger and Report. */
export function RechargePage({ path }: { path: string }) {
  const go = useNavigate()
  const tab = path.split('/')[0] ?? ''
  const open = (p: string) => void go({ to: '/admin/x/$key/$', params: { key: 'recharge', _splat: p } })
  const tabs: [string, string][] = [['', 'Float'], ['carriers', 'Carriers'], ['ledger', 'Ledger'], ['report', 'Report']]
  return (
    <>
      <PageHead title="Reload & Bills" sub="Each device's float, the carriers' commission and what the shop earned. Selling happens at the till." />
      <div className="mb-3 flex flex-wrap items-center gap-1.5">
        {tabs.map(([p, l]) => <button key={p} type="button" aria-pressed={tab === p} className={CHIP} onClick={() => open(p)}>{l}</button>)}
      </div>
      {tab === 'carriers' ? <CarriersTab /> : tab === 'ledger' ? <LedgerTab /> : tab === 'report' ? <ReportTab /> : <FloatTab onCarriers={() => open('carriers')} />}
    </>
  )
}

function Stat({ label, value, tone }: { label: string; value: string; tone?: string }) {
  return (
    <div className="min-w-0 rounded-[var(--radius)] border border-border bg-card px-3 py-2.5">
      <small className="text-muted-foreground">{label}</small>
      <b className={cn('block truncate font-display text-[17px] tabular-nums md:text-xl', tone)}>{value}</b>
    </div>
  )
}

function FloatTab({ onCarriers }: { onCarriers: () => void }) {
  const sym = useQuery(tillConfigQuery).data?.symbol ?? 'Rs.'
  const page = useQuery(floatQuery).data
  const [sheet, setSheet] = useState<'count' | 'buy' | null>(null)
  const [now] = useState(() => Date.now())
  if (!page) return null
  const stale = (f: Float) => !f.lastCount || now - new Date(f.lastCount).getTime() > WEEK
  const old = page.floats.filter(stale)
  return (
    <div className="grid gap-3">
      <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
        <Stat label="Float on devices" value={money(sym, page.total)} />
        <Stat label="Earned today" value={money(sym, page.today)} tone={num(page.today) < 0 ? 'text-danger' : 'text-success'} />
        <Stat label="Earned this month" value={money(sym, page.month)} tone={num(page.month) < 0 ? 'text-danger' : 'text-success'} />
        <button type="button" onClick={onCarriers} className="text-left"><Stat label="New packages to check" value={String(page.newPackages)} tone={page.newPackages ? 'text-warning' : undefined} /></button>
      </div>
      {page.suggestions.map((s) => <SuggestionCard key={s.carrierId} s={s} sym={sym} />)}
      {old.length > 0 && (
        <p role="status" className="rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] px-3 py-2 text-[13px] font-semibold text-warning">
          Not counted for a week: {old.map((f) => `${f.carrier} · ${f.label}`).join(', ')} — Count now checks them against the phones.
        </p>
      )}
      <div className="flex flex-wrap gap-2">
        <Button onClick={() => setSheet('count')} disabled={!page.floats.length}><Calculator className="size-4" /> Count now</Button>
        <Button variant="secondary" onClick={() => setSheet('buy')} disabled={!page.floats.length}><ShoppingCart className="size-4" /> Buy float</Button>
      </div>
      <DataList label="Floats" rows={page.floats} rowKey={(f) => f.id}
        empty={<EmptyState title="No devices yet">Add a carrier and its phone or SIM under Carriers.</EmptyState>}
        columns={[
          { key: 'd', header: 'Device', cell: (f) => <span className="font-semibold">{f.carrier} · {f.label}</span> },
          { key: 'b', header: 'Float', num: true, cell: (f) => <b className={cn('whitespace-nowrap', num(f.balance) < 0 && 'text-danger')}>{money(sym, f.balance)}</b> },
          { key: 't', header: 'Target', num: true, cell: (f) => (num(f.target) > 0 ? <span className="whitespace-nowrap">{money(sym, f.target)}</span> : '—') },
          { key: 'c', header: 'Last count', cell: (f) => <span className={cn(stale(f) && 'text-warning')}>{f.lastCount ? new Date(f.lastCount).toLocaleDateString() : 'never'}</span> },
          { key: 'u', header: 'For', wide: true, cell: (f) => [f.reload && 'reloads', f.money && 'money'].filter(Boolean).join(' · ') },
        ]} />
      {num(page.total) !== 0 && page.floats.some((f) => num(f.balance) < 0) && (
        <p className="text-[12.5px] text-muted-foreground">A float below zero means the suite expects less than the phone sent — count it to set it right.</p>
      )}
      {sheet === 'count' && <CountSheet floats={page.floats} sym={sym} onClose={() => setSheet(null)} />}
      {sheet === 'buy' && <BuySheet floats={page.floats} sym={sym} onClose={() => setSheet(null)} />}
    </div>
  )
}

/** "Counts keep coming out higher — Dialog reload ≈ 3.4%, not 3%. Update?" — never applied by itself. */
function SuggestionCard({ s, sym }: { s: Suggestion; sym: string }) {
  const setup = useQuery(setupQuery).data
  const [busy, setBusy] = useState(false)
  async function apply() {
    const c = setup?.carriers.find((x) => x.id === s.carrierId)
    if (!c) return
    setBusy(true)
    try {
      await setRates(c.id, c.rates.map((r) => (r.kind === 'reload' ? { ...r, percent: s.suggested } : r)))
      toast.success(`${s.carrier} reload commission is now ${num(s.suggested)}%`)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  const up = num(s.extra) > 0
  return (
    <div className="flex flex-wrap items-center gap-3 rounded-[var(--radius-sm)] border border-border bg-card px-3 py-2.5">
      <p className="min-w-0 flex-1 text-[13.5px]">
        <b>{s.carrier}:</b> the last {s.counts} counts came out {up ? 'higher' : 'lower'} by {money(sym, Math.abs(num(s.extra)))} on {money(sym, s.reloads)} of reloads —
        reload commission looks like <b>{num(s.suggested)}%</b>, not {num(s.current)}%.
      </p>
      <Button size="sm" disabled={busy || !setup} onClick={() => void apply()}>Update to {num(s.suggested)}%</Button>
    </div>
  )
}
