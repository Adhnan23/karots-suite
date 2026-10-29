import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { getRouteApi, Link } from '@tanstack/react-router'
import { ArrowLeft, FileText, Lock } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { EmptyState, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { configQuery } from '@/features/till/api'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { stamp } from '../format'
import { tillQuery } from '../queries'
import { CloseForThem } from './CloseForThem'
import { OverShort } from './TillsPage'

const route = getRouteApi('/_authed/admin/tills/$id')

/** What a drawer movement was, in the till's own words. */
const MOVES: Record<string, string> = {
  opening: 'Opened', pay_in: 'Deposit', withdrawal: 'Withdraw', expense: 'Expense', credit_payment: 'Credit collected', refund: 'Refund', closing: 'Closed',
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className={cn(SURFACE, 'overflow-hidden')}>
      <h2 className="border-b border-border px-4 py-2.5 text-[12px] font-bold uppercase tracking-wide text-faint">{title}</h2>
      {children}
    </section>
  )
}

/** A drawer count by denomination: value × count = amount, and the total. */
function Count({ sym, count }: { sym: string; count: Record<string, number> }) {
  const lines = Object.entries(count).filter(([, n]) => n > 0).sort(([a], [b]) => num(b) - num(a))
  if (!lines.length) return <p className="p-4 text-sm text-faint">Typed as a total — not counted note by note.</p>
  const total = lines.reduce((t, [v, n]) => t + num(v) * n, 0)
  return (
    <table className="w-full text-sm">
      <tbody>
        {lines.map(([v, n]) => (
          <tr key={v} className="border-b border-border">
            <td className="px-4 py-1.5 tabular-nums">{money(sym, v)}</td>
            <td className="px-2 py-1.5 text-right tabular-nums text-muted-foreground">× {n}</td>
            <td className="px-4 py-1.5 text-right tabular-nums">{money(sym, num(v) * n)}</td>
          </tr>
        ))}
        <tr className="font-bold"><td className="px-4 py-2" colSpan={2}>Total</td><td className="px-4 py-2 text-right tabular-nums">{money(sym, total)}</td></tr>
      </tbody>
    </table>
  )
}

/** One till session: its money, every drawer
 * movement, the counts at open and close, and its Z report. */
export function TillPage() {
  const id = Number(route.useParams().id)
  const sym = useSuspenseQuery(configQuery).data.symbol
  const v = useQuery(tillQuery(id)).data
  const [closing, setClosing] = useState(false)
  if (!v) return <EmptyState title="Loading…" />
  const s = v.session
  const tile = (label: string, body: ReactNode) => (
    <div className="flex flex-col gap-0.5 px-4 py-3"><small className="text-[12px] font-semibold text-muted-foreground">{label}</small>{body}</div>
  )
  return (
    <>
      <Link to="/admin/tills" className="mb-2 inline-flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground"><ArrowLeft size={14} /> Till sessions</Link>
      <header className={cn(SURFACE, 'mb-4 flex flex-col gap-4 p-5 md:flex-row md:items-end md:justify-between')}>
        <div className="min-w-0">
          <h1 className="flex flex-wrap items-center gap-2 font-display text-[24px] font-bold tracking-tight md:text-[28px]">
            {s.cashier}<span className="flex flex-wrap gap-1.5 font-sans tracking-normal">
              {s.open && (s.stale ? <Tag tone="danger">left open</Tag> : <Tag tone="brand">open now</Tag>)}
              {s.closedLate && <Tag tone="warn">closed late</Tag>}
              {s.closedBy && <Tag tone="muted">{`closed by ${s.closedBy}`}</Tag>}
            </span>
          </h1>
          <p className="text-sm text-muted-foreground">{stamp(s.openedAt)} → {s.closedAt ? stamp(s.closedAt) : 'still open'}</p>
        </div>
        <div className="flex flex-wrap gap-2">
          {s.open && <Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setClosing(true)}><Lock size={15} /> Close for them</Button>}
          <Button variant="outline" className="h-10 gap-1.5 px-4" asChild>
            <a href={`/print/z/${id}`} target="_blank" rel="noreferrer"><FileText size={15} /> Z report</a>
          </Button>
        </div>
      </header>
      <section aria-label="Money" className={cn(SURFACE, 'mb-4 grid grid-cols-2 md:grid-cols-4 md:divide-x divide-border')}>
        {tile('Opening float', <b className="font-display text-xl tabular-nums">{money(sym, s.opening)}</b>)}
        {tile(s.open ? 'Should be in it now' : 'Expected at close', <b className="font-display text-xl tabular-nums">{s.expected ? money(sym, s.expected) : '—'}</b>)}
        {tile('Counted', <b className="font-display text-xl tabular-nums">{s.counted ? money(sym, s.counted) : '—'}</b>)}
        {tile('Over / short', <OverShort sym={sym} v={s.difference} big />)}
      </section>
      {closing && <CloseForThem s={s} onClose={() => setClosing(false)} />}
      <div className="grid gap-4 lg:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)]">
        <Section title="Drawer movements">
          {v.moves.length === 0 ? <p className="p-4 text-sm text-faint">No deposits, withdrawals or expenses.</p> : (
            <ul className="divide-y divide-border">
              {v.moves.map((m, i) => {
                const n = num(m.amount)
                return (
                  <li key={i} className="flex items-center justify-between gap-3 px-4 py-2.5 text-sm">
                    <span className="min-w-0">
                      <b>{MOVES[m.type] ?? m.type}</b>
                      <small className="block truncate text-faint">{stamp(m.at)}{m.reason && ` · ${m.reason}`}</small>
                    </span>
                    <b className={cn('tabular-nums', n >= 0 ? 'text-success' : 'text-danger')}>{n >= 0 ? '↓ ' : '↑ '}{money(sym, Math.abs(n))}</b>
                  </li>
                )
              })}
            </ul>
          )}
        </Section>
        <div className="flex flex-col gap-4">
          <Section title="Counted at open"><Count sym={sym} count={v.openingCount} /></Section>
          {!s.open && <Section title="Counted at close"><Count sym={sym} count={v.closingCount} /></Section>}
        </div>
      </div>
    </>
  )
}
