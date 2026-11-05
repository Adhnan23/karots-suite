/** The plugin's admin JSON (plugins/pocketcashier/api_phones.go, review.go). */

export type PhoneRow = { id: number; name: string; status: 'pending' | 'active' | 'blocked'; userName: string; copyAt: string; lastSeen: string; pairCode: string }

export type Addresses = { setup: string[]; app: string[] }

export type SendRow = { id: number; phone: string; receivedAt: string; actions: number; failed: number; flagged: number }

export type SendAction = {
  ref: string
  kind: string
  label?: string
  userName: string
  at: string
  status: 'applied' | 'skipped' | 'failed'
  note: string
  saleId: number
  receiptNo: string
  flags: { kind: string; note: string }[]
}

export type SendDetail = SendRow & { actions: SendAction[] }
