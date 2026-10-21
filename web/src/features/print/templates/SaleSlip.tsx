import type { SlipData, SlipItem } from '@/api/types.gen'
import { abs, Blank, display, Footer, Hang, Header, money, neg, pos, ReceiptPage, Row, Rule, Text } from '../kit'

/** The sale receipt, line for line from `escpos.Document` + `printItems`. */
export function SaleSlip({ d }: { d: SlipData }) {
  const s = d.sale!
  const sym = d.currency
  const payments = s.payments ?? []
  return (
    <ReceiptPage dots={d.dots} columns={d.columns}>
      <Header shop={d.shop} dots={d.dots} />
      <Rule />
      <Row l="Receipt:" r={s.receiptNo} />
      <Row l="Date:" r={s.date} />
      <Row l="Cashier:" r={s.cashier} />
      {s.customer && <Row l="Customer:" r={s.customer} />}
      <Rule />
      {(s.items ?? []).map((it, i) => (it.group ? <Group key={i} it={it} /> : <Item key={i} it={it} />))}
      <Rule />
      <Row l="Subtotal" r={money(sym, s.subtotal)} />
      {pos(s.itemSavings) && <Row l="Item savings" r={'-' + money(sym, s.itemSavings)} />}
      {pos(s.billSaving) && <Row l={'Bill saving' + s.billSavingLabel} r={'-' + money(sym, s.billSaving)} />}
      {pos(s.tax) && <Row l="Tax" r={money(sym, s.tax)} />}
      {pos(s.given) && <Row l="Given to customer" r={'-' + money(sym, s.given)} />}
      <Blank />
      {!neg(s.total) && <Row big l="TOTAL" r={money(sym, s.total)} />}
      {pos(s.youSaved) && <Row l="You saved" r={money(sym, s.youSaved)} />}
      {payments.length > 0 && (
        <>
          <Blank />
          {payments.filter((p) => Number(p.amount) !== 0).map((p, i) => <Row key={i} l={p.method} r={money(sym, p.amount)} />)}
        </>
      )}
      {s.credit ? (
        <>
          <Blank />
          {pos(s.fromAdvance) && <Row l="From advance" r={'-' + money(sym, s.fromAdvance)} />}
          {pos(s.due) && <Row big l="DUE" r={money(sym, s.due)} />}
          {neg(s.balance) ? <Row big l="ADVANCE LEFT" r={money(sym, abs(s.balance))} /> : <Row big l="TOTAL DUE" r={money(sym, s.balance)} />}
        </>
      ) : (
        pos(s.change) && (
          <>
            <Blank />
            <Row big l={neg(s.total) ? 'GIVEN' : 'CHANGE'} r={money(sym, s.change)} />
          </>
        )
      )}
      {pos(s.onAccount) && <Row l="On account" r={money(sym, s.onAccount)} />}
      {pos(s.changeKept) && <Row l="Change kept" r={money(sym, s.changeKept)} />}
      <Rule />
      <Footer shop={d.shop} />
    </ReceiptPage>
  )
}

/** A line's warranty and serial numbers (`escpos.extras`). */
function Extras({ warranty, serials, indent }: { warranty: string; serials?: string[]; indent: string }) {
  return (
    <>
      {warranty && <Text>{indent + warranty}</Text>}
      {(serials ?? []).map((sn, i) => <Text key={i} indent={indent.length}>{sn}</Text>)}
    </>
  )
}

function Item({ it }: { it: SlipItem }) {
  if (it.payout) {
    return (
      <>
        <Text>{it.name}</Text>
        <Row l="  Given" r={'-' + display(abs(it.subtotal))} />
        <Extras warranty={it.warranty} serials={it.serials} indent="  " />
      </>
    )
  }
  const disc = pos(it.discount)
  let qty = `${display(it.quantity)} ${it.unitAbbr} x ${display(it.unitPrice)}`
  if (it.packLabel) qty = `${display(it.quantity)} ${it.unitAbbr} (${it.packLabel})`
  else if (disc) qty += ` -> ${display(it.netUnitPrice)}`
  return (
    <>
      <Text>{it.name}</Text>
      <Row l={'  ' + qty} r={display(it.gross)} />
      {disc && (
        <>
          <Row l={'  You saved' + it.discountLabel} r={'-' + display(it.discount)} />
          <Row l="  Line total" r={display(it.subtotal)} />
        </>
      )}
      <Extras warranty={it.warranty} serials={it.serials} indent="  " />
    </>
  )
}

/** A line group as one block (`escpos.printGroup`). */
function Group({ it }: { it: SlipItem }) {
  const disc = pos(it.discount)
  const qty = `${it.sets} x ${display(it.setGross)}` + (disc ? ` -> ${display(it.setNet)}` : '')
  return (
    <>
      <Text>{it.name}</Text>
      <Row l={'  ' + qty} r={display(it.gross)} />
      {disc && (
        <>
          <Row l="  You saved" r={'-' + display(it.discount)} />
          <Row l="  Line total" r={display(it.subtotal)} />
        </>
      )}
      {(it.members ?? []).map((m, i) => (
        <div key={i}>
          <Hang lead="   - ">{`${m.name} x${m.quantity}`}</Hang>
          <Extras warranty={m.warranty} serials={m.serials} indent="     " />
        </div>
      ))}
    </>
  )
}
