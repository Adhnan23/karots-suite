import { SANS_FAMILY } from '@/features/print/paper'
import { Fit } from '@/features/print/labels/fit'
import { INK, type LabelPaper, type LabelTemplateProps } from '@/features/print/labels/kit'
import { qtyText } from '@/lib/money'
import { supplyDays } from '../dosage'
import type { DoseLabel } from '../label'

/** ALL-CAPS text → Sentence case, or Title Case for a `name` (pharmacy-label
 * guidance: capitals are hard to read); text with small letters stays as typed. */
export const calm = (s = '', name = false) =>
  /[a-z]/.test(s) || !/[A-Z].*[A-Z]/.test(s) ? s : s.toLowerCase().replace(name ? /(^|\s)(\p{L})/gu : /^(\P{L}*)(\p{L})/u, (_, a, b) => a + b.toUpperCase())

export const dateOf = (d: DoseLabel) => (d.on ?? new Date()).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })

/** "5 days · After meals" (days only for a fixed dose). */
export function lastLine(d: DoseLabel) {
  const days = supplyDays(d.qty, d.perDay)
  return [days ? `${days} day${days === 1 ? '' : 's'}` : '', calm(d.note.trim())].filter(Boolean).join(' · ')
}

export const scale = (p: LabelPaper) => Math.min(p.h / 240, p.w / 400)

/** Patient + date, the medicine × qty, a rule, the dose big, what it means, days + how to take it. */
export function ClearDose({ d, paper }: LabelTemplateProps<DoseLabel>) {
  const k = scale(paper)
  const date = dateOf(d)
  const last = lastLine(d)
  return (
    <div data-dose-clear="" style={{ height: '100%', padding: `${10 * k}px ${16 * k}px`, boxSizing: 'border-box', fontFamily: SANS_FAMILY, lineHeight: 1.2 }}>
      {d.patient && (
        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8 * k, fontSize: 17 * k, fontWeight: 600 }}>
          <Fit lines={1}>{calm(d.patient, true)}</Fit><span style={{ flex: 'none' }}>{date}</span>
        </div>
      )}
      <div style={{ display: 'flex', justifyContent: 'space-between', gap: 8 * k, alignItems: 'baseline' }}>
        <Fit lines={2} style={{ fontSize: 22 * k, fontWeight: 700 }}>{calm(d.drug)} <span style={{ fontWeight: 400, fontSize: 17 * k }}>× {qtyText(d.qty)}</span></Fit>
        {!d.patient && <span style={{ flex: 'none', fontSize: 17 * k, fontWeight: 600 }}>{date}</span>}
      </div>
      <div style={{ height: Math.max(2, Math.round(3 * k)), background: INK, margin: `${6 * k}px 0` }} />
      <Fit lines={1} style={{ fontSize: 34 * k, fontWeight: 900, lineHeight: 1.1 }}>{d.dose.split('-').map((s) => s.trim()).join(' – ')}</Fit>
      {d.words.trim() && <Fit lines={2} style={{ fontSize: 19 * k, fontWeight: 600 }}>{calm(d.words.trim())}</Fit>}
      {last && <Fit lines={2} style={{ fontSize: 17 * k }}>{last}</Fit>}
    </div>
  )
}
