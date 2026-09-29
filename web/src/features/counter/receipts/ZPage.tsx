import type { ReactNode } from 'react'
import type { ShopHeader, ZReport } from '@/api/types.gen'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'

const MOVE: Record<string, string> = {
  opening: 'Opening cash', sale: 'Sale', credit_payment: 'Credit collected', withdrawal: 'Withdrawal',
  expense: 'Expense', pay_in: 'Pay in', refund: 'Refund', closing: 'Closing',
}
const METHOD: Record<string, string> = { cash: 'Cash', card: 'Card', online: 'Online', credit: 'On account' }
const stamp = (iso: string) => new Date(iso).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })
const clock = (iso: string) => new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="break-inside-avoid">
      <h2 className="mb-1.5 border-b border-ink pb-1 text-[13px] font-bold uppercase tracking-wide">{title}</h2>
      <table className="w-full text-[13px] tabular-nums"><tbody>{children}</tbody></table>
    </section>
  )
}
function Tr({ l, r, b }: { l: ReactNode; r: ReactNode; b?: boolean }) {
  return <tr className={cn('border-b border-ink/15 last:border-0', b && 'font-bold')}><td className="py-1">{l}</td><td className="py-1 text-right">{r}</td></tr>
}

/** Day-end (Z) — or, while the till is still open, an X read of the shift so far. A4 friendly. */
export function ZPage({ z, shop, sym }: { z: ZReport; shop: ShopHeader; sym: string }) {
  const diff = num(z.difference)
  return (
    <article className="mx-auto flex max-w-[760px] flex-col gap-6 bg-paper p-10 text-ink print:p-0">
      <header className="flex items-end justify-between border-b-2 border-ink pb-3">
        <div>
          <p className="text-[12px] uppercase tracking-widest">{z.closedAt ? 'Day-end (Z) report' : 'Shift so far (X read)'}</p>
          <h1 className="font-display text-2xl font-extrabold">{shop.name}</h1>
        </div>
        <div className="text-right text-[13px]">
          <b className="block">{z.cashierName}</b>
          {stamp(z.openedAt)} → {z.closedAt ? stamp(z.closedAt) : 'now'}
        </div>
      </header>
      <div className="grid grid-cols-2 gap-8">
        <Section title="Sales">
          <Tr l="Sales count" r={z.salesCount} />
          <Tr l="Gross" r={money(sym, z.gross)} />
          <Tr l="Discounts" r={money(sym, z.discount)} />
          <Tr l="Returns" r={money(sym, z.returns)} />
          <Tr l="Net sales" r={money(sym, z.net)} b />
        </Section>
        <Section title="Payments by method">
          {z.byMethod.length === 0 && <Tr l="No payments" r="" />}
          {z.byMethod.map((m) => <Tr key={m.method} l={METHOD[m.method] ?? m.method} r={money(sym, m.amount)} />)}
        </Section>
      </div>
      <Section title="Cash reconciliation">
        <Tr l="Opening cash" r={money(sym, z.openingCash)} />
        {z.expectedCash && <Tr l="Expected in drawer" r={money(sym, z.expectedCash)} />}
        {z.closingCash && <Tr l="Counted at close" r={money(sym, z.closingCash)} />}
        {z.difference && <Tr l="Over / short" r={`${diff > 0 ? '+' : ''}${money(sym, diff)}`} b />}
      </Section>
      <Section title="Cash movements">
        {z.moves.length === 0 && <Tr l="No movements" r="" />}
        {z.moves.map((m, i) => (
          <Tr key={i} l={<><span className="inline-block w-20 whitespace-nowrap">{clock(m.at)}</span>{MOVE[m.type] ?? m.type}{m.reason && <span className="opacity-60"> · {m.reason}</span>}</>} r={money(sym, m.amount)} />
        ))}
      </Section>
      {z.floats.map((g) => (
        <Section key={g.group} title={g.group}>
          {g.items.map((it) => <Tr key={it.name} l={it.name} r={money(sym, it.balance)} />)}
          <Tr l="Total" r={money(sym, g.total)} b />
        </Section>
      ))}
      <p className="text-center text-[11px] opacity-60">Printed {stamp(new Date().toISOString())}</p>
    </article>
  )
}
