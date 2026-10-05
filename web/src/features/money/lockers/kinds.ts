import { Box, CreditCard, Landmark, type LucideIcon, Vault, Wallet } from 'lucide-react'

/** Locker kinds (internal/features/lockers), in the order the form offers them. */
export const KINDS: { kind: string; label: string; Icon: LucideIcon }[] = [
  { kind: 'safe', label: 'Safe', Icon: Vault },
  { kind: 'bank', label: 'Bank account', Icon: Landmark },
  { kind: 'pocket', label: 'Pocket', Icon: Wallet },
  { kind: 'card', label: 'Card machine', Icon: CreditCard },
  { kind: 'other', label: 'Other', Icon: Box },
]
export const kindOf = (k: string) => KINDS.find((x) => x.kind === k) ?? KINDS[KINDS.length - 1]

/** What a ledger row was (locker_ledger.kind). */
export const LEDGER_KINDS: Record<string, string> = {
  open_balance: 'Opening balance', transfer: 'Transfer', payment: 'Paid out', intake: 'Money in',
  bank_charge: 'Bank charge', interest: 'Interest', adjust: 'Adjustment',
}
