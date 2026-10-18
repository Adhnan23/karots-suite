import type { ReactNode } from 'react'
import type { DebtReceiptRow, MoneyReceiptRow, SaleView, ShopHeader, WarrantyClaimRow } from '@/api/types.gen'
import { money, num, qtyText, r2 } from '@/lib/money'
import { saleLineBlocks } from '@/lib/saleLines'
import { cn } from '@/lib/utils'

const stamp = (iso: string) => new Date(iso).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })
const METHOD: Record<string, string> = { cash: 'Cash', card: 'Card', online: 'Online', credit: 'On account' }

/** A thermal-receipt look on screen: paper and ink in every palette (it's paper). */
export function Paper({ shop, children, className, narrow, logo }: { shop: ShopHeader; children: ReactNode; className?: string; narrow?: boolean; logo?: string }) {
  // 80 mm ≈ 48 characters a line, 58 mm ≈ 32: the preview narrows to match.
  return (
    <article className={cn('mx-auto bg-paper py-6 font-mono leading-relaxed text-ink shadow-[var(--shadow)]',
      narrow ? 'w-[220px] px-3 text-[11px]' : 'w-[300px] px-5 text-[12px]', className)}>
      <header className="mb-3 text-center">
        {logo && <img src={logo} alt="" className="mx-auto mb-2 max-h-16 max-w-[70%] object-contain grayscale" />}
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

export function SalePaper({ s, shop, sym, narrow }: { s: SaleView; shop: ShopHeader; sym: string; narrow?: boolean }) {
  const change = num(s.changeGiven)
  // Payout lines (negative) are money handed to the customer: shown as Given, kept out of
  // the subtotal; a sale they take below zero ends in Given, not TOTAL.
  const given = -s.lines.reduce((t, l) => t + Math.min(0, num(l.subtotal)), 0)
  const below = num(s.total) < 0
  return (
    <Paper shop={shop} narrow={narrow}>
      <Row l={s.receiptNo} r={stamp(s.createdAt)} />
      <Row l={`Cashier: ${s.cashierName}`} r={s.customerName || ''} />
      <Rule />
      {saleLineBlocks(s.lines).map((x) => (x.kind === 'line'
        ? (
          <div key={x.line.id} className="mb-1">
            <span className="block">{x.line.name}</span>
            {num(x.line.subtotal) < 0 ? <Row l="  Given" r={`-${money('', -num(x.line.subtotal)).trim()}`} /> : <Row l={x.line.packLabel ? `  ${qtyText(x.line.quantity)} ${x.line.unitAbbr} (${x.line.packLabel})` : `  ${qtyText(x.line.quantity)} × ${money('', x.line.unitPrice).trim()}`} r={money('', x.line.subtotal).trim()} />}
            {num(x.line.returnedQty) > 0 && <span className="block">  returned {qtyText(x.line.returnedQty)}</span>}
          </div>
        )
        : (
          <div key={`g${x.no}`} className="mb-1">
            <span className="block">{x.label}</span>
            <Row l={`  ${qtyText(x.sets)} × ${money('', r2(x.net / num(x.sets))).trim()}`} r={money('', x.net).trim()} />
            {x.lines.map((l) => (
              <span key={l.id} className="block whitespace-pre">{`   - ${l.name} ×${qtyText(l.quantity)}`}{num(l.returnedQty) > 0 && `, returned ${qtyText(l.returnedQty)}`}</span>
            ))}
          </div>
        )))}
      <Rule />
      <Row l="Subtotal" r={money(sym, num(s.subtotal) + given)} />
      {num(s.discount) > 0 && <Row l="Discount" r={`-${money(sym, s.discount)}`} />}
      {num(s.tax) > 0 && <Row l="Tax" r={money(sym, s.tax)} />}
      {given > 0 && <Row l="Given to customer" r={`-${money(sym, given)}`} />}
      {!below && <Row l="TOTAL" r={money(sym, s.total)} b />}
      {s.payments.filter((p) => num(p.amount) !== 0).map((p, i) => <Row key={i} l={METHOD[p.method] ?? p.method} r={money(sym, p.amount)} />)}
      {change > 0 && <Row l={below ? 'GIVEN' : 'Change'} r={money(sym, change)} b={below} />}
      {s.status !== 'completed' && <p className="mt-2 text-center font-bold uppercase">{s.status.replace('_', ' ')}</p>}
    </Paper>
  )
}

export function MoneyPaper({ r, shop, sym, narrow }: { r: MoneyReceiptRow; shop: ShopHeader; sym: string; narrow?: boolean }) {
  return (
    <Paper shop={shop} narrow={narrow}>
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

export function DebtPaper({ r, shop, sym, narrow }: { r: DebtReceiptRow; shop: ShopHeader; sym: string; narrow?: boolean }) {
  return (
    <Paper shop={shop} narrow={narrow}>
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

export function WarrantyPaper({ r, shop, narrow }: { r: WarrantyClaimRow; shop: ShopHeader; narrow?: boolean }) {
  return (
    <Paper shop={shop} narrow={narrow}>
      <p className="text-center font-bold uppercase">Warranty {r.resolution}</p>
      <Row l={`Claim #${r.id}`} r={stamp(r.at)} />
      <Rule />
      <span className="block font-bold">{r.productName}</span>
      {r.oldSerial && <Row l="Old serial" r={r.oldSerial} />}
      {r.replacementSerial && <Row l="New serial" r={r.replacementSerial} />}
      {r.customerName && <Row l="Customer" r={r.customerName} />}
      {r.reason && <Row l="Reason" r={r.reason} />}
      <Rule />
      <Row l="Handled by" r={r.handledBy} />
    </Paper>
  )
}
