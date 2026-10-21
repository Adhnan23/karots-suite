import type { SlipData, SlipItem, SlipSale } from '@/api/types.gen'
import type { SlipKit } from '../styles/kit'
import { abs, display, money, neg, pos } from '../styles/money'

/** The sale receipt, line for line from `escpos.Document` + `printItems`. */
export function SaleSlip({ d, kit: K }: { d: SlipData; kit: SlipKit }) {
  const s = d.sale!
  return (
    <K.Page d={d}>
      <K.Header shop={d.shop} dots={d.dots} number={s.receiptNo} />
      <K.Rule />
      <SaleHead s={s} K={K} />
      <K.Rule />
      <SaleItems s={s} K={K} />
      <K.Rule />
      <SaleMoney s={s} sym={d.currency} K={K} />
      <K.Rule />
      <K.Footer shop={d.shop} number={s.receiptNo} />
    </K.Page>
  )
}

/** Receipt no., date, cashier, customer. */
export function SaleHead({ s, K }: { s: SlipSale; K: SlipKit }) {
  return (
    <>
      <K.Info l="Receipt:" r={s.receiptNo} />
      <K.Info l="Date:" r={s.date} />
      <K.Info l="Cashier:" r={s.cashier} />
      {s.customer && <K.Info l="Customer:" r={s.customer} />}
    </>
  )
}

/** Every line, groups as one block. */
export function SaleItems({ s, K }: { s: SlipSale; K: SlipKit }) {
  return <>{(s.items ?? []).map((it, i) => (it.group ? <Group key={i} it={it} K={K} /> : <Item key={i} it={it} K={K} />))}</>
}

/** Subtotal down to the rounding: savings, tax, payouts, TOTAL, tenders, the credit block or the change. */
export function SaleMoney({ s, sym, K }: { s: SlipSale; sym: string; K: SlipKit }) {
  const payments = s.payments ?? []
  return (
    <>
      <K.Money l="Subtotal" r={money(sym, s.subtotal)} />
      {pos(s.itemSavings) && <K.Money saving l="Item savings" r={'-' + money(sym, s.itemSavings)} />}
      {pos(s.billSaving) && <K.Money saving l={'Bill saving' + s.billSavingLabel} r={'-' + money(sym, s.billSaving)} />}
      {pos(s.tax) && <K.Money l="Tax" r={money(sym, s.tax)} />}
      {pos(s.given) && <K.Money l="Given to customer" r={'-' + money(sym, s.given)} />}
      <K.Gap />
      {!neg(s.total) && <K.Total label="TOTAL" amount={money(sym, s.total)} />}
      {pos(s.youSaved) && <K.Money saving l="You saved" r={money(sym, s.youSaved)} />}
      {payments.length > 0 && (
        <>
          <K.Gap />
          {payments.filter((p) => Number(p.amount) !== 0).map((p, i) => <K.Money key={i} l={p.method} r={money(sym, p.amount)} />)}
        </>
      )}
      {s.credit ? (
        <>
          <K.Gap />
          {pos(s.fromAdvance) && <K.Money l="From advance" r={'-' + money(sym, s.fromAdvance)} />}
          {pos(s.due) && <K.Total label="DUE" amount={money(sym, s.due)} />}
          {neg(s.balance) ? <K.Total label="ADVANCE LEFT" amount={money(sym, abs(s.balance))} /> : <K.Total label="TOTAL DUE" amount={money(sym, s.balance)} />}
        </>
      ) : (
        pos(s.change) && (
          <>
            <K.Gap />
            <K.Total label={neg(s.total) ? 'GIVEN' : 'CHANGE'} amount={money(sym, s.change)} />
          </>
        )
      )}
      {pos(s.onAccount) && <K.Money l="On account" r={money(sym, s.onAccount)} />}
      {pos(s.changeKept) && <K.Money l="Change kept" r={money(sym, s.changeKept)} />}
    </>
  )
}

/** A line's warranty and serial numbers (`escpos.extras`). */
export function Extras({ warranty, serials, level, K }: { warranty: string; serials?: string[]; level: 1 | 2; K: SlipKit }) {
  return (
    <>
      {warranty && (
        <K.Note level={level} kind="warranty">
          {warranty}
        </K.Note>
      )}
      {(serials ?? []).map((sn, i) => (
        <K.Note key={i} level={level} kind="serial">
          {sn}
        </K.Note>
      ))}
    </>
  )
}

/** A line's quantity text, as Classic prints it. */
export function qtyText(it: SlipItem) {
  if (it.group) return `${it.sets} x ${display(it.setGross)}` + (pos(it.discount) ? ` -> ${display(it.setNet)}` : '')
  if (it.packLabel) return `${display(it.quantity)} ${it.unitAbbr} (${it.packLabel})`
  return `${display(it.quantity)} ${it.unitAbbr} x ${display(it.unitPrice)}` + (pos(it.discount) ? ` -> ${display(it.netUnitPrice)}` : '')
}

function Item({ it, K }: { it: SlipItem; K: SlipKit }) {
  if (it.payout) {
    return (
      <K.Item name={it.name} amount={'-' + display(abs(it.subtotal))}>
        <K.Sub kind="given" l="Given" r={'-' + display(abs(it.subtotal))} />
        <Extras warranty={it.warranty} serials={it.serials} level={1} K={K} />
      </K.Item>
    )
  }
  const disc = pos(it.discount)
  return (
    <K.Item name={it.name} amount={display(it.subtotal)}>
      <K.Sub kind="qty" l={qtyText(it)} r={display(it.gross)} />
      {disc && (
        <>
          <K.Sub kind="saving" l={'You saved' + it.discountLabel} r={'-' + display(it.discount)} />
          <K.Sub kind="total" l="Line total" r={display(it.subtotal)} />
        </>
      )}
      <Extras warranty={it.warranty} serials={it.serials} level={1} K={K} />
    </K.Item>
  )
}

/** A line group as one block (`escpos.printGroup`). */
function Group({ it, K }: { it: SlipItem; K: SlipKit }) {
  const disc = pos(it.discount)
  return (
    <K.Item name={it.name} amount={display(it.subtotal)}>
      <K.Sub kind="qty" l={qtyText(it)} r={display(it.gross)} />
      {disc && (
        <>
          <K.Sub kind="saving" l="You saved" r={'-' + display(it.discount)} />
          <K.Sub kind="total" l="Line total" r={display(it.subtotal)} />
        </>
      )}
      {(it.members ?? []).map((m, i) => (
        <div key={i}>
          <K.Member>{`${m.name} x${m.quantity}`}</K.Member>
          <Extras warranty={m.warranty} serials={m.serials} level={2} K={K} />
        </div>
      ))}
    </K.Item>
  )
}
