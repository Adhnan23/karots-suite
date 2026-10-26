import type { LabelKind, PriceLabel } from '../kit'
import { ClassicPrice } from './classic'

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
  styles: [{ key: 'classic', label: 'Classic', Template: ClassicPrice }],
}
