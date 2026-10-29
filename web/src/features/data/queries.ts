import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { DataPage, UpdateStatusDTO } from '@/api/types.gen'

export const dataQuery = queryOptions({ queryKey: ['admin', 'data'], queryFn: () => api<DataPage>('/admin/data') })

/** "4.2 MB" */
export function fileSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}

/** The Updates card: polls every 2 s while an update installs. */
export const updateQuery = queryOptions({
  queryKey: ['admin', 'data', 'update'],
  queryFn: () => api<UpdateStatusDTO>('/admin/data/update'),
  refetchInterval: (q) => (q.state.data?.state === 'installing' ? 2_000 : false),
  retry: true, // while installing the server restarts: keep asking
})
