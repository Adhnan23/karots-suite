import type { SlipData } from '@/api/types.gen'
import type { SlipKit } from '../styles/kit'
import { display, money, pos } from '../styles/money'

// The till's other slips, line for line from their Go builders.

type P = { d: SlipData; kit: SlipKit }

/** `escpos.ReturnDocument` */
export function RefundSlip({ d, kit: K }: P) {
  const r = d.refund!
  const sym = d.currency
  return (
    <K.Page d={d}>
      <K.Header shop={d.shop} dots={d.dots} number={r.receiptNo} />
      <K.Title>Refund</K.Title>
      <K.Rule />
      <K.Info l="Orig. receipt:" r={r.receiptNo} />
      <K.Info l="Date:" r={r.date} />
      {r.customer && <K.Info l="Customer:" r={r.customer} />}
      {r.reason && <K.Text>{`Reason: ${r.reason}`}</K.Text>}
      <K.Rule />
      {(r.items ?? []).map((it, i) => (
        <div key={i}>
          <K.Item name={it.name} amount={money(sym, it.refund)}>
            <K.Sub l={`${display(it.quantity)} ${it.unitAbbr}`} r={money(sym, it.refund)} />
          </K.Item>
        </div>
      ))}
      <K.Rule />
      <K.Money bold l="CASH REFUND" r={money(sym, r.refund)} />
      {pos(r.creditReduction) && <K.Money l="Credit reduced" r={money(sym, r.creditReduction)} />}
      {r.remainingBalance && <K.Money l="Balance due" r={money(sym, r.remainingBalance)} />}
      <K.Rule />
      <K.Text center>Refund slip - please retain.</K.Text>
      <K.Footer shop={d.shop} number={r.receiptNo} />
    </K.Page>
  )
}

/** `moneySlip` in till_print.go (a CR- receipt) */
export function MoneySlip({ d, kit: K }: P) {
  const m = d.money!
  return (
    <K.Page d={d}>
      <K.Header shop={d.shop} dots={d.dots} number={m.receiptNo} />
      <K.Title>{m.title}</K.Title>
      <K.Rule />
      <K.Info l="Receipt:" r={m.receiptNo} />
      <K.Info l="Date:" r={m.date} />
      <K.Info l="From:" r={m.from} />
      <K.Info l="To:" r={m.to} />
      {m.party.trim() && <K.Info l="Party:" r={m.party} />}
      {m.by && <K.Info l="By:" r={m.by} />}
      <K.Rule />
      <K.Money bold l="Amount" r={money(d.currency, m.amount)} />
      {m.note.trim() && <K.Text>{`Note: ${m.note}`}</K.Text>}
      <K.Rule />
      {/* Classic's 20-underscore line, shortened to fit inside the margin on 58 mm (30 columns). */}
      <K.Text center>{`Signature: ${'_'.repeat(Math.min(20, d.columns - 2 - 'Signature: '.length))}`}</K.Text>
      <K.Gap />
      <K.Gap />
      <K.Footer shop={d.shop} number={m.receiptNo} />
    </K.Page>
  )
}

/** `escpos.DebtDocument` (a credit payment) */
export function DebtSlip({ d, kit: K }: P) {
  const s = d.debt!
  const sym = d.currency
  return (
    <K.Page d={d}>
      <K.Header shop={d.shop} dots={d.dots} number={s.receiptNo} />
      <K.Title>Credit payment</K.Title>
      <K.Rule />
      <K.Info l="Receipt:" r={s.receiptNo} />
      <K.Info l="Date:" r={s.date} />
      <K.Info l="Customer:" r={s.customer} />
      {s.phone && <K.Info l="Phone:" r={s.phone} />}
      <K.Rule />
      <K.Money bold l="Amount paid" r={money(sym, s.amount)} />
      <K.Info l="Method:" r={s.method} />
      {s.balanceAfter && (
        <>
          <K.Rule />
          <K.Money l="Previous balance" r={money(sym, s.balanceBefore)} />
          <K.Money bold l="Remaining balance" r={money(sym, s.balanceAfter)} />
        </>
      )}
      <K.Rule />
      {s.cashier && <K.Text center>{`Served by: ${s.cashier}`}</K.Text>}
      <K.Footer shop={d.shop} number={s.receiptNo} />
    </K.Page>
  )
}

/** `escpos.WarrantyDocument` (a serial replacement) */
export function WarrantySlip({ d, kit: K }: P) {
  const w = d.warranty!
  return (
    <K.Page d={d}>
      <K.Header shop={d.shop} dots={d.dots} />
      <K.Title>Warranty replacement</K.Title>
      <K.Rule />
      <K.Info l="Date:" r={w.date} />
      {w.customer && <K.Info l="Customer:" r={w.customer} />}
      <K.Text>{`Product: ${w.product}`}</K.Text>
      <K.Rule />
      <K.Text>{`Returned serial: ${w.oldSerial}`}</K.Text>
      <K.Text bold>{`New serial: ${w.newSerial}`}</K.Text>
      {w.warrantyUntil && <K.Info l="Warranty until:" r={w.warrantyLeft ? `${w.warrantyUntil} (${w.warrantyLeft})` : w.warrantyUntil} />}
      <K.Rule />
      <K.Text center>Replacement slip - please retain.</K.Text>
      <K.Footer shop={d.shop} />
    </K.Page>
  )
}

/** `escpos.TestDocument`: no header or footer — just enough to prove the wiring. */
export function TestSlip({ d, kit: K }: P) {
  return (
    <K.Page d={d}>
      <div style={{ fontWeight: 700, fontSize: 40, lineHeight: '48px', height: 48, textAlign: 'center', whiteSpace: 'pre' }}>{d.shop.name.trim() ? d.shop.name : 'POS'}</div>
      <K.Gap />
      <K.Text center>*** PRINTER TEST ***</K.Text>
      <K.Gap />
      <K.Rule />
      <K.Info l="Width:" r={`${d.width}mm (${d.columns} cols)`} />
      <K.Info l="Printed:" r={d.test!.printed} />
      <K.Rule />
      <K.Text center>If you can read this,</K.Text>
      <K.Text center>your printer is set up.</K.Text>
    </K.Page>
  )
}
