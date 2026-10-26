import type { CSSProperties } from 'react'
import { BAR_QUIET, Bars, MIN_MODULE, QR_QUIET, barModules, fitModule, qrSize } from '../codes'
import { Fit, sizeScale } from '../fit'
import type { LabelPaper, PriceLabel, Slot } from '../kit'
import { isBlank, splitPrice } from './hero'

/** A style's scale: 1 on a 50×25 sticker, bigger on bigger paper, smaller on narrow. */
export const scaleOf = (p: LabelPaper) => Math.min(p.h / 200, p.w / 400)

/** Lines + size setting of the slot (first or second). */
export const fmt = (d: PriceLabel, s: Slot) => (s === d.first ? { lines: d.firstLines, scale: sizeScale(d.firstSize) } : { lines: d.secondLines, scale: sizeScale(d.secondSize) })

/** One of the two texts at `px` (before the shop's size setting), cut after its lines. */
export function Text({ d, slot, px, style, weight = 600 }: { d: PriceLabel; slot: Slot; px: number; style?: CSSProperties; weight?: number }) {
  if (isBlank(slot)) return null
  const f = fmt(d, slot)
  return (
    <Fit lines={f.lines} data-slot={slot.key} style={{ fontSize: Math.round(px * f.scale), lineHeight: 1.15, fontWeight: weight, ...style }}>
      {slot.text}
    </Fit>
  )
}

/** The big text: a price as small currency + huge figures + small cents; anything else just big. */
export function Hero({ d, slot, px, style }: { d: PriceLabel; slot: Slot; px: number; style?: CSSProperties }) {
  if (isBlank(slot)) return null
  const f = fmt(d, slot)
  const big = Math.round(px * f.scale)
  const p = slot.key === 'price' ? splitPrice(slot.text) : null
  return (
    <Fit lines={p ? 1 : f.lines} data-slot={slot.key} data-hero="" style={{ fontSize: big, lineHeight: 1.05, fontWeight: 900, letterSpacing: -big / 34, ...style }}>
      {p ? (
        <>
          {p.before && <span style={{ fontSize: big / 3, fontWeight: 700, letterSpacing: 0, marginRight: big / 12 }}>{p.before}</span>}
          {p.whole}
          {p.after && <span style={{ fontSize: big / 3, fontWeight: 700, letterSpacing: 0 }}>{p.after}</span>}
        </>
      ) : slot.text}
    </Fit>
  )
}

/** The widest whole-dot module (2–3) that fits the code in `width`; 0 = it can't. */
export const moduleFor = (d: PriceLabel, width: number, max = 3) => {
  const m = barModules(d.code, d.format)
  return m ? fitModule(m.length, BAR_QUIET, width, max) : 0
}

/** The bars at whole dots, `height` tall, or nothing when the code can't fit / isn't valid. */
export function BarsAt({ d, width, height, max }: { d: PriceLabel; width: number; height: number; max?: number }) {
  const module = moduleFor(d, width, max)
  return module ? <Bars code={d.code} format={d.format} module={module} height={height} /> : null
}

/** The code's digits, in the paper's monospace. */
export function Digits({ d, px, style }: { d: PriceLabel; px: number; style?: CSSProperties }) {
  return <div data-digits="" style={{ fontFamily: '"JetBrains Mono", monospace', fontSize: px, lineHeight: 1.1, letterSpacing: 1, whiteSpace: 'pre', ...style }}>{d.code}</div>
}

/** Fits when the bars fit the paper less the side padding at 2 dots a module. */
export const barsFit = (pad: number) => (d: PriceLabel, p: LabelPaper) => !d.code || moduleFor(d, p.w - 2 * pad * scaleOf(p)) > 0

/** Bars with their digits beside them (split over two lines) when both fit
 * `width`, else the digits under full-width bars. */
export function BarRow({ d, width, height, digitsPx, gap }: { d: PriceLabel; width: number; height: number; digitsPx: number; gap: number }) {
  const digitsW = digitsPx * 0.62 * Math.ceil(d.code.length / 2) + gap
  if (moduleFor(d, width - digitsW)) {
    const half = Math.ceil(d.code.length / 2)
    return (
      <div style={{ display: 'flex', alignItems: 'flex-end', gap }}>
        <BarsAt d={d} width={width - digitsW} height={height} />
        <Digits d={{ ...d, code: `${d.code.slice(0, half)}\n${d.code.slice(half)}` }} px={digitsPx} />
      </div>
    )
  }
  return (
    <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
      <BarsAt d={d} width={width} height={height - digitsPx} />
      <Digits d={d} px={digitsPx} />
    </div>
  )
}

/** The QR module (dots) that fits the code's QR, quiet zone included, in `side`
 * dots square (capped at `max`); 0 when even 2 dots doesn't fit. */
export const qrModuleFor = (d: PriceLabel, side: number, max = 8) => {
  if (!d.code) return 0
  const m = Math.min(max, Math.floor(side / (qrSize(d.code) + 2 * QR_QUIET)))
  return m >= MIN_MODULE ? m : 0
}
