import type { ReorderList, ShopHeader } from '@/api/types.gen'
import { money, qtyText } from '@/lib/money'

/** The low-stock list on A4 (karots-pos LowStockPrint): what's at or under its
 * reorder level, the suggested quantity, and a blank Order column. */
export function LowStockPaper({ d, shop, sym, all }: { d: ReorderList; shop: ShopHeader; sym: string; all: boolean }) {
  return (
    <article className="mx-auto flex max-w-[760px] flex-col gap-5 bg-paper p-10 text-ink print:p-0">
      <header className="flex items-end justify-between border-b-2 border-ink pb-3">
        <div>
          <p className="text-[12px] uppercase tracking-widest">Low stock</p>
          <h1 className="font-display text-2xl font-extrabold">{shop.name}</h1>
        </div>
        <div className="text-right text-[13px]">
          <b className="block text-base">{all ? `All ${d.total}` : `${d.items.length} of ${d.total}`} items</b>
          printed {new Date().toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}
        </div>
      </header>
      <table className="w-full text-[12px]">
        <thead>
          <tr className="border-b border-ink text-left"><th className="py-1.5">Product</th><th>Supplier</th><th className="text-right">On hand</th><th className="text-right">Reorder at</th><th className="text-right">Suggest</th><th className="text-right">Cost</th><th className="w-20 text-right">Order</th></tr>
        </thead>
        <tbody>
          {d.items.map((r) => (
            <tr key={r.productId} className="break-inside-avoid border-b border-ink/20">
              <td className="py-1.5">{r.name}</td><td>{r.supplierName || '—'}</td>
              <td className="text-right tabular-nums">{qtyText(r.onHand)} {r.unitAbbr}</td><td className="text-right tabular-nums">{r.reorderLevel}</td>
              <td className="text-right tabular-nums">{qtyText(r.suggested)}</td><td className="text-right tabular-nums">{money(sym, r.cost)}</td>
              <td className="border-b border-ink/60" />
            </tr>
          ))}
          {d.items.length === 0 && <tr><td colSpan={7} className="py-6 text-center">Nothing is under its reorder level.</td></tr>}
        </tbody>
      </table>
    </article>
  )
}
