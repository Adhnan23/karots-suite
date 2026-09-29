import type { TillConfig } from '@/api/types.gen'
import { slots } from '@/plugins/slots'

export type Tab = { to: string; label: string; show?: (c: TillConfig) => boolean }

// Primary tabs sit in the bar; the rest live under More (the bar must fit at 1024).
export const PRIMARY: Tab[] = [
  { to: '/till', label: 'Sell' },
  { to: '/till/returns', label: 'Returns' },
  { to: '/till/credit', label: 'Credit' },
  { to: '/till/receipts', label: 'Receipts' },
]
export const MORE: Tab[] = [
  { to: '/till/write-off', label: 'Write-off' },
  { to: '/till/convert', label: 'Convert' },
  { to: '/till/labels', label: 'Labels' },
  { to: '/till/warranty', label: 'Warranty' },
  { to: '/till/suppliers', label: 'Suppliers', show: (c) => c.canHandleSuppliers },
  { to: '/till/expenses', label: 'Expenses', show: (c) => c.canHandleSuppliers },
]

/** Every till tab this user may open, for the Ctrl K palette. */
export const tillTabList = (config: TillConfig): Tab[] => [
  ...[...PRIMARY, ...MORE].filter((t) => !t.show || t.show(config)),
  ...slots.tillTabs.map((t) => ({ to: `/till/x/${t.key}`, label: t.label })),
]

