import type { ReactNode } from 'react'
import type { DebtReceiptRow, MoneyReceiptRow, SaleView, ShopHeader } from '@/api/types.gen'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'

const stamp = (iso: string) => new Date(iso).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })
const METHOD: Record<string, string> = { cash: 'Cash', card: 'Card', online: 'Online', credit: 'On account' }

/** A thermal-receipt look on screen: paper and ink in every palette (it's paper). */
export function Paper({ shop, children, className }: { shop: ShopHeader; children: ReactNode; className?: string }) {
  return (
    <article className={cn('mx-auto w-[300px] bg-paper px-5 py-6 font-mono text-[12px] leading-relaxed text-ink shadow-[var(--shadow)]', className)}>
      <header className="mb-3 text-center">
        <b className="block text-[15px]">{shop.name}</b>
        {shop.address && <span className="block">{shop.address}</span>}
        {shop.phone && <span className="block">{shop.phone}</span>}
      </header>
      {children}
      {shop.footer && <footer className="mt-4 border-t border-dashed border-ink pt-2 text-center">{shop.footer}</footer>}
    </article>
  )
}

function Row({ l, r, b }: { l: ReactNode; r: ReactNode; b?: boolean }) {
  return <div className={cn('flex justify-between gap-3', b && 'font-bold')}><span>{l}</span><span className="tabular-nums">{r}</span></div>
}
const Rule = () => <hr className="my-2 border-dashed border-ink" />

export function SalePaper({ s, shop, sym }: { s: SaleView; shop: ShopHeader; sym: string }) {
  const change = num(s.changeGiven)
  return (
    <Paper shop={shop}>
      <Row l={s.receiptNo} r={stamp(s.createdAt)} />
      <Row l={`Cashier: ${s.cashierName}`} r={s.customerName || ''} />
      <Rule />
      {s.lines.map((l) => (
        <div key={l.id} className="mb-1">
          <span className="block">{l.name}</span>
          <Row l={`  ${qtyText(l.quantity)} × ${money('', l.unitPrice).trim()}`} r={money('', l.subtotal).trim()} />
          {num(l.returnedQty) > 0 && <span className="block">  returned {qtyText(l.returnedQty)}</span>}
        </div>
      ))}
      <Rule />
      <Row l="Subtotal" r={money(sym, s.subtotal)} />
      {num(s.discount) > 0 && <Row l="Discount" r={`-${money(sym, s.discount)}`} />}
      {num(s.tax) > 0 && <Row l="Tax" r={money(sym, s.tax)} />}
      <Row l="TOTAL" r={money(sym, s.total)} b />
      {s.payments.map((p, i) => <Row key={i} l={METHOD[p.method] ?? p.method} r={money(sym, p.amount)} />)}
      {change > 0 && <Row l="Change" r={money(sym, change)} />}
      {s.status !== 'completed' && <p className="mt-2 text-center font-bold uppercase">{s.status.replace('_', ' ')}</p>}
    </Paper>
  )
}

export function MoneyPaper({ r, shop, sym }: { r: MoneyReceiptRow; shop: ShopHeader; sym: string }) {
  return (
    <Paper shop={shop}>
      <p className="text-center font-bold uppercase">{r.kindLabel}</p>
      <Row l={r.receiptNo} r={stamp(r.createdAt)} />
      <Rule />
      <Row l="From" r={r.fromLabel} />
      <Row l="To" r={r.toLabel} />
      {r.party && <Row l="Party" r={r.party} />}
      {r.note && <span className="block">{r.note}</span>}
      <Rule />
      <Row l="AMOUNT" r={money(sym, r.amount)} b />
      {r.createdBy && <Row l="By" r={r.createdBy} />}
    </Paper>
  )
}

export function DebtPaper({ r, shop, sym }: { r: DebtReceiptRow; shop: ShopHeader; sym: string }) {
  return (
    <Paper shop={shop}>
      <p className="text-center font-bold uppercase">Credit payment</p>
      <Row l={r.receiptNo} r={stamp(r.createdAt)} />
      <Rule />
      <Row l="Customer" r={r.customerName} />
      {r.customerPhone && <Row l="Phone" r={r.customerPhone} />}
      <Row l="Paid by" r={r.method} />
      <Rule />
      <Row l="PAID" r={money(sym, r.amount)} b />
      {r.balanceBefore && <Row l="Balance before" r={money(sym, r.balanceBefore)} />}
      {r.balanceAfter && <Row l="Balance now" r={money(sym, r.balanceAfter)} />}
      {r.cashierName && <Row l="Cashier" r={r.cashierName} />}
    </Paper>
  )
}
