/** Mirrors plugins/combos/api.go by hand (core's types.gen.ts is core-only). */
export type DiscountType = 'percent' | 'fixed' | 'price'

export type Member = {
  productId: number
  name: string
  qty: string
  sellingPrice: string
  wholesalePrice: string
  active: boolean
  isService: boolean
  stock: string
}

export type Combo = {
  id: number
  name: string
  discountType: DiscountType
  discountValue: string
  active: boolean
  startsOn: string
  endsOn: string
  window: 'on' | 'scheduled' | 'ended'
  members: Member[]
  fullPrice: string
  problem: string
  warnings: string[]
  soldSets: string
  soldNet: string
  lastSold: string | null
}

export type ComboRequest = {
  name: string
  discountType: DiscountType
  discountValue: string
  startsOn: string
  endsOn: string
  items: { productId: number; qty: string }[]
}

export type ReportRow = { comboId: number | null; name: string; sets: string; net: string; discount: string; bills: number }
export type Report = { rows: ReportRow[]; totals: ReportRow }

export type Settings = { autoApply: boolean }
