/** Reload & Bills JSON (plugins/recharge, Go). Money is a decimal string. */
export type Kind = 'reload' | 'billpay' | 'deposit' | 'cashout' | 'wallet_in'
export type Rate = { kind: Kind; percent: string; fixed: string; charge: string }
export type Carrier = { id: number; name: string; productId: number; isActive: boolean; rates: Rate[] }
export type Device = {
  id: number; carrierId: number; carrier: string; label: string; number: string
  reload: boolean; money: boolean; isActive: boolean; target: string
}
export type Float = Device & { balance: string; lastCount: string | null; lastMove: string | null }
export type Package = { id: number; carrierId: number; name: string; price: string; floatCost: string; isActive: boolean; sort: number; addedBy: string }
export type Bank = { id: number; name: string; balance: string }
export type TillData = {
  carriers: Carrier[]; floats: Float[]; packages: Package[]; banks: Bank[]; billTypes: string[]
  bills: { billType: string; ref: string }[] // bill + account number pairs paid before, newest first
  products: { billpay: number; deposit: number; cashout: number; charge: number }
}
export type CountResult = { deviceId: number; expected: string; counted: string; diff: string }
export type Source = { value: string; label: string; balance: string }
export type SaleLine = { saleItemId: number; kind: string; label: string; left: string }
export type OldSlip = { no: string; at: string; title: string; detail: string; amount: string; charge: string; by: string }
export type Suggestion = { carrierId: number; carrier: string; current: string; suggested: string; counts: number; extra: string; reloads: string }
export type FloatPage = { floats: Float[]; total: string; suggestions: Suggestion[]; today: string; month: string; newPackages: number }
export type Setup = { carriers: Carrier[]; devices: Device[]; packages: Package[] }
export type LedgerRow = { id: number; at: string; carrier: string; device: string; type: string; amount: string; floatDelta: string; detail: string; receipt: string; by: string }
export type ReportLine = { carrier: string; kind: string; count: number; amount: string; commission: string }
export type CountRow = { at: string; device: string; expected: string; counted: string; by: string }
export type Report = { from: string; to: string; earnings: string; lines: ReportLine[]; banks: ReportLine[]; counts: CountRow[] }

/** What a recharge cart line carries (CartLine.plugin.recharge); the Go lineData. */
export type LineTag = {
  kind: 'reload' | 'package' | 'billpay' | 'deposit' | 'cashout' | 'charge'
  device?: number; bank?: number; package?: number; number?: string; billType?: string; ref?: string
  label?: string // shown on the line hint (where the float / money is)
}
