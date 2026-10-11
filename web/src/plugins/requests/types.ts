// Mirrors plugins/requests/api.go (a plugin's JSON isn't in core's types.gen.ts).

export type AskRow = {
  /** 0 = the request's own first ask */
  id: number
  name: string
  phone: string
  by: string
  at: string
  calledAt: string | null
}

export type RequestRow = {
  id: number
  note: string
  status: 'open' | 'converted' | 'declined'
  /** waiting = has a product, no stock yet, someone to tell; call = it's in */
  stage: 'open' | 'waiting' | 'call' | 'done'
  reason: string
  productId: number | null
  productName: string
  askCount: number
  lastAsked: string
  asks: AskRow[]
}

export type RequestTab = 'open' | 'call' | 'done'

export type RequestList = { rows: RequestRow[]; counts: Record<RequestTab, number>; total: number }

export type MakeProduct = { name: string; categoryId: number; unitId: number; sellingPrice: string; costPrice: string; supplierId: number }
