import type { SlipData, SlipItem } from '@/api/types.gen'
import { Extras, qtyText } from '../../templates/SaleSlip'
import type { SlipKit } from '../kit'
import { abs, display, money, neg, pos } from '../money'
import { Bar } from './kit'

/** Sections' sale: RECEIPT, ITEMS, YOU SAVED (every
 * saving collected), PAYMENT (subtotal down to the change). */
export function SectionsSale({ d, kit: K }: { d: SlipData; kit: SlipKit }) {
  const s = d.sale!
  const sym = d.currency
  const items = s.items ?? []
  const savings = items.filter((it) => pos(it.discount))
  const payments = (s.payments ?? []).filter((p) => Number(p.amount) !== 0)
  return (
    <K.Page d={d}>
      <K.Header shop={d.shop} dots={d.dots} />
      <Bar>Receipt</Bar>
      <K.Info l="Receipt:" r={s.receiptNo} />
      <K.Info l="Date:" r={s.date} />
      <K.Info l="Cashier:" r={s.cashier} />
      {s.customer && <K.Info l="Customer:" r={s.customer} />}
      <Bar>Items</Bar>
      {items.map((it, i) => (
        <Line key={i} it={it} K={K} />
      ))}
      {(savings.length > 0 || pos(s.billSaving)) && (
        <>
          <Bar>You saved</Bar>
          {savings.map((it, i) => (
            <K.Money key={i} l={it.name + it.discountLabel} r={'-' + display(it.discount)} />
          ))}
          {pos(s.itemSavings) && <K.Money l="Item savings" r={'-' + money(sym, s.itemSavings)} />}
          {pos(s.billSaving) && <K.Money l={'Bill saving' + s.billSavingLabel} r={'-' + money(sym, s.billSaving)} />}
          {pos(s.youSaved) && <K.Money bold l="Saved" r={money(sym, s.youSaved)} />}
        </>
      )}
      <Bar>Payment</Bar>
      <K.Money l="Subtotal" r={money(sym, s.subtotal)} />
      {pos(s.tax) && <K.Money l="Tax" r={money(sym, s.tax)} />}
      {pos(s.given) && <K.Money l="Given to customer" r={'-' + money(sym, s.given)} />}
      {!neg(s.total) && <K.Total label="TOTAL" amount={money(sym, s.total)} />}
      {payments.map((p, i) => (
        <K.Money key={i} l={p.method} r={money(sym, p.amount)} />
      ))}
      {s.credit ? (
        <>
          {pos(s.fromAdvance) && <K.Money l="From advance" r={'-' + money(sym, s.fromAdvance)} />}
          {pos(s.due) && <K.Total label="DUE" amount={money(sym, s.due)} />}
          {neg(s.balance) ? <K.Total label="ADVANCE LEFT" amount={money(sym, abs(s.balance))} /> : <K.Total label="TOTAL DUE" amount={money(sym, s.balance)} />}
        </>
      ) : (
        pos(s.change) && <K.Total label={neg(s.total) ? 'GIVEN' : 'CHANGE'} amount={money(sym, s.change)} />
      )}
      {pos(s.onAccount) && <K.Money l="On account" r={money(sym, s.onAccount)} />}
      {pos(s.changeKept) && <K.Money l="Change kept" r={money(sym, s.changeKept)} />}
      <K.Footer shop={d.shop} number={s.receiptNo} />
    </K.Page>
  )
}

/** An item: its line total, the full price beside the quantity when a saving
 * took it down (the saving itself is in YOU SAVED). */
function Line({ it, K }: { it: SlipItem; K: SlipKit }) {
  if (it.payout) {
    return (
      <K.Item name={`${it.name} (given)`} amount={'-' + display(abs(it.subtotal))}>
        <Extras warranty={it.warranty} serials={it.serials} level={1} K={K} />
      </K.Item>
    )
  }
  return (
    <K.Item name={it.name} amount={display(it.subtotal)}>
      <K.Sub kind="qty" l={qtyText(it)} r={pos(it.discount) ? display(it.gross) : ''} />
      {(it.members ?? []).map((m, i) => (
        <div key={i}>
          <K.Member>{`${m.name} x${m.quantity}`}</K.Member>
          <Extras warranty={m.warranty} serials={m.serials} level={2} K={K} />
        </div>
      ))}
      <Extras warranty={it.warranty} serials={it.serials} level={1} K={K} />
    </K.Item>
  )
}
