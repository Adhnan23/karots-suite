import { SANS_FAMILY } from '../../paper'
import type { LabelTemplateProps, PriceLabel } from '../kit'
import { BarRow, Text, scaleOf } from './parts'

/** Quiet: left-aligned, the first text light and the second bold, one thin rule,
 * a smaller barcode with its digits beside it (under it on narrow paper), lots of white. */
export function Minimal({ d, paper }: LabelTemplateProps<PriceLabel>) {
  const k = scaleOf(paper)
  const pad = 20 * k
  return (
    <div style={{ height: '100%', padding: `${14 * k}px ${pad}px`, boxSizing: 'border-box', fontFamily: SANS_FAMILY, display: 'flex', flexDirection: 'column' }}>
      <Text d={d} slot={d.first} px={18 * k} weight={400} />
      <Text d={d} slot={d.second} px={26 * k} weight={700} style={{ marginTop: 2 * k }} />
      <div style={{ height: Math.max(2, Math.round(2 * k)), background: '#000', margin: `${8 * k}px 0` }} />
      <div style={{ marginTop: 'auto' }}><BarRow d={d} width={paper.w - 2 * pad} height={Math.round(56 * k)} digitsPx={Math.round(12 * k)} gap={12 * k} /></div>
    </div>
  )
}
