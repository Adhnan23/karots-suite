import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { CashierReport, CategoryProfitReport, DuesReport, ExpensesReport, TaxReport, PeakHoursReport, ProductSalesReport, PurchasesReport, ReturnLineRow, ReturnsReport, SalesReport, SupplierSpendReport, TopProductsReport } from '@/api/types.gen'
import { paged } from '@/features/counter/queries'
import { type SalesFilter, salesQS } from '@/features/money/queries'
import { rangeOf, type RangeSearch, reportQS } from './kit/search'

/** One report's JSON for its URL filters. */
const report = <T,>(name: string, s: RangeSearch & Record<string, string | undefined>) =>
  queryOptions({ queryKey: ['admin', 'report', name, s], queryFn: () => api<T>(`/admin/reports/${name}?${reportQS(s)}`) })

export const salesReportQuery = (f: SalesFilter) =>
  queryOptions({ queryKey: ['admin', 'report', 'sales', f], queryFn: () => api<SalesReport>(`/admin/reports/sales?${salesQS({ ...f, ...rangeOf(f) })}`) })
export const peakHoursQuery = (s: RangeSearch) => report<PeakHoursReport>('peak-hours', s)
export const topProductsQuery = (s: RangeSearch & { order?: string }) => report<TopProductsReport>('top-products', s)
export const productSalesQuery = (s: RangeSearch & { product?: string; grain?: string }) => report<ProductSalesReport>('product-sales', s)
export const cashiersQuery = (s: RangeSearch) => report<CashierReport>('sales-by-cashier', s)
export const returnsQuery = (s: RangeSearch) => report<ReturnsReport>('returns', s)
export const returnLinesQuery = (s: RangeSearch) => paged<ReturnLineRow>(['admin', 'report', 'return-lines', s], `/admin/reports/returns/lines?${reportQS(s)}`)
export const duesQuery = (kind: 'customer' | 'supplier') =>
  queryOptions({ queryKey: ['admin', 'report', `${kind}-dues`], queryFn: () => api<DuesReport>(`/admin/reports/${kind}-dues`) })
export const spendQuery = (s: RangeSearch) => report<SupplierSpendReport>('supplier-spend', s)
export const purchasesReportQuery = (s: RangeSearch) => report<PurchasesReport>('purchases', s)
export const categoryProfitQuery = (s: RangeSearch & { category?: string }) => report<CategoryProfitReport>('profit-by-category', s)
export const expensesReportQuery = (s: RangeSearch) => report<ExpensesReport>('expenses', s)
export const taxQuery = (s: RangeSearch) => report<TaxReport>('tax', s)
