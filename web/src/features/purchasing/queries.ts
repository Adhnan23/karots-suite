import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AdminSupplierDetail, AdminSupplierList, PurchaseDetail, PurchaseList, PurchaseReturnDetail, PurchaseReturnList, ReorderList, ReturnableList, Statement } from '@/api/types.gen'

export type SupplierFilter = { q: string; show: '' | 'owing' | 'credit'; disabled: boolean; page: number }

export const suppliersQuery = (f: SupplierFilter) =>
  queryOptions({
    queryKey: ['admin', 'suppliers', f],
    queryFn: () => api<AdminSupplierList>(`/admin/suppliers?q=${encodeURIComponent(f.q)}&show=${f.show}&disabled=${f.disabled ? 1 : 0}&page=${f.page}`),
  })
export const supplierQuery = (id: number) => queryOptions({ queryKey: ['admin', 'supplier', id], queryFn: () => api<AdminSupplierDetail>(`/admin/suppliers/${id}`) })
export const statementQuery = (kind: 'supplier' | 'customer', id: number) =>
  queryOptions({ queryKey: ['admin', 'statement', kind, id], queryFn: () => api<Statement>(`/admin/${kind}s/${id}/statement`) })

export type PurchaseFilter = { tab: 'orders' | 'received'; q: string; supplier: number; unpaid: boolean; page: number }

export const purchasesQuery = (f: PurchaseFilter) =>
  queryOptions({
    queryKey: ['admin', 'purchases', f],
    queryFn: () => api<PurchaseList>(`/admin/purchases?tab=${f.tab}&q=${encodeURIComponent(f.q)}&supplier=${f.supplier}&unpaid=${f.unpaid ? 1 : 0}&page=${f.page}`),
  })
export const purchaseQuery = (id: number) => queryOptions({ queryKey: ['admin', 'purchase', id], queryFn: () => api<PurchaseDetail>(`/admin/purchases/${id}`) })
export const purchasesPrintQuery = (ids: string) =>
  queryOptions({ queryKey: ['admin', 'purchases-print', ids], queryFn: () => api<PurchaseDetail[]>(`/admin/purchases/print?ids=${ids}`) })

export type ReorderFilter = { q: string; category: number; supplier: number; page: number; hideCovered?: boolean; all?: boolean }
export const reorderQuery = (f: ReorderFilter) =>
  queryOptions({
    queryKey: ['admin', 'reorder', f],
    queryFn: () => api<ReorderList>(`/admin/reorder?q=${encodeURIComponent(f.q)}&category=${f.category}&supplier=${f.supplier}&page=${f.page}${f.hideCovered ? '&hideCovered=1' : ''}${f.all ? '&all=1' : ''}`),
  })

export const purchaseReturnsQuery = (supplier: number, page: number) =>
  queryOptions({ queryKey: ['admin', 'purchase-returns', supplier, page], queryFn: () => api<PurchaseReturnList>(`/admin/purchase-returns?supplier=${supplier}&page=${page}`) })
export const purchaseReturnQuery = (id: number) =>
  queryOptions({ queryKey: ['admin', 'purchase-return', id], queryFn: () => api<PurchaseReturnDetail>(`/admin/purchase-returns/${id}`) })
export const returnableQuery = (purchaseId: number) =>
  queryOptions({ queryKey: ['admin', 'returnable', purchaseId], queryFn: () => api<ReturnableList>(`/admin/purchases/${purchaseId}/returnable`) })
