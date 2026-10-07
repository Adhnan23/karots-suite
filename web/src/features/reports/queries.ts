import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { PeakHoursReport, SalesReport } from '@/api/types.gen'
import { type SalesFilter, salesQS } from '@/features/money/queries'
import { rangeOf, type RangeSearch, reportQS } from './kit/search'

/** One report's JSON for its URL filters. */
const report = <T,>(name: string, s: RangeSearch & Record<string, string | undefined>) =>
  queryOptions({ queryKey: ['admin', 'report', name, s], queryFn: () => api<T>(`/admin/reports/${name}?${reportQS(s)}`) })

export const salesReportQuery = (f: SalesFilter) =>
  queryOptions({ queryKey: ['admin', 'report', 'sales', f], queryFn: () => api<SalesReport>(`/admin/reports/sales?${salesQS({ ...f, ...rangeOf(f) })}`) })
export const peakHoursQuery = (s: RangeSearch) => report<PeakHoursReport>('peak-hours', s)
