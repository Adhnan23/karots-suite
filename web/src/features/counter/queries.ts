import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ConversionRow, ExpensesPage, SaleSummary } from '@/api/types.gen'

export const expensesQuery = queryOptions({ queryKey: ['till', 'expenses'], queryFn: () => api<ExpensesPage>('/till/expenses') })
export const conversionsQuery = queryOptions({ queryKey: ['till', 'conversions'], queryFn: () => api<ConversionRow[]>('/till/conversions') })
export const salesQuery = (q: string) =>
  queryOptions({ queryKey: ['till', 'sales', q], queryFn: () => api<SaleSummary[]>(`/till/sales?q=${encodeURIComponent(q)}`) })
