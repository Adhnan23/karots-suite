import type { LabelRequest, TillConfig } from '@/api/types.gen'
import type { LabelKind, PriceLabel, Slot } from '../kit'
import { ClassicPrice } from './classic'
import { Inverse } from './inverse'
import { Minimal } from './minimal'
import { barsFit } from './parts'
import { PriceTag } from './pricetag'
import { Ticket } from './ticket'

/** The made-up product Appearance and Settings draw. */
export const PRICE_SAMPLE: PriceLabel = {
  first: { key: 'name', text: 'Anchor Milk Powder 400g' },
  second: { key: 'price', text: 'Rs 1,250.00' },
  code: '4792024012348',
  format: 'EAN13',
  firstLines: 1,
  secondLines: 1,
  firstSize: '2',
  secondSize: '3',
}

/** The barcode (price) label, core's own kind. */
export const PRICE_KIND: LabelKind<PriceLabel> = {
  kind: 'price',
  label: 'Price label',
  sample: PRICE_SAMPLE,
  styles: [
    { key: 'classic', label: 'Classic', Template: ClassicPrice },
    { key: 'pricetag', label: 'Price Tag', Template: PriceTag, fits: barsFit(14) },
    { key: 'inverse', label: 'Inverse', Template: Inverse, fits: barsFit(16) },
    { key: 'ticket', label: 'Ticket', Template: Ticket, fits: barsFit(36) },
    { key: 'minimal', label: 'Minimal', Template: Minimal, fits: barsFit(20) },
  ],
}

const slot = (key: string | undefined, text: string): Slot =>
  !text.trim() || key === 'none' ? { key: 'none', text: '' } : { key: key || 'free', text: text.trim() }

/** A label request (what Classic prints) as the price label a style draws, with
 * the shop's lines and sizes. Text with no field key (typed, a plugin's) is `free`. */
export const toPriceLabel = (r: LabelRequest, cfg: Pick<TillConfig, 'labelFirstLines' | 'labelSecondLines' | 'labelFirstSize' | 'labelSecondSize'>): PriceLabel => ({
  first: slot(r.topKey, r.top),
  second: slot(r.bottomKey, r.bottom),
  code: r.code || (r.productId ? `SKU${r.productId}` : ''), // the server's own fallback for a product with no barcode
  format: r.format || 'CODE128',
  firstLines: cfg.labelFirstLines || 1,
  secondLines: cfg.labelSecondLines || 1,
  firstSize: cfg.labelFirstSize || '2',
  secondSize: cfg.labelSecondSize || '3',
})
