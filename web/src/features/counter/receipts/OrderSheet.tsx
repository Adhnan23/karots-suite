import type { OrderView, ShopHeader } from '@/api/types.gen'
import { qtyText } from '@/lib/money'

/** A purchase order to hand or send to the supplier (A4). No prices: it's what we want. */
export function OrderSheet({ o, shop }: { o: OrderView; shop: ShopHeader }) {
  return (
    <article className="mx-auto flex max-w-[760px] flex-col gap-6 bg-paper p-10 text-ink print:p-0">
      <header className="flex items-end justify-between border-b-2 border-ink pb-3">
        <div>
          <p className="text-[12px] uppercase tracking-widest">Purchase order #{o.id}</p>
          <h1 className="font-display text-2xl font-extrabold">{shop.name}</h1>
          <p className="text-[13px]">{[shop.address, shop.phone].filter(Boolean).join(' · ')}</p>
        </div>
        <div className="text-right text-[13px]">
          <span className="block">To</span>
          <b className="block text-base">{o.supplierName}</b>
          {new Date(o.date).toLocaleDateString([], { dateStyle: 'medium' })}
        </div>
      </header>
      <table className="w-full text-[14px]">
        <thead><tr className="border-b border-ink text-left"><th className="py-1.5">#</th><th>Item</th><th className="text-right">Qty</th></tr></thead>
        <tbody>
          {o.lines.map((l, i) => (
            <tr key={l.productId} className="border-b border-ink/15"><td className="py-1.5">{i + 1}</td><td>{l.name}</td><td className="text-right tabular-nums">{qtyText(l.ordered)}</td></tr>
          ))}
        </tbody>
      </table>
      {o.notes && <p className="text-[14px]"><b>Notes:</b> {o.notes}</p>}
    </article>
  )
}
