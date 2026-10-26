import { SANS_FAMILY } from '../../paper'
import type { LabelTemplateProps, PriceLabel } from '../kit'
import { isBlank } from './hero'
import { BarsAt, Digits, Text, scaleOf } from './parts'

/** First text on top, the bars in the middle, the second text in a black band
 * along the bottom (white on black); no second text → no band, taller bars. */
export function Inverse({ d, paper }: LabelTemplateProps<PriceLabel>) {
  const k = scaleOf(paper)
  const band = !isBlank(d.second)
  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', fontFamily: SANS_FAMILY, textAlign: 'center' }}>
      <Text d={d} slot={d.first} px={18 * k} weight={700} style={{ padding: `${8 * k}px ${14 * k}px 0` }} />
      <div style={{ flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column', justifyContent: 'center', alignItems: 'center', padding: `${6 * k}px ${16 * k}px ${4 * k}px` }}>
        <BarsAt d={d} width={paper.w - 32 * k} height={Math.round((band ? 66 : 104) * k)} />
        <Digits d={d} px={Math.round(12 * k)} style={{ marginTop: 2 * k }} />
      </div>
      {band && (
        <div data-band="" style={{ background: '#000', color: '#fff', padding: `${6 * k}px ${12 * k}px`, minHeight: 52 * k, boxSizing: 'border-box', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          <Text d={d} slot={d.second} px={32 * k} weight={900} />
        </div>
      )}
    </div>
  )
}
