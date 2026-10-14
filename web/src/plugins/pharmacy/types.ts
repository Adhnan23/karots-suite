/** Mirrors plugins/pharmacy/api*.go. Quantities are decimal strings, dates YYYY-MM-DD. */
export type Found = { patientId: number; customerId: number; name: string; phone: string; nic: string; allergies: string }
export type Patient = { id: number; customerId: number; name: string; phone: string; dob: string; sex: string; nic: string; allergies: string; notes: string }
export type Suggest = { productId: number; name: string; stock: string }
export type DueStatus = 'overdue' | 'soon' | 'covered' | ''
export type ScriptItem = {
  id: number; productId: number | null; groupId: number | null; drug: string; productName: string; groupName: string
  dosage: string; quantity: string; perDay: string; dispensed: string; owed: string
  lastAt: string | null; nextDue: string | null; status: DueStatus; suggest: Suggest | null
}
export type Script = {
  id: number; prescriberName: string; prescriberReg: string; prescribedOn: string; notes: string
  isRepeat: boolean; repeatIntervalDays: number; active: boolean; items: ScriptItem[]
}
export type ScriptItemInput = { id?: number; productId?: number | null; groupId?: number | null; drugText: string; dosage: string; quantity: string; perDay: string }
export type ScriptInput = Omit<Script, 'id' | 'active' | 'items'> & { patientId: number; items: ScriptItemInput[] }
export type RecentSale = { id: number; receipt: string; at: string; total: string; customer: string; items: string; recorded: string; medicines: number }
export type Flags = { controlled: boolean; schedule: string; rxOnly: boolean; medicine?: boolean }
export type Controlled = { productId: number; name: string; schedule: string; stock: string; unrecorded: number; oldRows: number }
export type RegisterRow = {
  id: number; at: string; type: string; in: string; out: string; balance: string; saleId: number | null
  reference: string; note: string; by: string; patient: string; prescriber: string
}
export type OldEntry = { at: string; direction: 'in' | 'out'; qty: string; patient: string; prescriber: string; reference: string; notes: string }
export type Register = Controlled & { from: string; opening: string; rows: RegisterRow[]; old: OldEntry[] }
export type Unrecorded = { saleId: number; receipt: string; at: string; productId: number; name: string; qty: string }
export type Medicine = { kind: 'group' | 'product'; id: number; name: string; members: number }
export type Refill = {
  itemId: number; rxId: number; patientId: number; patient: string; phone: string; prescriber: string; drug: string; dosage: string
  isRepeat: boolean; lastAt: string; due: string; status: 'overdue' | 'soon'; calledAt: string
}
export type LogItem = { drug: string; qty: string; days: number; controlled: boolean }
export type LogRow = {
  id: number; at: string; patientId: number; patient: string; prescriber: string; saleId: number | null; receipt: string
  by: string; returned: boolean; controlled: boolean; items: LogItem[]
}
