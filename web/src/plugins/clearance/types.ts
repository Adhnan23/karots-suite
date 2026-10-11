// Mirrors plugins/clearance/api.go (a plugin's JSON isn't in core's types.gen.ts).

export type ClearanceSettings = { staleDays: number; defaultPercent: string; minMarginPercent: string; expiryDays: number; deadMonths: number }

export type MarkdownRow = {
  productId: number
  name: string
  unit: string
  onHand: string
  cost: string
  price: string
  /** the expiring lot's cost when the reason is expiry, else the product's */
  floorCost: string
  reason: 'stale' | 'expiring'
  reasonLabel: string
  status: '' | 'live' | 'snoozed'
  /** the live markdown, else the safe suggestion */
  percent: string
  newPrice: string
  snoozedUntil: string
}

export type DeadRow = { productId: number; name: string; unit: string; monthsIdle: number; lastActivity: string }

export type ClearancePageData = { settings: ClearanceSettings; rows: MarkdownRow[]; dead: DeadRow[] }
