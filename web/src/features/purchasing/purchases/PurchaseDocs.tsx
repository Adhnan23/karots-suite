import type { PurchaseDetail, ShopHeader } from '@/api/types.gen'
import { money, num, qtyText } from '@/lib/money'

const date = (iso: string) => new Date(iso).toLocaleDateString([], { dateStyle: 'medium' })

/** Shop ↔ supplier header block shared by the PO and the GRN (karots-pos fromTo). */
function FromTo({ p, shop, kind }: { p: PurchaseDetail; shop: ShopHeader; kind: string }) {
  const s = p.supplier
  return (
    <header className="flex items-end justify-between gap-6 border-b-2 border-ink pb-3">
      <div>
        <p className="text-[12px] uppercase tracking-widest">{kind}</p>
        <h1 className="font-display text-2xl font-extrabold">{shop.name}</h1>
        <p className="text-[13px]">{[shop.address, shop.phone].filter(Boolean).join(' · ')}</p>
      </div>
      <div className="text-right text-[13px]">
        <span className="block text-[11px] uppercase tracking-widest">{kind.startsWith('Purchase') ? 'To' : 'From'}</span>
        <b className="block text-base">{s.name}</b>
        {[s.contact, s.phone].filter(Boolean).join(' · ')}
        {s.address && <span className="block">{s.address}</span>}
      </div>
    </header>
  )
}

/** A purchase order on A4: what we want, at the price we expect. */
export function PoSheet({ p, shop, sym }: { p: PurchaseDetail; shop: ShopHeader; sym: string }) {
  return (
    <article className="mx-auto flex max-w-[760px] flex-col gap-5 bg-paper p-10 text-ink print:p-0">
      <FromTo p={p} shop={shop} kind={`Purchase order #${p.id}`} />
      <p className="text-[13px]">Ordered {date(p.date)}{p.expectedDate && ` · please deliver by ${date(p.expectedDate)}`}</p>
      <table className="w-full text-[13px]">
        <thead><tr className="border-b border-ink text-left"><th className="py-1.5">#</th><th>Item</th><th className="text-right">Qty</th><th className="text-right">Unit cost</th><th className="text-right">Amount</th></tr></thead>
        <tbody>
          {p.lines.map((l, i) => (
            <tr key={l.productId} className="border-b border-ink/15">
              <td className="py-1.5">{i + 1}</td><td>{l.name}</td>
              <td className="text-right tabular-nums">{qtyText(l.quantity)} {l.unitAbbr}</td>
              <td className="text-right tabular-nums">{money(sym, l.costPrice)}</td>
              <td className="text-right tabular-nums">{money(sym, l.net)}</td>
            </tr>
          ))}
        </tbody>
        <tfoot>
          {num(p.discount) > 0 && <tr><td colSpan={4} className="pt-2 text-right">Discount</td><td className="pt-2 text-right tabular-nums">−{money(sym, p.discount)}</td></tr>}
          <tr className="font-bold"><td colSpan={4} className="py-1.5 text-right">Total</td><td className="text-right tabular-nums">{money(sym, p.total)}</td></tr>
        </tfoot>
      </table>
      {p.notes && <p className="text-[13px]"><b>Notes:</b> {p.notes}</p>}
    </article>
  )
}

/** Goods received note (karots-pos GRN slip): ordered vs received, invoice, paid, balance. */
export function GrnSheet({ p, shop, sym }: { p: PurchaseDetail; shop: ShopHeader; sym: string }) {
  const ordered = p.lines.some((l) => l.ordered)
  return (
    <article className="mx-auto flex max-w-[760px] flex-col gap-5 bg-paper p-10 text-ink print:p-0">
      <FromTo p={p} shop={shop} kind="Goods received note" />
      <p className="text-[13px]">Received {date(p.date)}{p.by && ` by ${p.by}`}{p.invoiceNo && ` · their invoice ${p.invoiceNo}`} · GRN #{p.id}</p>
      <table className="w-full text-[13px]">
        <thead>
          <tr className="border-b border-ink text-left"><th className="py-1.5">Item</th>{ordered && <th className="text-right">Ordered</th>}<th className="text-right">Received</th><th className="text-right">Cost</th><th className="text-right">Amount</th></tr>
        </thead>
        <tbody>
          {p.lines.map((l) => (
            <tr key={l.productId} className="border-b border-ink/15">
              <td className="py-1.5">{l.name}{l.expiryDate && <small className="block">exp {l.expiryDate}</small>}</td>
              {ordered && <td className="text-right tabular-nums">{l.ordered ? qtyText(l.ordered) : '—'}</td>}
              <td className="text-right tabular-nums">{qtyText(l.quantity)}{num(l.freeQty) > 0 && ` + ${qtyText(l.freeQty)} free`}</td>
              <td className="text-right tabular-nums">{money(sym, l.costPrice)}</td>
              <td className="text-right tabular-nums">{money(sym, l.net)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <dl className="ml-auto grid w-64 grid-cols-2 gap-y-1 text-[13px] tabular-nums">
        {num(p.discount) > 0 && <><dt>Discount</dt><dd className="text-right">−{money(sym, p.discount)}</dd></>}
        <dt className="font-bold">Total</dt><dd className="text-right font-bold">{money(sym, p.total)}</dd>
        <dt>Paid</dt><dd className="text-right">{money(sym, p.paid)}</dd>
        {num(p.credited) > 0 && <><dt>Returned</dt><dd className="text-right">{money(sym, p.credited)}</dd></>}
        <dt className="font-bold">Balance</dt><dd className="text-right font-bold">{money(sym, Math.max(num(p.balance), 0))}</dd>
      </dl>
      <div className="mt-10 grid grid-cols-2 gap-10 text-[12px]"><span className="border-t border-ink pt-1">Received by</span><span className="border-t border-ink pt-1">Supplier's signature</span></div>
    </article>
  )
}
