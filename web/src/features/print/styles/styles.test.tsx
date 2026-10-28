import { renderToStaticMarkup } from 'react-dom/server'
import { afterAll, describe, expect, it } from 'vitest'
import { slots, type SlipTemplate } from '@/plugins/slots'
import type { SlipData } from '@/api/types.gen'
import { slipElement } from '../templates'
import { SAMPLE_NAMES, sample, type SampleName } from './sample'
import { STYLES } from '.'

const draw = (d: SlipData, style = 'classic') => renderToStaticMarkup(slipElement(d, style)!)
const text = (html: string) => {
  const el = document.createElement('div')
  el.innerHTML = html
  return (el.textContent ?? '').toLowerCase()
}

describe('classic is unchanged', () => {
  for (const name of SAMPLE_NAMES)
    for (const width of ['80', '58'] as const)
      it(`${name} at ${width} mm`, () => {
        expect(draw(sample(name, width))).toMatchSnapshot()
      })
})

// What each sample must show, whatever the style (a style changes looks, never content).
const MUST: Record<SampleName, string[]> = {
  busy: [
    'S-00231',
    'Panadol 500mg',
    '1 Card + 2',
    'Samsung Charger 25W',
    '245.00',
    '2,450.00',
    '2,205.00',
    '1,300.00',
    '1,200.00',
    '2027-10-21',
    'SC-88231',
    'Breakfast Deal',
    'Bread',
    'Milk 1L',
    'Reload cash out',
    '500.00',
    'Rice Samba 5kg',
    '6,765.00',
    '6,265.00',
    '5,000.00',
    '1,500.00',
    '234.50',
    '0.50',
    'Goods once sold are not returnable.',
    'Karots Suite',
    'Karots Store',
    '12 Main Street, Kandy',
    '081 222 3344',
    '114-556677-7000',
  ],
  payout: ['Bank cash out', '2,000.00', 'Biscuits', '1,700.00', 'given'],
  credit: ['Nimal Perera', '1,000.00', '200.00', '1,380.00', '4,380.00', '0.25', 'due'],
  refund: ['refund', 'S-00231', 'Wrong size', 'Samsung Charger 25W', '1,205.00', '1,000.00', '3,380.00'],
  money: ['Withdrawal', 'CR-000018', 'Till 1', 'Owner', 'Sunil', 'Kamal', '15,000.00', 'Weekly draw', 'Signature'],
  debt: ['DP-000002', 'Nimal Perera', '0771234567', '2,000.00', 'Cash', '6,380.00', '4,380.00', 'Kamal'],
  warranty: ['Nimal Perera', 'Samsung Charger 25W', 'SC-88231', 'SC-90412', '2027-10-21', '12 months left'],
  test: ['Karots Store', 'printer test', '80mm', '2026-10-21 14:32'],
}

describe('every style renders every line', () => {
  for (const s of STYLES)
    for (const width of ['80', '58'] as const)
      for (const name of SAMPLE_NAMES)
        it(`${s.key} · ${name} · ${width} mm`, () => {
          const d = sample(name, width)
          const got = text(draw(d, s.key))
          for (const want of MUST[name]) {
            const w = want === '80mm' ? `${width}mm` : want
            expect(got, `${s.key} ${name} ${width}: ${w}`).toContain(w.toLowerCase())
          }
          // The serial comes labelled ("SN: …"); a style must not label it again.
          expect(got, `${s.key} ${name} ${width}: serial label`).not.toMatch(/(sn|serial)\s*sn:/)
        })
})

it('an unknown style draws classic', () => {
  expect(draw(sample('busy'), 'modern')).toBe(draw(sample('busy'), 'classic'))
})

it('styles are the ten keys in order', () => {
  expect(STYLES.map((s) => s.key)).toEqual(['classic', 'bold', 'minimal', 'ticket', 'poster', 'grid', 'stamp', 'elegant', 'cards', 'sections'])
})

describe('a plugin slip follows every style', () => {
  const fake: SlipTemplate = {
    kind: 'x.test',
    label: 'Test',
    Template: ({ d, kit: K }) => {
      const x = d as SlipData & { note: string }
      return (
        <K.Page d={x}>
          <K.Header shop={x.shop} dots={x.dots} number="R-0007" />
          <K.Title>Plugin slip</K.Title>
          <K.Info l="Ticket:" r="R-0007" />
          <K.Total label="DEPOSIT" amount="Rs. 2,000.00" />
          <K.Text>{x.note}</K.Text>
          <K.Footer shop={x.shop} number="R-0007" />
        </K.Page>
      )
    },
  }
  slots.slipTemplates.push(fake)
  afterAll(() => void slots.slipTemplates.splice(slots.slipTemplates.indexOf(fake), 1))
  for (const s of STYLES)
    it(s.key, () => {
      const d = { ...sample('test'), kind: 'x.test', note: 'Bring this slip' } as SlipData
      const got = text(draw(d, s.key))
      for (const want of ['plugin slip', 'r-0007', '2,000.00', 'bring this slip', 'karots store']) expect(got).toContain(want)
    })
})
