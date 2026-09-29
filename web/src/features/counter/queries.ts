import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ConversionRow, DebtReceiptRow, ExpensesPage, MoneyReceiptRow, OrderView, SaleSummary, SaleView, SupplierRow, SupplierView, WarrantyClaimRow, WarrantyReceiptView, WarrantyUnitRow, WarrantyUnitView, ZReport } from '@/api/types.gen'

export const expensesQuery = queryOptions({ queryKey: ['till', 'expenses'], queryFn: () => api<ExpensesPage>('/till/expenses') })
export const conversionsQuery = queryOptions({ queryKey: ['till', 'conversions'], queryFn: () => api<ConversionRow[]>('/till/conversions') })
export const salesQuery = (q: string, day = '') =>
  queryOptions({ queryKey: ['till', 'sales', q, day], queryFn: () => api<SaleSummary[]>(`/till/sales?q=${encodeURIComponent(q)}&day=${day}`) })
export const moneyReceiptsQuery = (q: string, kind = '', day = '') =>
  queryOptions({ queryKey: ['till', 'money-receipts', q, kind, day], queryFn: () => api<MoneyReceiptRow[]>(`/till/money-receipts?q=${encodeURIComponent(q)}&kind=${kind}&day=${day}`) })
export const debtReceiptsQuery = (q: string, day = '') =>
  queryOptions({ queryKey: ['till', 'customer-payments', q, day], queryFn: () => api<DebtReceiptRow[]>(`/till/customer-payments?q=${encodeURIComponent(q)}&day=${day}`) })
export const saleQuery = (id: number) => queryOptions({ queryKey: ['till', 'sale', id], queryFn: () => api<SaleView>(`/till/sales/${id}`) })
export const zQuery = (id: number) => queryOptions({ queryKey: ['till', 'z', id], queryFn: () => api<ZReport>(`/till/z/${id}`) })
export const warrantyUnitsQuery = (status: string, q: string) =>
  queryOptions({ queryKey: ['till', 'warranty', status, q], queryFn: () => api<WarrantyUnitRow[]>(`/till/warranty?status=${status}&q=${encodeURIComponent(q)}`) })
export const warrantySerialQuery = (serial: string) =>
  queryOptions({ queryKey: ['till', 'warranty-serial', serial], queryFn: () => api<WarrantyUnitView>(`/till/warranty/serial/${encodeURIComponent(serial)}`), retry: false })
export const warrantyReceiptQuery = (no: string) =>
  queryOptions({ queryKey: ['till', 'warranty-receipt', no], queryFn: () => api<WarrantyReceiptView>(`/till/warranty/receipt?no=${encodeURIComponent(no)}`), retry: false })
export const warrantyClaimsQuery = (q: string) =>
  queryOptions({ queryKey: ['till', 'warranty-claims', q], queryFn: () => api<WarrantyClaimRow[]>(`/till/warranty-claims?q=${encodeURIComponent(q)}`) })
export const suppliersQuery = (q: string) =>
  queryOptions({ queryKey: ['till', 'suppliers', q], queryFn: () => api<SupplierRow[]>(`/till/suppliers?q=${encodeURIComponent(q)}`) })
export const supplierQuery = (id: number) => queryOptions({ queryKey: ['till', 'supplier', id], queryFn: () => api<SupplierView>(`/till/suppliers/${id}`) })
export const orderQuery = (id: number) => queryOptions({ queryKey: ['till', 'order', id], queryFn: () => api<OrderView>(`/till/orders/${id}`) })
