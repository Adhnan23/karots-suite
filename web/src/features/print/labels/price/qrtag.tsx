import { SANS_FAMILY } from '../../paper'
import { Qr } from '../codes'
import type { LabelPaper, LabelTemplateProps, PriceLabel } from '../kit'
import { hero } from './hero'
import { Digits, Hero, Text, qrModuleFor, scaleOf } from './parts'

const PAD = 14

/** The QR's square side on this paper: the label's height less the padding, at most 45% of its width. */
const qrSide = (p: LabelPaper) => Math.min(p.h - 2 * PAD * scaleOf(p), p.w * 0.45)

/** Fits when the QR keeps 2-dot modules. */
export const qrTagFits = (d: PriceLabel, p: LabelPaper) => !d.code || qrModuleFor(d, qrSide(p)) > 0

/** A QR (the same code as the barcode — needs a 2D scanner) on the left; the
 * texts stacked on the right in their order, the hero big under a rule; digits last. */
export function QrTag({ d, paper }: LabelTemplateProps<PriceLabel>) {
  const k = scaleOf(paper)
  const { hero: h, other, heroIsFirst } = hero(d)
  const small = <Text d={d} slot={other} px={19 * k} />
  const big = <Hero d={d} slot={h} px={40 * k} style={{ borderTop: heroIsFirst ? undefined : `${Math.max(2, Math.round(3 * k))}px solid #000`, paddingTop: heroIsFirst ? 0 : 6 * k }} />
  return (
    <div style={{ height: '100%', padding: PAD * k, boxSizing: 'border-box', fontFamily: SANS_FAMILY, display: 'flex', gap: 14 * k, alignItems: 'center' }}>
      <Qr value={d.code} module={qrModuleFor(d, qrSide(paper))} />
      <div style={{ display: 'flex', flexDirection: 'column', gap: 6 * k, minWidth: 0, flex: 1 }}>
        {heroIsFirst ? big : small}
        {heroIsFirst ? <div style={{ borderTop: `${Math.max(2, Math.round(3 * k))}px solid #000`, paddingTop: 6 * k }}>{small}</div> : big}
        <Digits d={d} px={Math.round(12 * k)} style={{ letterSpacing: 0 }} />
      </div>
    </div>
  )
}
