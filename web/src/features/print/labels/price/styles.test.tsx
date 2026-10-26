import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { paperOf, type PriceLabel, type Slot } from '../kit'
import { LabelPage } from '../picture'
import { hero, splitPrice } from './hero'
import { PRICE_KIND, PRICE_SAMPLE } from '.'

const NAME: Slot = { key: 'name', text: 'Anchor Milk Powder 400g' }
const PRICE: Slot = { key: 'price', text: 'Rs 1,250.00' }
const MODEL: Slot = { key: 'f:Model no', text: 'SM-A155F' }
const NONE: Slot = { key: 'none', text: '' }
const SIZES = ['50x25', '38x25', '100x50']

const draw = (style: string, first: Slot, second: Slot, size = '50x25', more: Partial<PriceLabel> = {}) => {
  const el = document.createElement('div')
  el.innerHTML = renderToStaticMarkup(<LabelPage kind="price" data={{ ...PRICE_SAMPLE, first, second, ...more }} paper={paperOf(size)} style={style} />)
  return el.firstElementChild as HTMLElement
}

describe('hero', () => {
  it('the price wins wherever it is; no price → the second text; a blank hero → the other', () => {
    expect(hero({ ...PRICE_SAMPLE, first: NAME, second: PRICE }).hero).toBe(PRICE)
    expect(hero({ ...PRICE_SAMPLE, first: PRICE, second: NAME })).toMatchObject({ hero: PRICE, heroIsFirst: true })
    expect(hero({ ...PRICE_SAMPLE, first: NAME, second: MODEL }).hero).toBe(MODEL)
    expect(hero({ ...PRICE_SAMPLE, first: NAME, second: NONE })).toMatchObject({ hero: NAME, heroIsFirst: true })
  })
  it('splits a price into what goes small and what goes big', () => {
    expect(splitPrice('Rs 1,250.00')).toEqual({ before: 'Rs', whole: '1,250', after: '.00' })
    expect(splitPrice('1,250')).toEqual({ before: '', whole: '1,250', after: '' })
    expect(splitPrice('Free')).toBeNull()
  })
})

for (const style of ['pricetag', 'inverse', 'ticket', 'minimal'])
  describe(style, () => {
    it('is listed after classic', () => expect(PRICE_KIND.styles.map((s) => s.key)).toContain(style))
    for (const size of SIZES) {
      it(`${size}: draws both chosen texts, the bars and the digits, in its own style`, () => {
        const root = draw(style, NAME, PRICE, size)
        expect(root.dataset.labelStyle).toBe(style)
        expect(root.textContent).toContain('Anchor Milk Powder 400g')
        expect(root.textContent).toContain('1,250')
        expect(root.querySelector('[data-code="bars"]')).not.toBeNull()
        expect(root.textContent).toContain('4792024012348'.slice(-6))
      })
      it(`${size}: swapped texts are still both there`, () => {
        const root = draw(style, PRICE, NAME, size)
        expect(root.textContent).toContain('Anchor Milk Powder 400g')
        expect(root.textContent).toContain('1,250')
      })
    }
    it('a text set to none closes up', () => {
      const root = draw(style, NAME, NONE)
      expect(root.querySelectorAll('[data-slot]')).toHaveLength(1)
    })
  })

describe('Price Tag: the hero is the big one', () => {
  for (const [first, second, big] of [[NAME, PRICE, '1,250'], [PRICE, NAME, '1,250'], [NAME, MODEL, 'SM-A155F']] as const)
    it(`${first.key} + ${second.key} → ${big}`, () => {
      expect(draw('pricetag', first, second).querySelector('[data-hero]')?.textContent).toContain(big)
    })
  it('first none → only the hero', () => {
    const root = draw('pricetag', NONE, PRICE)
    expect(root.querySelectorAll('[data-slot]')).toHaveLength(1)
    expect(root.querySelector('[data-hero]')?.textContent).toContain('1,250')
  })
})

describe('Inverse', () => {
  it('the band holds the second text', () => {
    expect(draw('inverse', NAME, PRICE).querySelector('[data-band]')?.textContent).toContain('1,250')
    expect(draw('inverse', PRICE, NAME).querySelector('[data-band]')?.textContent).toContain('Anchor')
  })
  it('second none → no band', () => expect(draw('inverse', PRICE, NONE).querySelector('[data-band]')).toBeNull())
})
