import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ConversionRow, DebtReceiptRow, ExpensesPage, MoneyReceiptRow, SaleSummary, SaleView, ZReport } from '@/api/types.gen'

export const expensesQuery = queryOptions({ queryKey: ['till', 'expenses'], queryFn: () => api<ExpensesPage>('/till/expenses') })
export const conversionsQuery = queryOptions({ queryKey: ['till', 'conversions'], queryFn: () => api<ConversionRow[]>('/till/conversions') })
export const salesQuery = (q: string) =>
  queryOptions({ queryKey: ['till', 'sales', q], queryFn: () => api<SaleSummary[]>(`/till/sales?q=${encodeURIComponent(q)}`) })
export const moneyReceiptsQuery = (q: string) =>
  queryOptions({ queryKey: ['till', 'money-receipts', q], queryFn: () => api<MoneyReceiptRow[]>(`/till/money-receipts?q=${encodeURIComponent(q)}`) })
export const debtReceiptsQuery = (q: string) =>
  queryOptions({ queryKey: ['till', 'customer-payments', q], queryFn: () => api<DebtReceiptRow[]>(`/till/customer-payments?q=${encodeURIComponent(q)}`) })
export const saleQuery = (id: number) => queryOptions({ queryKey: ['till', 'sale', id], queryFn: () => api<SaleView>(`/till/sales/${id}`) })
export const zQuery = (id: number) => queryOptions({ queryKey: ['till', 'z', id], queryFn: () => api<ZReport>(`/till/z/${id}`) })
