import { PAPER_FAMILY } from '../../paper'
import { BAR_QUIET, Bars, barModules, fitModule } from '../codes'
import { Fit } from '../fit'
import type { LabelTemplateProps, PriceLabel, Slot } from '../kit'

// Today's Classic label (internal/tspl LayoutOf) as a picture: same margins, the
// printer font sizes as a monospace of the same column width, the barcode filling
// the middle with its digits under it.
const MARGIN_X = 16
const MARGIN_Y = 12
const READABLE_H = 26
/** TSPL font token → per-character width and line height (dots). */
const FONT: Record<string, [number, number]> = { '1': [8, 18], '2': [12, 26], '3': [16, 30], '4': [24, 40] }

function Line({ slot, lines, size, def }: { slot: Slot; lines: number; size: string; def: string }) {
  if (slot.key === 'none' || !slot.text.trim()) return null
  const [charW, lineH] = FONT[size] ?? FONT[def]
  return (
    <Fit lines={lines} data-slot={slot.key} style={{ fontSize: charW / 0.6, lineHeight: `${lineH}px`, textAlign: 'center', fontWeight: 700, flex: 'none' }}>
      {slot.text}
    </Fit>
  )
}

export function ClassicPrice({ d, paper }: LabelTemplateProps<PriceLabel>) {
  const mods = barModules(d.code, d.format)
  const module = mods ? fitModule(mods.length, BAR_QUIET, paper.w - 2 * MARGIN_X, 2) || 2 : 0
  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', padding: `${MARGIN_Y}px ${MARGIN_X}px`, boxSizing: 'border-box', fontFamily: PAPER_FAMILY }}>
      <Line slot={d.first} lines={d.firstLines} size={d.firstSize} def="2" />
      {mods && (
        <div style={{ flex: 1, minHeight: 40, display: 'flex', flexDirection: 'column', alignItems: 'center', overflow: 'hidden' }}>
          <div style={{ flex: 1, minHeight: 0, display: 'flex' }}><Bars code={d.code} format={d.format} module={module} height={200} stretch /></div>
          <div data-digits="" style={{ height: READABLE_H, fontSize: 18, lineHeight: `${READABLE_H}px`, letterSpacing: 2, flex: 'none' }}>{d.code}</div>
        </div>
      )}
      {!mods && <div style={{ flex: 1 }} />}
      <Line slot={d.second} lines={d.secondLines} size={d.secondSize} def="3" />
    </div>
  )
}
