import type { SlipData } from '@/api/types.gen'
import { Blank, display, Footer, Header, money, pos, ReceiptPage, Row, Rule, Text, Title } from '../kit'

// The till's other slips, line for line from their Go builders.

/** `escpos.ReturnDocument` */
export function RefundSlip({ d }: { d: SlipData }) {
  const r = d.refund!
  const sym = d.currency
  return (
    <ReceiptPage dots={d.dots} columns={d.columns}>
      <Header shop={d.shop} dots={d.dots} />
      <Title>Refund</Title>
      <Rule />
      <Row l="Orig. receipt:" r={r.receiptNo} />
      <Row l="Date:" r={r.date} />
      {r.customer && <Row l="Customer:" r={r.customer} />}
      {r.reason && <Text>{`Reason: ${r.reason}`}</Text>}
      <Rule />
      {(r.items ?? []).map((it, i) => (
        <div key={i}>
          <Text>{it.name}</Text>
          <Row l={`  ${display(it.quantity)} ${it.unitAbbr}`} r={money(sym, it.refund)} />
        </div>
      ))}
      <Rule />
      <Row bold l="CASH REFUND" r={money(sym, r.refund)} />
      {pos(r.creditReduction) && <Row l="Credit reduced" r={money(sym, r.creditReduction)} />}
      {r.remainingBalance && <Row l="Balance due" r={money(sym, r.remainingBalance)} />}
      <Rule />
      <Text center>Refund slip - please retain.</Text>
      <Footer shop={d.shop} />
    </ReceiptPage>
  )
}

/** `moneySlip` in till_print.go (a CR- receipt) */
export function MoneySlip({ d }: { d: SlipData }) {
  const m = d.money!
  return (
    <ReceiptPage dots={d.dots} columns={d.columns}>
      <Header shop={d.shop} dots={d.dots} />
      <Title>{m.title}</Title>
      <Rule />
      <Row l="Receipt:" r={m.receiptNo} />
      <Row l="Date:" r={m.date} />
      <Row l="From:" r={m.from} />
      <Row l="To:" r={m.to} />
      {m.party.trim() && <Row l="Party:" r={m.party} />}
      {m.by && <Row l="By:" r={m.by} />}
      <Rule />
      <Row bold l="Amount" r={money(d.currency, m.amount)} />
      {m.note.trim() && <Text>{`Note: ${m.note}`}</Text>}
      <Rule />
      <Text center>Signature: ____________________</Text>
      <Blank />
      <Blank />
      <Footer shop={d.shop} />
    </ReceiptPage>
  )
}

/** `escpos.DebtDocument` (a credit payment) */
export function DebtSlip({ d }: { d: SlipData }) {
  const s = d.debt!
  const sym = d.currency
  return (
    <ReceiptPage dots={d.dots} columns={d.columns}>
      <Header shop={d.shop} dots={d.dots} />
      <Title>Credit payment</Title>
      <Rule />
      <Row l="Receipt:" r={s.receiptNo} />
      <Row l="Date:" r={s.date} />
      <Row l="Customer:" r={s.customer} />
      {s.phone && <Row l="Phone:" r={s.phone} />}
      <Rule />
      <Row bold l="Amount paid" r={money(sym, s.amount)} />
      <Row l="Method:" r={s.method} />
      {s.balanceAfter && (
        <>
          <Rule />
          <Row l="Previous balance" r={money(sym, s.balanceBefore)} />
          <Row bold l="Remaining balance" r={money(sym, s.balanceAfter)} />
        </>
      )}
      <Rule />
      {s.cashier && <Text center>{`Served by: ${s.cashier}`}</Text>}
      <Footer shop={d.shop} />
    </ReceiptPage>
  )
}

/** `escpos.WarrantyDocument` (a serial replacement) */
export function WarrantySlip({ d }: { d: SlipData }) {
  const w = d.warranty!
  return (
    <ReceiptPage dots={d.dots} columns={d.columns}>
      <Header shop={d.shop} dots={d.dots} />
      <Title>Warranty replacement</Title>
      <Rule />
      <Row l="Date:" r={w.date} />
      {w.customer && <Row l="Customer:" r={w.customer} />}
      <Text>{`Product: ${w.product}`}</Text>
      <Rule />
      <Text>{`Returned serial: ${w.oldSerial}`}</Text>
      <Text bold>{`New serial: ${w.newSerial}`}</Text>
      {w.warrantyUntil && <Row l="Warranty until:" r={w.warrantyLeft ? `${w.warrantyUntil} (${w.warrantyLeft})` : w.warrantyUntil} />}
      <Rule />
      <Text center>Replacement slip - please retain.</Text>
      <Footer shop={d.shop} />
    </ReceiptPage>
  )
}

/** `escpos.TestDocument`: no header or footer — just enough to prove the wiring. */
export function TestSlip({ d }: { d: SlipData }) {
  return (
    <ReceiptPage dots={d.dots} columns={d.columns}>
      <div style={{ fontWeight: 700, fontSize: 40, lineHeight: '48px', height: 48, textAlign: 'center', whiteSpace: 'pre' }}>{d.shop.name.trim() ? d.shop.name : 'POS'}</div>
      <Blank />
      <Text center>*** PRINTER TEST ***</Text>
      <Blank />
      <Rule />
      <Row l="Width:" r={`${d.width}mm (${d.columns} cols)`} />
      <Row l="Printed:" r={d.test!.printed} />
      <Rule />
      <Text center>If you can read this,</Text>
      <Text center>your printer is set up.</Text>
    </ReceiptPage>
  )
}
