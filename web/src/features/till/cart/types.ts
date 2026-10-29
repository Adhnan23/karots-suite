export type SaleType = 'retail' | 'wholesale'
export type DiscountType = 'fixed' | 'percent'
export type Component = { productId: number; quantity: string }

/** One cart line. Numbers here are a live preview; the server re-prices on post. */
export type CartLine = {
  key: number // unique per line: a product can be two lines (two lots, two service jobs)
  productId: number
  name: string
  unitPrice: number
  taxRate: number
  qty: number
  stock: number // MAX_STOCK for services: they never trip the found-at-till prompt
  isService: boolean
  allowDecimal: boolean
  batchId: number // the lot picked at "which price?" (0 = FEFO)
  batchRemaining: number
  batchLabel: string
  allowOversell: boolean // found-at-till confirmed for this line
  discount: string
  discountType: DiscountType // fixed is PER UNIT
  trackSerial: boolean
  warrantyMonths: number
  serials: string[]
  priceOverride: string // service lines only
  description: string
  components: Component[]
  plugin?: Record<string, unknown> // opaque data a plugin slot attached (M7)
  group?: LineGroup // lines a plugin added together (a combo): scaled, removed and broken together
  fixedQty?: boolean // a plugin job priced and costed for this qty: × removes it, nothing changes the qty
  packs?: LinePack[] // also sold by the pack: full packs ring at the pack price (cart/packs.ts)
  payout?: boolean // money handed to the customer: unitPrice < 0, priceOverride = the amount, never discounted
  noDiscount?: boolean // a plugin line whose amount passes through (a bill): no line discount offered
}

/** A pack on a cart line; price is for the sale type it was added under (0 = size × unit). */
export type LinePack = { id: number; name: string; size: number; price: number }

/** id is unique in the cart; per is this line's qty in one set (qty = per × count). */
/** fixed: service lines added as one priced job (a job and its add-ons) — one set, no − / +,
 *  removed as a whole, never broken back into plain lines. */
export type LineGroup = { id: number; label: string; per: number; count: number; fixed?: boolean }

export type Payment = {
  method: string // '' = not picked yet
  amount: string
  reference: string
  accountId: string // card/bank locker id for card/online
  meta: Record<string, string> // plugin tender data
  cheque?: ChequeDraft // a cheque line's cheque
}

/** The cheque a customer hands over: their bank, its number and date. */
export type ChequeDraft = { bank: string; number: string; date: string; note: string }

export type ServiceLineInput = {
  productId: number
  name: string
  price: number
  qty?: number
  unitPrice?: number
  description?: string
  components?: Component[]
  plugin?: Record<string, unknown>
  fixedQty?: boolean
  noDiscount?: boolean
}

/** Money the shop hands the customer (e.g. a cash out) on a pass-through service product. */
export type PayoutInput = { productId: number; name: string; amount: number; description?: string; plugin?: Record<string, unknown> }

export type Notice = { tone: 'success' | 'error'; text: string } | null

export type TillState = {
  lines: CartLine[]
  seq: number
  lastAddedKey: number
  saleType: SaleType
  customerId: number | null
  billDiscount: string
  billDiscountType: DiscountType
  tenders: Payment[]
  draft: Payment
  keep: string
  roundToAccount: boolean
  overLimitApproved: boolean
  postSaleChange: number // auto-print mode: last sale's change, shown until the next add
  meta?: Record<string, unknown> // plugins' notes on the sale in progress (TillApi.setSaleMeta), kept on hold
  notice: Notice
}
