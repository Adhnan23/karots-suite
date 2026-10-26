import { SANS_FAMILY } from '../../paper'
import type { LabelTemplateProps, PriceLabel } from '../kit'
import { BarsAt, Digits, Text, scaleOf } from './parts'

/** A tag: a dashed border with a notch in each side, the first text on top, the
 * bars, then the digits and the second text (bold) on one line. */
export function Ticket({ d, paper }: LabelTemplateProps<PriceLabel>) {
  const k = scaleOf(paper)
  const inset = 8 * k
  const notch = 28 * k
  return (
    <div style={{ height: '100%', position: 'relative', fontFamily: SANS_FAMILY, boxSizing: 'border-box', padding: inset + 6 * k, display: 'flex', flexDirection: 'column' }}>
      <div style={{ position: 'absolute', inset, border: `${Math.max(2, Math.round(3 * k))}px dashed #000`, borderRadius: 4 * k }} />
      <div style={{ position: 'absolute', left: -notch / 2, top: '50%', width: notch, height: notch, marginTop: -notch / 2, borderRadius: '50%', background: '#000' }} />
      <div style={{ position: 'absolute', right: -notch / 2, top: '50%', width: notch, height: notch, marginTop: -notch / 2, borderRadius: '50%', background: '#000' }} />
      <Text d={d} slot={d.first} px={17 * k} weight={700} style={{ textAlign: 'center' }} />
      <div style={{ flex: 1, minHeight: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: `${4 * k}px ${notch}px 0` }}>
        <BarsAt d={d} width={paper.w - 2 * notch - 2 * inset} height={Math.round(64 * k)} />
      </div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 8 * k, padding: `${2 * k}px ${notch - inset}px 0` }}>
        <Digits d={d} px={Math.round(12 * k)} />
        <Text d={d} slot={d.second} px={24 * k} weight={900} style={{ textAlign: 'right', minWidth: 0 }} />
      </div>
    </div>
  )
}
