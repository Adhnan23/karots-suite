import type { AdminCashflow, CashflowRow, ShopHeader } from '@/api/types.gen'
import { money, num } from '@/lib/money'
import { FLOW_KINDS } from '../lockers/kinds'

/** Cash flow on A4 (karots-pos CashflowPrintPage): the same totals and net
 * position as the screen, and every movement in the range. */
export function CashflowSheet({ d, rows, shop, sym }: { d: AdminCashflow; rows: CashflowRow[]; shop: ShopHeader; sym: string }) {
  const sum = (label: string, v: string) => (
    <div className="flex justify-between gap-3 py-1"><span>{label}</span><b className="tabular-nums">{money(sym, v)}</b></div>
  )
  return (
    <article className="mx-auto flex max-w-[760px] flex-col gap-6 bg-paper p-10 text-ink print:p-0">
      <header className="flex items-end justify-between border-b-2 border-ink pb-3">
        <div>
          <p className="text-[12px] uppercase tracking-widest">Cash flow</p>
          <h1 className="font-display text-2xl font-extrabold">{shop.name}</h1>
          <p className="text-[13px]">{[shop.address, shop.phone].filter(Boolean).join(' · ')}</p>
        </div>
        <div className="text-right text-[13px]">
          <b className="block text-base">{d.from === d.to ? d.from : `${d.from} – ${d.to}`}</b>
          printed {new Date().toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}
        </div>
      </header>
      <section className="grid grid-cols-2 gap-8 text-[13px]">
        <div className="divide-y divide-ink/15">
          <p className="pb-1 font-bold uppercase tracking-wider">In the range</p>
          {sum('Cash sales at the tills', d.tillSales)}
          {sum('Other money in', d.in)}
          {num(d.openings) !== 0 && sum('Opening balances of lockers added', d.openings)}
          {sum('Money out', d.out)}
          {sum('Moved between your places', d.moved)}
          <div className="flex justify-between border-t-2 border-ink py-1 font-bold"><span>Net change</span><span className="tabular-nums">{money(sym, d.net)}</span></div>
        </div>
        <div className="divide-y divide-ink/15">
          <p className="pb-1 font-bold uppercase tracking-wider">Now</p>
          {d.places.map((p) => <div key={p.value}>{sum(p.name, p.balance)}</div>)}
          <div className="flex justify-between border-t-2 border-ink py-1 font-bold"><span>Cash on hand</span><span className="tabular-nums">{money(sym, d.cashOnHand)}</span></div>
          {d.netLines.slice(1).map((l) => <div key={l.label}>{sum(`${l.sign > 0 ? '+' : '−'} ${l.label}`, l.amount)}</div>)}
          <div className="flex justify-between border-t-2 border-ink py-1 font-bold"><span>Net position</span><span className="tabular-nums">{money(sym, d.netPosition)}</span></div>
        </div>
      </section>
      <table className="w-full text-[12px]">
        <thead>
          <tr className="border-b border-ink text-left"><th className="py-1.5">When</th><th>Place</th><th>Movement</th><th>Receipt</th><th className="text-right">In</th><th className="text-right">Out</th></tr>
        </thead>
        <tbody>
          {rows.map((r, i) => {
            const v = num(r.delta)
            return (
              <tr key={i} className="border-b border-ink/15 align-top">
                <td className="py-1 whitespace-nowrap">{new Date(r.at).toLocaleString([], { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' })}</td>
                <td>{r.location}</td>
                <td>{FLOW_KINDS[r.kind] ?? r.kind}{r.moved && ' (moved)'}{r.note && <span className="block text-ink/70">{r.note}</span>}</td>
                <td className="font-mono">{r.receiptNo}</td>
                <td className="text-right tabular-nums">{v > 0 ? money(sym, v) : ''}</td>
                <td className="text-right tabular-nums">{v < 0 ? money(sym, -v) : ''}</td>
              </tr>
            )
          })}
          {rows.length === 0 && <tr><td colSpan={6} className="py-6 text-center">No movements in this range.</td></tr>}
        </tbody>
      </table>
    </article>
  )
}
