import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { CheckFinding, DataPage } from '@/api/types.gen'

export const dataQuery = queryOptions({ queryKey: ['admin', 'data'], queryFn: () => api<DataPage>('/admin/data') })
export const dataCheckQuery = queryOptions({ queryKey: ['admin', 'data', 'check'], queryFn: () => api<CheckFinding[]>('/admin/data/check') })

/** "4.2 MB" */
export function fileSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`
}
