// Mirrors plugins/repairs/api*.go by hand. Money and qty are decimal strings.

export type Status = 'received' | 'in_progress' | 'ready' | 'collected' | 'cancelled'

export type Row = {
  id: number
  ticketNo: string
  ticketCode: string
  customerName: string
  customerPhone: string
  repairType: string
  deviceModel: string
  status: Status
  due: string
  dueUrgent: boolean
  createdAt: string
  collectedAt: string
}

export type Part = { id: number; productId: number; name: string; qty: string; price: string; oldDiscount: string; warrantyDays: number }
export type Charge = { id: number; label: string; amount: string }
export type Deposit = { paymentId: number; amount: string; receiptNo: string; at: string }
export type Payout = { expenseId: number; amount: string; note: string; date: string }

export type Job = Row & {
  customerId: number | null
  repairedBy: string
  fault: string
  notes: string
  promisedDate: string
  urgent: boolean
  warrantyDays: number
  warrantyUntil: string
  inWarranty: boolean
  saleId: number | null
  saleReceipt: string
  reworkOf: number | null
  readyAt: string
  cancelledAt: string
  parts: Part[]
  charges: Charge[]
  estimate: string
  deposits: Deposit[]
  depositTotal: string
  legacyHeld: string
  outstanding: string
  repairerPaid: string
  payouts: Payout[]
  warrantyMode: 'days' | 'parts'
}

export type JobInput = {
  customerId: number
  repairType: string
  deviceModel: string
  repairedBy: string
  fault: string
  notes: string
  promisedDate: string
  urgent: boolean
  warrantyDays?: number
}

export type BillLine = { kind: 'part' | 'labour'; productId: number; name: string; qty: string; price: string; discount: string; discountType: '' | 'fixed' | 'percent' }
export type Bill = { customerId: number | null; lines: BillLine[]; free: boolean; onAccount: string; deposits: string; legacyHeld: string }

export type Suggestions = { types: string[]; models: string[]; repairers: string[]; labour: string[] }
export type Config = { warrantyMode: 'days' | 'parts'; defaultWarrantyDays: number }

export type ReportRow = { id: number; ticketNo: string; device: string; customer: string; collectedAt: string; free: boolean; revenue: string; cost: string; repairer: string; profit: string }
export type Report = { from: string; to: string; rows: ReportRow[]; revenue: string; cost: string; repairer: string; profit: string }

/** What a cart line added from a job carries (CartLine.plugin.repairs). */
export type LineTag = { job: number; ticket: string }
