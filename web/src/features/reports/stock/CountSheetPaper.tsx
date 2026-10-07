import type { CountSheet, ShopHeader } from '@/api/types.gen'
import { qtyText } from '@/lib/money'

/** The count sheet on A4 (karots-pos CountSheetPrintPage): grouped by
 * category, with blank Counted / Order columns to fill in at the shelf. */
export function CountSheetPaper({ d, shop }: { d: CountSheet; shop: ShopHeader }) {
  const groups = d.rows.reduce<[string, CountSheet['rows']][]>((acc, r) => {
    const last = acc.at(-1)
    if (last && last[0] === r.category) last[1].push(r)
    else acc.push([r.category, [r]])
    return acc
  }, [])
  return (
    <article className="mx-auto flex max-w-[760px] flex-col gap-5 bg-paper p-10 text-ink print:p-0">
      <header className="flex items-end justify-between border-b-2 border-ink pb-3">
        <div>
          <p className="text-[12px] uppercase tracking-widest">Stock count sheet</p>
          <h1 className="font-display text-2xl font-extrabold">{shop.name}</h1>
          <p className="text-[13px]">{[d.category && `Category: ${d.category}`, d.supplier && `Supplier: ${d.supplier}`].filter(Boolean).join(' · ') || 'Every stocked product'}</p>
        </div>
        <div className="text-right text-[13px]">
          <b className="block text-base">{d.rows.length} products</b>
          printed {new Date().toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })}<br />Counted by ____________
        </div>
      </header>
      {groups.map(([cat, rows]) => (
        <table key={cat} className="w-full break-inside-auto text-[12px]">
          <thead>
            <tr><th colSpan={5} className="pb-1 text-left text-[13px] font-bold uppercase tracking-wider">{cat || 'No category'}</th></tr>
            <tr className="border-b border-ink text-left"><th className="py-1">Product</th><th>Barcode</th><th className="text-right">On hand</th><th className="w-24 text-right">Counted</th><th className="w-20 text-right">Order</th></tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id} className="break-inside-avoid border-b border-ink/20">
                <td className="py-1.5">{r.name}</td><td className="font-mono">{r.barcode}</td>
                <td className="text-right tabular-nums">{qtyText(r.onHand)} {r.unit}</td>
                <td className="border-b border-ink/60" /><td className="border-b border-ink/60" />
              </tr>
            ))}
          </tbody>
        </table>
      ))}
      {d.rows.length === 0 && <p className="py-6 text-center">No stocked products match.</p>}
    </article>
  )
}
