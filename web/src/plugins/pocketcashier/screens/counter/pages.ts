import { HandCoins, ReceiptText, ShieldCheck, Undo2, type LucideIcon } from 'lucide-react'
import type { ComponentType } from 'react'
import { Credit } from './Credit'
import type { CounterProps } from './CounterShell'
import { Receipts } from './Receipts'
import { Returns } from './Returns'
import { Warranty } from './Warranty'

export type CounterPage = 'credit' | 'returns' | 'receipts' | 'warranty'

/** The counter's other jobs on the phone: the same on or off the PC. */
export const COUNTER_PAGES: { key: CounterPage; label: string; Icon: LucideIcon; Page: ComponentType<CounterProps> }[] = [
  { key: 'credit', label: 'Credit', Icon: HandCoins, Page: Credit },
  { key: 'returns', label: 'Returns', Icon: Undo2, Page: Returns },
  { key: 'receipts', label: 'Receipts', Icon: ReceiptText, Page: Receipts },
  { key: 'warranty', label: 'Warranty', Icon: ShieldCheck, Page: Warranty },
]
