import { HandCoins, type LucideIcon } from 'lucide-react'
import type { ComponentType } from 'react'
import { Credit } from './Credit'
import type { CounterProps } from './CounterShell'

export type CounterPage = 'credit'

/** The counter's other jobs on the phone: the same on or off the PC. */
export const COUNTER_PAGES: { key: CounterPage; label: string; Icon: LucideIcon; Page: ComponentType<CounterProps> }[] = [
  { key: 'credit', label: 'Credit', Icon: HandCoins, Page: Credit },
]
