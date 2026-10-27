import { render, screen } from '@testing-library/react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { paperOf } from '@/features/print/labels/kit'
import { LabelPage } from '@/features/print/labels/picture'
import { slots } from '../slots'
import pharmacy from '.'
import type { DoseLabel } from './label'
import { DoseLabelPreview } from './LabelPreview'

vi.mock('@/features/print/labels/LabelView', () => ({
  LabelView: ({ onCut }: { onCut?: (c: boolean) => void }) => { onCut?.(true); return null },
}))
vi.mock('@tanstack/react-query', async (orig) => ({
  ...(await orig<object>()),
  useQuery: () => ({ data: { labelPrintAs: 'image' } }),
}))

pharmacy(slots)

const D: DoseLabel = { patient: 'NIMAL PERERA', drug: 'AMOXICILLIN 500MG', qty: 15, dose: '1-0-1', words: 'Morning · Night', perDay: 2, note: 'AFTER MEALS', on: new Date('2026-10-26T10:00:00+05:30') }

const draw = (style: string, d: DoseLabel = D, size = '50x30') => {
  const el = document.createElement('div')
  el.innerHTML = renderToStaticMarkup(<LabelPage kind="pharmacy.dose" data={d} paper={paperOf(size)} style={style} />)
  return el.firstElementChild as HTMLElement
}
const texts = (root: HTMLElement) => {
  const w = document.createTreeWalker(root, NodeFilter.SHOW_TEXT)
  const out: string[] = []
  while (w.nextNode()) if (w.currentNode.textContent!.trim()) out.push(w.currentNode.textContent!.trim())
  return out
}

describe('pharmacy.dose', () => {
  it('registers Classic, Clear, Boxed', () => {
    expect(slots.labelTemplates.find((k) => k.kind === 'pharmacy.dose')?.styles.map((s) => s.key)).toEqual(['classic', 'clear', 'boxed'])
  })
  it('Classic draws today’s lines', () => {
    const root = draw('classic')
    expect(root.textContent).toContain('NIMAL PERERA')
    expect(root.textContent).toContain('AMOXICILLIN 500MG × 15')
  })
  it('Clear: patient, drug, qty, dose and note in sentence case', () => {
    const root = draw('clear')
    expect(root.dataset.labelStyle).toBe('clear')
    const t = root.textContent!
    for (const s of ['Nimal Perera', 'Amoxicillin 500mg', '15', '1 – 0 – 1', 'Morning · Night', 'After meals']) expect(t).toContain(s)
    for (const n of texts(root)) if (/[A-Z].*[A-Z]/.test(n) && /[a-z]/i.test(n)) expect(n).not.toBe(n.toUpperCase())
  })
  it('Boxed: 1-0-1 → Morning 1 / Noon – / Night 1', () => {
    const root = draw('boxed')
    const boxes = [...root.querySelectorAll('[data-box]')].map((b) => b.textContent)
    expect(boxes).toEqual(['Morning1', 'Noon–', 'Night1'])
  })
  it('Boxed with sos draws as Clear', () => {
    const root = draw('boxed', { ...D, dose: 'sos', words: 'When needed', perDay: 0 })
    expect(root.querySelector('[data-box]')).toBeNull()
    expect(root.querySelector('[data-dose-clear]')).not.toBeNull()
  })
  it('the preview still warns when the picture had to cut', () => {
    render(<DoseLabelPreview d={D} />)
    expect(screen.getByText(/doesn’t fit/)).toBeTruthy()
  })
})
