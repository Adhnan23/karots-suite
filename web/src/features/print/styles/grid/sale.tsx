import type { CSSProperties } from 'react'
import type { SlipData, SlipItem } from '@/api/types.gen'
import { SaleHead, SaleMoney } from '../../templates/SaleSlip'
import type { SlipKit } from '../kit'
import { abs, display, pos } from '../money'
import { pretty } from '../parts'
import { small } from './kit'

const td: CSSProperties = { padding: '4px 4px', verticalAlign: 'top' }
const num: CSSProperties = { ...td, textAlign: 'right', whiteSpace: 'nowrap' }

/** Grid's sale: the lines as a QTY / ITEM / OFF / AMT table, totals on the right. */
export function GridSale({ d, kit: K }: { d: SlipData; kit: SlipKit }) {
  const s = d.sale!
  const wide = d.dots >= 576
  return (
    <K.Page d={d}>
      <K.Header shop={d.shop} dots={d.dots} number={s.receiptNo} />
      <div style={{ marginTop: 6 }}>
        <SaleHead s={s} K={K} />
      </div>
      <table style={{ width: '100%', borderCollapse: 'collapse', marginTop: 8, fontSize: 18, lineHeight: '23px' }}>
        <thead>
          <tr style={{ background: '#000', color: '#fff', fontWeight: 700 }}>
            <td style={td}>QTY</td>
            <td style={td}>ITEM</td>
            {wide && <td style={num}>OFF</td>}
            <td style={num}>AMT</td>
          </tr>
        </thead>
        <tbody>
          {(s.items ?? []).map((it, i) => (
            <Row key={i} it={it} wide={wide} last={i === (s.items ?? []).length - 1} />
          ))}
        </tbody>
      </table>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 8 }}>
        <div style={{ width: wide ? '68%' : '100%' }}>
          <SaleMoney s={s} sym={d.currency} K={K} />
        </div>
      </div>
      <K.Footer shop={d.shop} number={s.receiptNo} />
    </K.Page>
  )
}

function Row({ it, last, wide }: { it: SlipItem; last: boolean; wide: boolean }) {
  const qty = it.payout ? '' : it.group ? it.sets : display(it.quantity).replace(/\.00$/, '')
  const subs: string[] = []
  // The price before the saving, only when it differs from AMT.
  const gross = pos(it.discount) ? ` (was ${display(it.gross)})` : ''
  if (it.payout) subs.push('Given')
  else if (it.group) subs.push(pretty(`${it.sets} x ${display(it.setGross)}` + (pos(it.discount) ? ` -> ${display(it.setNet)}` : '')) + gross)
  else if (it.packLabel) subs.push(`${it.unitAbbr} · ${it.packLabel}` + gross)
  else subs.push(`${it.unitAbbr} @ ${display(it.unitPrice)}` + gross)
  // No OFF column on 58 mm: the saving goes under the name.
  if (!wide && pos(it.discount)) subs.push(`saved ${display(it.discount)}${it.discountLabel}`)
  for (const m of it.members ?? []) {
    subs.push(`· ${pretty(`${m.name} x${m.quantity}`)}`)
    if (m.warranty) subs.push(`  ${m.warranty}`)
    for (const sn of m.serials ?? []) subs.push(`  SN ${sn}`)
  }
  if (it.warranty) subs.push(it.warranty)
  for (const sn of it.serials ?? []) subs.push(`SN ${sn}`)
  return (
    <tr style={{ borderBottom: `${last ? 4 : 2}px solid #000` }}>
      <td style={td}>{qty}</td>
      <td style={{ ...td, overflowWrap: 'anywhere' }}>
        <b>{it.name}</b>
        {subs.map((t, i) => (
          <div key={i} style={{ ...small, whiteSpace: 'pre-wrap' }}>
            {t}
          </div>
        ))}
      </td>
      {wide && (
        <td style={num}>
          {pos(it.discount) && (
            <>
              <div>{display(it.discount)}</div>
              {it.discountLabel && <div style={small}>{it.discountLabel.trim().replace(/[()]/g, '')}</div>}
            </>
          )}
        </td>
      )}
      <td style={{ ...num, fontWeight: 700 }}>{it.payout ? '-' + display(abs(it.subtotal)) : display(it.subtotal)}</td>
    </tr>
  )
}
