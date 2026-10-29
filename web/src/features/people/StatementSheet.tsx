import type { ShopHeader, Statement } from '@/api/types.gen'
import { money, num } from '@/lib/money'

/** A customer's or supplier's ledger on A4: every bill, payment and return with a running balance. */
export function StatementSheet({ st, shop, sym, kind }: { st: Statement; shop: ShopHeader; sym: string; kind: 'supplier' | 'customer' }) {
  const bal = num(st.balance)
  const owes = kind === 'supplier' ? (bal < 0 ? 'They owe us' : 'We owe') : bal < 0 ? 'In credit' : 'Balance due'
  const cell = (v: string) => (num(v) ? money(sym, v) : '')
  return (
    <article className="mx-auto flex max-w-[760px] flex-col gap-6 bg-paper p-10 text-ink print:p-0">
      <header className="flex items-end justify-between border-b-2 border-ink pb-3">
        <div>
          <p className="text-[12px] uppercase tracking-widest">{kind === 'supplier' ? 'Supplier' : 'Customer'} statement</p>
          <h1 className="font-display text-2xl font-extrabold">{shop.name}</h1>
          <p className="text-[13px]">{[shop.address, shop.phone].filter(Boolean).join(' · ')}</p>
        </div>
        <div className="text-right text-[13px]">
          <b className="block text-base">{st.name}</b>
          {[st.contact, st.phone].filter(Boolean).join(' · ')}
          {st.address && <span className="block">{st.address}</span>}
          <span className="block">as of {new Date().toLocaleDateString([], { dateStyle: 'medium' })}</span>
        </div>
      </header>
      <table className="w-full text-[13px]">
        <thead>
          <tr className="border-b border-ink text-left"><th className="py-1.5">Date</th><th>Entry</th><th>Ref</th><th className="text-right">{kind === 'supplier' ? 'Owed +' : 'Charged'}</th><th className="text-right">{kind === 'supplier' ? 'Paid −' : 'Paid'}</th><th className="text-right">Balance</th></tr>
        </thead>
        <tbody>
          {st.lines.map((l, i) => (
            <tr key={i} className="border-b border-ink/15">
              <td className="py-1.5 whitespace-nowrap">{new Date(l.date).toLocaleDateString([], { day: 'numeric', month: 'short', year: '2-digit' })}</td>
              <td>{l.kind}</td><td className="text-ink/70">{l.ref}</td>
              <td className="text-right tabular-nums">{cell(l.debit)}</td><td className="text-right tabular-nums">{cell(l.credit)}</td>
              <td className="text-right tabular-nums">{money(sym, l.balance)}</td>
            </tr>
          ))}
          {st.lines.length === 0 && <tr><td colSpan={6} className="py-6 text-center">Nothing recorded yet.</td></tr>}
        </tbody>
        <tfoot>
          <tr className="border-t-2 border-ink font-bold"><td colSpan={3} className="py-1.5">Totals</td><td className="text-right tabular-nums">{money(sym, st.totalDebit)}</td><td className="text-right tabular-nums">{money(sym, st.totalCredit)}</td><td /></tr>
        </tfoot>
      </table>
      <p className="self-end text-right text-lg"><span className="text-[13px] uppercase tracking-widest">{owes}</span><b className="block font-display text-2xl">{money(sym, Math.abs(bal))}</b></p>
    </article>
  )
}
