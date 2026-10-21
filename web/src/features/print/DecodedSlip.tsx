import type { CSSProperties } from 'react'
import type { SlipLine, SlipPreview } from '@/api/types.gen'
import { PAPER_FAMILY } from './paper'

// Printer geometry in dots (1 CSS px = 1 dot): Font A is 12 × 24, Font B 9 × 17.
// JetBrains Mono advances 0.6 em, so 20 px text = 12 px columns.
const A = { size: 20, line: 24 }
const B = { size: 15, line: 17 }

/** A Classic slip's decoded lines drawn at printer dots — the page Image mode
 * prints for a slip kind that has no React template of its own. Black on white:
 * it's paper, not themed UI. */
export function DecodedSlip({ preview }: { preview: SlipPreview }) {
  return (
    <div style={{ width: preview.dots, background: '#fff', color: '#000', fontFamily: PAPER_FAMILY, fontVariantLigatures: 'none' }}>
      {preview.lines.map((l, i) => <Line key={i} l={l} />)}
    </div>
  )
}

function Line({ l }: { l: SlipLine }) {
  const textAlign = l.align === 'center' ? 'center' : l.align === 'right' ? 'right' : 'left'
  if (l.image) {
    const margin = textAlign === 'center' ? '0 auto' : textAlign === 'right' ? '0 0 0 auto' : undefined
    return <img src={l.image} alt="" style={{ display: 'block', width: l.imageDots, margin, imageRendering: 'pixelated' }} />
  }
  const f = l.small ? B : A
  // Every line is an exact number of dot rows (no inline-box rounding). Double
  // width doubles the advance (a bigger font); double height alone stretches the
  // glyphs over two rows.
  const rows = f.line * (l.tall ? 2 : 1)
  const style: CSSProperties = { textAlign, whiteSpace: 'pre', fontWeight: l.bold ? 700 : 400, height: rows, overflow: 'hidden',
    fontSize: l.wide ? f.size * 2 : f.size, lineHeight: `${l.wide ? rows : f.line}px` }
  const text = l.text || ' '
  return (
    <div style={style}>
      {l.tall && !l.wide ? <span style={{ display: 'block', transform: 'scaleY(2)', transformOrigin: 'top' }}>{text}</span> : text}
    </div>
  )
}
