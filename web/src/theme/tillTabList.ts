import type { TillConfig } from '@/api/types.gen'
import { slots } from '@/plugins/slots'

export type Tab = { to: string; label: string; show?: (c: TillConfig) => boolean }

// The tab order. The bar shows as many as fit, in this order (TillTabs); the
// rest go under More.
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
  { to: '/till/low-stock', label: 'Low stock', show: (c) => c.canSeeLowStock },
  { to: '/till/suppliers', label: 'Suppliers', show: (c) => c.canHandleSuppliers },
  { to: '/till/expenses', label: 'Expenses', show: (c) => c.canHandleSuppliers },
]

/** A sales person's till has no tab that touches money (plugin tabs included). */
const SALES_PERSON = [...PRIMARY, ...MORE].filter((t) => ['/till', '/till/labels', '/till/low-stock'].includes(t.to))

/** Every till tab this user may open, for the Ctrl K palette. */
export const tillTabList = (config: TillConfig): Tab[] => config.sendsToCashier ? SALES_PERSON.filter((t) => !t.show || t.show(config)) : [
  ...[...PRIMARY, ...MORE].filter((t) => !t.show || t.show(config)),
  ...slots.tillTabs.map((t) => ({ to: `/till/x/${t.key}`, label: t.label })),
]

