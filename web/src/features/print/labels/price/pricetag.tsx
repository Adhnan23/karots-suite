import { SANS_FAMILY } from '../../paper'
import type { LabelTemplateProps, PriceLabel } from '../kit'
import { hero } from './hero'
import { BarRow, Hero, Text, scaleOf } from './parts'

/** The big one is the hero (the price, wherever it is); the bars sit under it
 * with their digits beside them; the other text keeps its place, small. */
export function PriceTag({ d, paper }: LabelTemplateProps<PriceLabel>) {
  const k = scaleOf(paper)
  const { hero: h, other, heroIsFirst } = hero(d)
  const pad = 14 * k
  const bars = <div style={{ marginTop: 4 * k }}><BarRow d={d} width={paper.w - 2 * pad} height={Math.round(54 * k)} digitsPx={Math.round(13 * k)} gap={10 * k} /></div>
  const small = <Text d={d} slot={other} px={16 * k} style={{ marginTop: heroIsFirst ? 6 * k : 0 }} />
  return (
    <div style={{ height: '100%', padding: `${10 * k}px ${pad}px`, boxSizing: 'border-box', fontFamily: SANS_FAMILY, display: 'flex', flexDirection: 'column' }}>
      {!heroIsFirst && small}
      <Hero d={d} slot={h} px={68 * k} />
      {bars}
      {heroIsFirst && small}
    </div>
  )
}
