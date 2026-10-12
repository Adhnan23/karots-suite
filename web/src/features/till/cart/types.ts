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
}

/** id is unique in the cart; per is this line's qty in one set (qty = per × count). */
export type LineGroup = { id: number; label: string; per: number; count: number }

export type Payment = {
  method: string // '' = not picked yet
  amount: string
  reference: string
  accountId: string // card/bank locker id for card/online
  meta: Record<string, string> // plugin tender data
}

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
}

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
  notice: Notice
}
