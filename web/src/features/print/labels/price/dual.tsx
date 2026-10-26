import { SANS_FAMILY } from '../../paper'
import { Qr, QR_QUIET, qrSize } from '../codes'
import type { LabelPaper, LabelTemplateProps, PriceLabel } from '../kit'
import { BarsAt, Digits, Text, moduleFor, qrModuleFor, scaleOf } from './parts'

const PAD = 14
const GAP = 12

/** The QR's side and the width left for the bars on this paper. */
function room(d: PriceLabel, p: LabelPaper) {
  const k = scaleOf(p)
  const qm = qrModuleFor(d, 104 * k)
  const qrSide = qm ? qm * (qrSize(d.code) + 2 * QR_QUIET) : 0
  return { qm, barsW: p.w - 2 * PAD * k - qrSide - GAP * k }
}

/** Fits when both the 1D bars and the QR keep 2-dot modules. */
export const dualFits = (d: PriceLabel, p: LabelPaper) => {
  if (!d.code) return true
  const r = room(d, p)
  return r.qm > 0 && moduleFor(d, r.barsW) > 0
}

/** The change-over look: the first text and the second (bold) across the top,
 * a normal barcode with its digits and a QR of the same code side by side —
 * every scanner reads one of them. */
export function Dual({ d, paper }: LabelTemplateProps<PriceLabel>) {
  const k = scaleOf(paper)
  const r = room(d, paper)
  return (
    <div style={{ height: '100%', padding: `${10 * k}px ${PAD * k}px`, boxSizing: 'border-box', fontFamily: SANS_FAMILY, display: 'flex', flexDirection: 'column' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', gap: 8 * k }}>
        <Text d={d} slot={d.first} px={17 * k} style={{ minWidth: 0, flex: 1 }} />
        <Text d={d} slot={d.second} px={28 * k} weight={900} style={{ textAlign: 'right', flex: 'none', maxWidth: '55%' }} />
      </div>
      <div style={{ display: 'flex', gap: GAP * k, alignItems: 'flex-end', marginTop: 'auto' }}>
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
          <BarsAt d={d} width={r.barsW} height={Math.round(80 * k)} />
          <Digits d={d} px={Math.round(12 * k)} style={{ marginTop: 3 * k }} />
        </div>
        <Qr value={d.code} module={r.qm} />
      </div>
    </div>
  )
}
