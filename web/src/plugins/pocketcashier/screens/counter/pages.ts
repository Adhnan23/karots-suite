import { HandCoins, Undo2, type LucideIcon } from 'lucide-react'
import type { ComponentType } from 'react'
import { Credit } from './Credit'
import type { CounterProps } from './CounterShell'
import { Returns } from './Returns'

export type CounterPage = 'credit' | 'returns'

/** The counter's other jobs on the phone: the same on or off the PC. */
export const COUNTER_PAGES: { key: CounterPage; label: string; Icon: LucideIcon; Page: ComponentType<CounterProps> }[] = [
  { key: 'credit', label: 'Credit', Icon: HandCoins, Page: Credit },
  { key: 'returns', label: 'Returns', Icon: Undo2, Page: Returns },
]
