import { PAPER_FAMILY } from '@/features/print/paper'
import { Fit } from '@/features/print/labels/fit'
import type { LabelTemplateProps } from '@/features/print/labels/kit'
import { doseLabel, type DoseLabel } from '../label'

/** Today's text label (doseLabel's lines) as a picture, in the printer's monospace. */
export function ClassicDose({ d, paper }: LabelTemplateProps<DoseLabel>) {
  const k = Math.min(paper.h / 240, paper.w / 400)
  return (
    <div style={{ height: '100%', padding: `${12 * k}px ${16 * k}px`, boxSizing: 'border-box', fontFamily: PAPER_FAMILY, fontWeight: 700, fontSize: 19 * k, lineHeight: 1.25 }}>
      {doseLabel(d).map((l, i) => <Fit key={i} lines={2}>{l}</Fit>)}
    </div>
  )
}
