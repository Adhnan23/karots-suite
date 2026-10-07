import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Dashboard, DashboardCharts } from '@/api/types.gen'

export const dashboardQuery = queryOptions({ queryKey: ['admin', 'dashboard'], queryFn: () => api<Dashboard>('/admin/dashboard'), refetchInterval: 60_000 })
export const dashboardChartsQuery = (days: number) =>
  queryOptions({ queryKey: ['admin', 'dashboard-charts', days], queryFn: () => api<DashboardCharts>(`/admin/dashboard/charts?days=${days}`) })
