import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AdminCustomerDetail, AdminCustomerList } from '@/api/types.gen'

export type CustomerFilter = { q: string; show: '' | 'owing' | 'advance' | 'over'; disabled: boolean; page: number }

export const customersQuery = (f: CustomerFilter) =>
  queryOptions({
    queryKey: ['admin', 'customers', f],
    queryFn: () => api<AdminCustomerList>(`/admin/customers?q=${encodeURIComponent(f.q)}&show=${f.show}&disabled=${f.disabled ? 1 : 0}&page=${f.page}`),
  })
export const customerQuery = (id: number) => queryOptions({ queryKey: ['admin', 'customer', id], queryFn: () => api<AdminCustomerDetail>(`/admin/customers/${id}`) })
