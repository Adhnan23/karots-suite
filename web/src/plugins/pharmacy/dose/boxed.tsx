import { SANS_FAMILY } from '@/features/print/paper'
import { Fit } from '@/features/print/labels/fit'
import { INK, type LabelTemplateProps } from '@/features/print/labels/kit'
import { qtyText } from '@/lib/money'
import type { DoseLabel } from '../label'
import { calm, ClearDose, dateOf, lastLine, scale } from './clear'

const PART = String.raw`(\d+(?:\.\d+|\/\d+)?|½)`
const TIMES = new RegExp(`^\\s*${PART}\\s*-\\s*${PART}\\s*-\\s*${PART}\\s*$`)

/** "1-0-1" → ['1', '–', '1']; null when it isn't three fixed times (sos, 2-part, all 0). */
export function boxesOf(dose: string): string[] | null {
  const m = TIMES.exec(dose)
  if (!m) return null
  const parts = m.slice(1, 4).map((p) => (Number(p) === 0 ? '–' : p))
  return parts.every((p) => p === '–') ? null : parts
}

/** Morning / Noon / Night boxes with the count in each (a dashed box = none);
 * a dose with no fixed times draws as Clear. */
export function BoxedDose({ d, paper }: LabelTemplateProps<DoseLabel>) {
  const boxes = boxesOf(d.dose)
  if (!boxes) return <ClearDose d={d} paper={paper} />
  const k = scale(paper)
  const b = Math.max(2, Math.round(3 * k))
  const last = lastLine(d)
  return (
    <div style={{ height: '100%', padding: `${10 * k}px ${14 * k}px`, boxSizing: 'border-box', fontFamily: SANS_FAMILY, lineHeight: 1.2 }}>
      <Fit lines={1} style={{ fontSize: 18 * k, fontWeight: 700 }}>{calm(d.drug)} × {qtyText(d.qty)}</Fit>
      <Fit lines={1} style={{ fontSize: 15 * k }}>{[d.patient && calm(d.patient, true), dateOf(d)].filter(Boolean).join(' · ')}</Fit>
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 8 * k, marginTop: 6 * k }}>
        {['Morning', 'Noon', 'Night'].map((t, i) => (
          <div key={t} data-box="" style={{ border: `${b}px ${boxes[i] === '–' ? 'dashed' : 'solid'} ${INK}`, borderRadius: 6 * k, textAlign: 'center', padding: `${3 * k}px 0` }}>
            <div style={{ fontSize: 14 * k, fontWeight: 600 }}>{t}</div>
            <div style={{ fontSize: 30 * k, fontWeight: 900, lineHeight: 1.05 }}>{boxes[i]}</div>
          </div>
        ))}
      </div>
      {last && <Fit lines={1} style={{ fontSize: 16 * k, fontWeight: 600, textAlign: 'center', marginTop: 4 * k }}>{last}</Fit>}
    </div>
  )
}
