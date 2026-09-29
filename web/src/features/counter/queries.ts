import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ConversionRow, ExpensesPage } from '@/api/types.gen'

export const expensesQuery = queryOptions({ queryKey: ['till', 'expenses'], queryFn: () => api<ExpensesPage>('/till/expenses') })
export const conversionsQuery = queryOptions({ queryKey: ['till', 'conversions'], queryFn: () => api<ConversionRow[]>('/till/conversions') })
