import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AdminSupplierDetail, AdminSupplierList, Statement } from '@/api/types.gen'

export type SupplierFilter = { q: string; show: '' | 'owing' | 'credit'; disabled: boolean; page: number }

export const suppliersQuery = (f: SupplierFilter) =>
  queryOptions({
    queryKey: ['admin', 'suppliers', f],
    queryFn: () => api<AdminSupplierList>(`/admin/suppliers?q=${encodeURIComponent(f.q)}&show=${f.show}&disabled=${f.disabled ? 1 : 0}&page=${f.page}`),
  })
export const supplierQuery = (id: number) => queryOptions({ queryKey: ['admin', 'supplier', id], queryFn: () => api<AdminSupplierDetail>(`/admin/suppliers/${id}`) })
export const statementQuery = (kind: 'supplier' | 'customer', id: number) =>
  queryOptions({ queryKey: ['admin', 'statement', kind, id], queryFn: () => api<Statement>(`/admin/${kind}s/${id}/statement`) })
