import { SANS_FAMILY } from '../../paper'
import { Qr } from '../codes'
import type { LabelPaper, LabelTemplateProps, PriceLabel } from '../kit'
import { hero } from './hero'
import { BarsAt, Digits, Hero, Text, moduleFor, qrModuleFor, scaleOf } from './parts'

/** Fits when the bars and the QR keep 2-dot modules on this (wide) paper. */
export const shelfFits = (d: PriceLabel, p: LabelPaper) => {
  const k = scaleOf(p)
  return !d.code || (qrModuleFor(d, 75 * k) > 0 && moduleFor(d, p.w * 0.45) > 0)
}

const printedOn = () => new Date().toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })

/** For wide shelf-edge labels (≥ 70 mm): the other text as a heading, the hero
 * huge, a QR beside it, the barcode under it and the date it was printed (old
 * tags are easy to spot). Narrower paper draws Price Tag. */
export function Shelf({ d, paper }: LabelTemplateProps<PriceLabel>) {
  const k = scaleOf(paper)
  const { hero: h, other } = hero(d)
  return (
    <div style={{ height: '100%', padding: `${12 * k}px ${16 * k}px`, boxSizing: 'border-box', fontFamily: SANS_FAMILY, display: 'grid', gridTemplateColumns: '1fr auto', gridTemplateRows: 'auto 1fr auto', columnGap: 14 * k }}>
      <Text d={d} slot={other} px={17 * k} weight={700} style={{ gridColumn: '1 / 3' }} />
      <div style={{ alignSelf: 'center', minWidth: 0 }}><Hero d={d} slot={h} px={75 * k} /></div>
      <div style={{ alignSelf: 'center' }}><Qr value={d.code} module={qrModuleFor(d, 75 * k)} /></div>
      <div style={{ alignSelf: 'end' }}>
        <BarsAt d={d} width={paper.w * 0.45} height={Math.round(34 * k)} />
        <Digits d={d} px={Math.round(7 * k)} style={{ textAlign: 'center', marginTop: 2 * k }} />
      </div>
      <div style={{ alignSelf: 'end', textAlign: 'right', fontSize: 8 * k, lineHeight: 1.2 }}>{printedOn()}</div>
    </div>
  )
}
