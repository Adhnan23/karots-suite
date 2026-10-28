// Mirrors plugins/documents/api.go and api_till.go.

export type Kind = 'metered' | 'custom'
export type Category = 'copy' | 'print' | 'laminate' | 'bind' | 'other'

export type Service = {
  id: number
  productId: number
  name: string
  kind: Kind
  category: Category
  active: boolean
  productActive: boolean
  sellingPrice: string
  prices: number
  paper: number
}

export type ServiceRequest = { productId?: number; name?: string; categoryId?: number; sellingPrice?: string; kind: Kind; category: Category }

/** Cells: B&W 1-side, B&W 2-side, Colour 1-side, Colour 2-side ('' = not offered). */
export type GridTier = { minQty: number; cells: [string, string, string, string] }
export type PaperLine = { productId: number; name: string; perSheet: string }
export type GridRow = { size: string; material: string; tiers: GridTier[]; paper: PaperLine[] }
export type Grid = { rows: GridRow[] }

export type TillBand = { minQty: number; price: string }
export type TillOffer = { size: string; material: string; color: boolean; double: boolean; bands: TillBand[] }
export type TillService = { id: number; productId: number; name: string; kind: Kind; category: Category; offers: TillOffer[] }

export type Quote = {
  unitPrice: string
  total: string
  minQty: number
  description: string
  sheets: number
  cost: string
  components: { productId: number; quantity: string }[]
}

/** What a Print & Copy cart line carries (CartLine.plugin.documents). */
export type LineJob = { serviceId: number; size?: string; material?: string; color?: boolean; double?: boolean; sheets?: number }

export type ReportRow = {
  name: string
  size: string | null
  material: string
  color: boolean | null
  double: boolean | null
  earlier: boolean
  jobs: number
  copies: string
  sheets: string
  revenue: string
}

export type Report = {
  rows: ReportRow[]
  jobs: number
  copies: string
  sheets: string
  revenue: string
  ownUseJobs: number
  ownUseSheets: string
  ownUseCost: string
}
