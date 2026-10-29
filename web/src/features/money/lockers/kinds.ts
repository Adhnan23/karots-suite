import { Box, CreditCard, Landmark, type LucideIcon, ScrollText, Vault, Wallet } from 'lucide-react'

/** Locker kinds (internal/features/lockers), in the order the form offers them. */
export const KINDS: { kind: string; label: string; Icon: LucideIcon }[] = [
  { kind: 'safe', label: 'Safe', Icon: Vault },
  { kind: 'bank', label: 'Bank account', Icon: Landmark },
  { kind: 'pocket', label: 'Pocket', Icon: Wallet },
  { kind: 'card', label: 'Card machine', Icon: CreditCard },
  { kind: 'other', label: 'Other', Icon: Box },
]
/** Cheques in hand: kept by the suite (one per shop), never offered in the form. */
const CHEQUE = { kind: 'cheque', label: 'Cheques received, not cleared yet', Icon: ScrollText }
export const kindOf = (k: string) => (k === 'cheque' ? CHEQUE : KINDS.find((x) => x.kind === k) ?? KINDS[KINDS.length - 1])

/** What a ledger row was (locker_ledger.kind). */
export const LEDGER_KINDS: Record<string, string> = {
  open_balance: 'Opening balance', transfer: 'Transfer', payment: 'Payment', intake: 'Money in',
  bank_charge: 'Bank charge', interest: 'Interest', adjust: 'Adjustment',
}

/** Every kind a Cash flow row can be: a locker ledger kind or a till cash move. */
export const FLOW_KINDS: Record<string, string> = {
  ...LEDGER_KINDS, withdrawal: 'Taken from a till', pay_in: 'Put into a till', credit_payment: 'Credit collected', refund: 'Refund', expense: 'Expense',
}
