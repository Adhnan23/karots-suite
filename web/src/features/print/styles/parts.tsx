import type { CSSProperties, ReactNode } from 'react'
import type { SlipShop } from '@/api/types.gen'
import { Code128 } from '../Barcode'

// Building blocks the new styles share. Paper is 1-bit: black or white only —
// rules are solid borders at whole dots, emphasis is a black band, never grey.

/** One Font A column of white each side (12 dots), the margin every style keeps. */
export const MARGIN = 12

export function Paper({ dots, family, size = 20, line = 26, children }: { dots: number; family: string; size?: number; line?: number; children: ReactNode }) {
  return (
    <div
      style={{
        // flow-root: a child's top / bottom margin stays inside the page. Collapsed through it,
        // the measured height would miss the margin and the picture would lose its last dots.
        display: 'flow-root',
        width: dots,
        boxSizing: 'border-box',
        padding: `0 ${MARGIN}px`,
        background: '#fff',
        color: '#000',
        fontFamily: family,
        fontSize: size,
        lineHeight: `${line}px`,
        fontVariantNumeric: 'tabular-nums',
        fontVariantLigatures: 'none',
      }}
    >
      {children}
    </div>
  )
}

/** Left text that wraps, right text that never does. `drop`: a label / value
 * line — when both don't fit, the value moves under the label (right-aligned)
 * instead of the label being split mid-word. */
export function LR({ l, r, style, drop }: { l: ReactNode; r?: ReactNode; style?: CSSProperties; drop?: boolean }) {
  return (
    <div style={{ display: 'flex', flexWrap: drop ? 'wrap' : undefined, justifyContent: 'space-between', alignItems: 'baseline', columnGap: 12, ...style }}>
      <span style={{ minWidth: 0, overflowWrap: drop ? 'break-word' : 'anywhere' }}>{l}</span>
      {r !== undefined && r !== '' && <span style={{ flexShrink: 0, whiteSpace: 'nowrap', marginLeft: 'auto' }}>{r}</span>}
    </div>
  )
}

/** Name ……… amount: a dotted leader between them. */
export function Leader({ l, r, style }: { l: ReactNode; r: ReactNode; style?: CSSProperties }) {
  return (
    <div style={{ display: 'flex', alignItems: 'baseline', gap: 6, ...style }}>
      <span style={{ minWidth: 0, overflowWrap: 'anywhere' }}>{l}</span>
      <span style={{ flex: '1 0 16px', borderBottom: '2px dotted #000', transform: 'translateY(-6px)' }} />
      <span style={{ flexShrink: 0, whiteSpace: 'nowrap' }}>{r}</span>
    </div>
  )
}

export const Line = ({ w = 2, kind = 'solid', gap = 10 }: { w?: number; kind?: 'solid' | 'dashed' | 'dotted' | 'double'; gap?: number }) => (
  <div style={{ borderTop: `${w}px ${kind} #000`, margin: `${gap}px 0` }} />
)

export const Space = ({ h = 12 }: { h?: number }) => <div style={{ height: h }} />

export function Para({ children, center, bold, style }: { children: ReactNode; center?: boolean; bold?: boolean; style?: CSSProperties }) {
  return <div style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', textAlign: center ? 'center' : 'left', fontWeight: bold ? 700 : undefined, ...style }}>{children}</div>
}

/** A small outlined tag ("−10% · 245.00"). */
export const Tag = ({ children }: { children: ReactNode }) => (
  <span style={{ display: 'inline-block', border: '2px solid #000', borderRadius: 4, padding: '0 6px', fontSize: 16, lineHeight: '22px', whiteSpace: 'nowrap' }}>{children}</span>
)

/** Classic's ASCII marks, set in type: "2 x 650.00 -> 600.00" → "2 × 650.00 → 600.00". */
export const pretty = (s: string) => s.replace(/ x /g, ' × ').replace(/ -> /g, ' → ').replace(/ x(\d)/g, ' ×$1')

/** The slip number as a scannable Code 128 with the number under it. */
export function NumberCode({ number, caption = true, height = 56 }: { number?: string; caption?: boolean; height?: number }) {
  if (!number) return null
  return (
    <div style={{ margin: '12px 0 4px' }}>
      <Code128 value={number} height={height} />
      {caption && <div style={{ textAlign: 'center', fontSize: 17, lineHeight: '22px', letterSpacing: 2 }}>{number}</div>}
    </div>
  )
}

/** The shop's logo raster, full paper width. */
export const Logo = ({ shop, dots }: { shop: SlipShop; dots: number }) =>
  shop.logo ? <img src={shop.logo} alt="" style={{ display: 'block', width: dots, margin: `0 -${MARGIN}px 8px`, imageRendering: 'pixelated' }} /> : null

/** Address · phone and VAT lines. */
export function ShopLines({ shop, style }: { shop: SlipShop; style?: CSSProperties }) {
  return (
    <>
      {shop.address && <Para center style={style}>{shop.address}</Para>}
      {shop.phone && <Para center style={style}>{`Tel: ${shop.phone}`}</Para>}
      {shop.vatNo && <Para center style={style}>{`VAT: ${shop.vatNo}`}</Para>}
    </>
  )
}

/** The Karots credit, small, every style; room under it so a serif descender never sits on the edge. */
export function Credit({ style }: { style?: CSSProperties }) {
  return (
    <div style={{ textAlign: 'center', fontSize: 15, lineHeight: '19px', paddingBottom: 6, ...style }}>
      <div>Karots Suite</div>
      <div>adhnanmsa@gmail.com | 0769626396</div>
    </div>
  )
}
