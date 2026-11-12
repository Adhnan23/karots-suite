import type { CSSProperties } from 'react'
import type { SlipData, SlipItem } from '@/api/types.gen'
import { packSaving, SaleMoney } from '../../templates/SaleSlip'
import type { SlipKit } from '../kit'
import { abs, display, pos, qty } from '../money'
import { LR, pretty } from '../parts'
import { small } from './kit'

const td: CSSProperties = { padding: '1px 0 1px 8px', verticalAlign: 'top', textAlign: 'right', whiteSpace: 'nowrap' }
const name: CSSProperties = { padding: '1px 0', verticalAlign: 'top', overflowWrap: 'anywhere', width: '100%' }

/** Compact's sale: number and date on one row, one row per product (name · qty · single price · amount). */
export function CompactSale({ d, kit: K }: { d: SlipData; kit: SlipKit }) {
  const s = d.sale!
  return (
    <K.Page d={d}>
      <K.Header shop={d.shop} dots={d.dots} />
      <K.Rule />
      <LR drop l={<b>{s.receiptNo}</b>} r={s.date} style={small} />
      <LR drop l={`Cashier: ${s.cashier}`} r={s.customer ? `Customer: ${s.customer}` : ''} style={small} />
      <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: 4 }}>
        <thead>
          <tr style={{ ...small, fontWeight: 700, borderBottom: '2px solid #000' }}>
            <td style={name}>Item</td>
            <td style={td}>Qty</td>
            <td style={td}>Price</td>
            <td style={td}>Amount</td>
          </tr>
        </thead>
        <tbody>
          {(s.items ?? []).map((it, i) => (
            <Rows key={i} it={it} />
          ))}
        </tbody>
      </table>
      <K.Rule />
      <SaleMoney s={s} sym={d.currency} K={K} />
      <K.Rule />
      <K.Footer shop={d.shop} />
    </K.Page>
  )
}

/** A product's row, then a small line only for what it has: a saving, a group's members, a warranty or serials. */
function Rows({ it }: { it: SlipItem }) {
  const extra: [string, string][] = []
  const pack = packSaving(it)
  if (pack > 0) extra.push(['Pack price', '-' + display(String(pack))])
  if (pos(it.discount)) extra.push([`You saved${it.discountLabel} (was ${display(pack > 0 ? String(pack + Number(it.gross)) : it.gross)})`, '-' + display(it.discount)])
  for (const m of it.members ?? []) {
    extra.push([`· ${pretty(`${m.name} x${qty(m.quantity)}`)}`, it.itemised ? display(m.subtotal) : ''])
    if (it.itemised && pos(m.discount)) extra.push(['  You saved', '-' + display(m.discount)])
    if (m.warranty) extra.push([`  ${m.warranty}`, ''])
    for (const sn of m.serials ?? []) extra.push([`  ${sn}`, ''])
  }
  if (it.warranty) extra.push([it.warranty, ''])
  for (const sn of it.serials ?? []) extra.push([sn, ''])
  const count = it.payout ? '' : it.group ? it.sets : qty(it.quantity)
  const price = it.payout ? 'Given' : it.group ? (it.itemised ? '' : display(it.setGross)) : display(it.unitPrice)
  return (
    <>
      <tr>
        <td style={name}>{it.name}</td>
        <td style={td}>{count}</td>
        <td style={td}>{price}</td>
        <td style={{ ...td, fontWeight: 700 }}>{it.payout ? '-' + display(abs(it.subtotal)) : display(it.subtotal)}</td>
      </tr>
      {extra.length > 0 && (
        <tr>
          <td colSpan={4} style={{ ...small, padding: '0 0 2px 12px' }}>
            {extra.map(([l, r], i) => (
              <LR key={i} l={<span style={{ whiteSpace: 'pre-wrap' }}>{l}</span>} r={r} />
            ))}
          </td>
        </tr>
      )}
    </>
  )
}
