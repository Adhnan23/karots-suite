import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { LATEST, type Range, rangeQS } from './range'
import type { ConversionRow, DebtReceiptRow, ExpensesPage, MoneyReceiptRow, OrderView, SaleSummary, SaleView, SupplierRow, SupplierView, WarrantyClaimRow, WarrantyReceiptView, WarrantyUnitRow, WarrantyUnitView, ZReport } from '@/api/types.gen'

/** Rows a counter list loads at a time (internal/api counterPage). */
const PAGE = 50
/** A counter list that pages instead of stopping silently at its newest rows:
 *  the next page is asked for (?offset=) while the last one came back full. */
export const paged = <T,>(queryKey: readonly unknown[], url: string) =>
  infiniteQueryOptions({
    queryKey, initialPageParam: 0,
    queryFn: ({ pageParam }) => api<T[]>(`${url}&offset=${pageParam}`),
    getNextPageParam: (last: T[], all: T[][]) => (last.length < PAGE ? undefined : all.reduce((n, p) => n + p.length, 0)),
  })

/** "12", or "50+" while a paged list has older rows still to load. */
export const countText = (n: number, more: boolean) => `${n}${more ? '+' : ''}`

export const expensesQuery = queryOptions({ queryKey: ['till', 'expenses'], queryFn: () => api<ExpensesPage>('/till/expenses') })
export const conversionsQuery = queryOptions({ queryKey: ['till', 'conversions'], queryFn: () => api<ConversionRow[]>('/till/conversions') })
export const salesQuery = (q: string, r: Range = LATEST) =>
  paged<SaleSummary>(['till', 'sales', q, r], `/till/sales?q=${encodeURIComponent(q)}${rangeQS(r)}`)
export const moneyReceiptsQuery = (q: string, kind = '', r: Range = LATEST) =>
  paged<MoneyReceiptRow>(['till', 'money-receipts', q, kind, r], `/till/money-receipts?q=${encodeURIComponent(q)}&kind=${kind}${rangeQS(r)}`)
export const debtReceiptsQuery = (q: string, r: Range = LATEST) =>
  paged<DebtReceiptRow>(['till', 'customer-payments', q, r], `/till/customer-payments?q=${encodeURIComponent(q)}${rangeQS(r)}`)
export const saleQuery = (id: number) => queryOptions({ queryKey: ['till', 'sale', id], queryFn: () => api<SaleView>(`/till/sales/${id}`) })
export const zQuery = (id: number) => queryOptions({ queryKey: ['till', 'z', id], queryFn: () => api<ZReport>(`/till/z/${id}`) })
export const warrantyUnitsQuery = (status: string, q: string) =>
  paged<WarrantyUnitRow>(['till', 'warranty', status, q], `/till/warranty?status=${status}&q=${encodeURIComponent(q)}`)
export const warrantySerialQuery = (serial: string) =>
  queryOptions({ queryKey: ['till', 'warranty-serial', serial], queryFn: () => api<WarrantyUnitView>(`/till/warranty/serial/${encodeURIComponent(serial)}`), retry: false })
export const warrantyReceiptQuery = (no: string) =>
  queryOptions({ queryKey: ['till', 'warranty-receipt', no], queryFn: () => api<WarrantyReceiptView>(`/till/warranty/receipt?no=${encodeURIComponent(no)}`), retry: false })
export const warrantyClaimsQuery = (q: string, r: Range = LATEST) =>
  paged<WarrantyClaimRow>(['till', 'warranty-claims', q, r], `/till/warranty-claims?q=${encodeURIComponent(q)}${rangeQS(r)}`)
export const suppliersQuery = (q: string) =>
  queryOptions({ queryKey: ['till', 'suppliers', q], queryFn: () => api<SupplierRow[]>(`/till/suppliers?q=${encodeURIComponent(q)}`) })
export const supplierQuery = (id: number) => queryOptions({ queryKey: ['till', 'supplier', id], queryFn: () => api<SupplierView>(`/till/suppliers/${id}`) })
export const orderQuery = (id: number) => queryOptions({ queryKey: ['till', 'order', id], queryFn: () => api<OrderView>(`/till/orders/${id}`) })
