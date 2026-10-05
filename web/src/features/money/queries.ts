import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AdminCashflow, AdminSaleRow, AdminSalesSummary, AdminDenomination, CashierOption, TillSessionRow, TillSessionView, AdminExpenseRow, AdminExpenseSummary, AdminLockerList, CashflowRow, AdminLockerView, ExpenseVoidInfo, LockerLedgerRow } from '@/api/types.gen'
import { paged } from '@/features/counter/queries'
import { type Range, rangeQS } from '@/features/counter/range'

export const lockersQuery = (disabled: boolean) =>
  queryOptions({ queryKey: ['admin', 'lockers', disabled], queryFn: () => api<AdminLockerList>(`/admin/lockers${disabled ? '?disabled=1' : ''}`) })
export const lockerQuery = (id: number, r: Range) =>
  queryOptions({ queryKey: ['admin', 'locker', id, r], queryFn: () => api<AdminLockerView>(`/admin/lockers/${id}?${rangeQS(r).slice(1)}`) })
export const lockerLedgerQuery = (id: number, r: Range) =>
  paged<LockerLedgerRow>(['admin', 'locker-ledger', id, r], `/admin/lockers/${id}/ledger?${rangeQS(r).slice(1)}`)

export type ExpenseFilter = Range & { category: string; q: string }
const expenseQS = (f: ExpenseFilter) => `category=${encodeURIComponent(f.category)}&q=${encodeURIComponent(f.q)}${rangeQS(f)}`
export const expensesQuery = (f: ExpenseFilter) => paged<AdminExpenseRow>(['admin', 'expenses', f], `/admin/expenses?${expenseQS(f)}`)
export const expenseSummaryQuery = (f: ExpenseFilter) =>
  queryOptions({ queryKey: ['admin', 'expenses-summary', f], queryFn: () => api<AdminExpenseSummary>(`/admin/expenses/summary?${expenseQS(f)}`) })
export const expenseVoidQuery = (id: number) => queryOptions({ queryKey: ['admin', 'expense-void', id], queryFn: () => api<ExpenseVoidInfo>(`/admin/expenses/${id}/void`) })

const cfQS = (r: Range) => (rangeQS(r) || '&preset=today').slice(1)
export const cashflowQuery = (r: Range) => queryOptions({ queryKey: ['admin', 'cashflow', r], queryFn: () => api<AdminCashflow>(`/admin/cashflow?${cfQS(r)}`) })
export const cashflowLedgerQuery = (r: Range, place: string, kind: string) =>
  paged<CashflowRow>(['admin', 'cashflow-ledger', r, place, kind], `/admin/cashflow/ledger?${cfQS(r)}&place=${encodeURIComponent(place)}&kind=${kind}`)
export const cashflowPrintQuery = (r: Range) => queryOptions({ queryKey: ['admin', 'cashflow-print', r], queryFn: () => api<CashflowRow[]>(`/admin/cashflow/ledger?${cfQS(r)}&all=1`) })

export type TillFilter = Range & { user: number; open: boolean }
export const tillsQuery = (f: TillFilter) =>
  paged<TillSessionRow>(['admin', 'tills', f], `/admin/tills?user=${f.user || ''}&open=${f.open ? 1 : ''}${rangeQS(f)}`)
export const tillQuery = (id: number) => queryOptions({ queryKey: ['admin', 'till', id], queryFn: () => api<TillSessionView>(`/admin/tills/${id}`) })
export const tillCashiersQuery = queryOptions({ queryKey: ['admin', 'till-cashiers'], queryFn: () => api<CashierOption[]>('/admin/tills/cashiers') })
export const denominationsQuery = queryOptions({ queryKey: ['admin', 'denominations'], queryFn: () => api<AdminDenomination[]>('/admin/denominations') })

export type SalesFilter = Range & { status: string; method: string; q: string }
export const salesQS = (f: SalesFilter) => `status=${f.status}&method=${f.method}&q=${encodeURIComponent(f.q)}${rangeQS(f)}`
export const adminSalesQuery = (f: SalesFilter) => paged<AdminSaleRow>(['admin', 'sales', f], `/admin/sales?${salesQS(f)}`)
export const adminSalesSummaryQuery = (f: SalesFilter) =>
  queryOptions({ queryKey: ['admin', 'sales-summary', f], queryFn: () => api<AdminSalesSummary>(`/admin/sales/summary?${salesQS(f)}`) })
