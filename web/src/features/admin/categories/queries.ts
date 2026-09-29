import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { CategoryNode } from '@/api/types.gen'

export const categoriesQuery = queryOptions({ queryKey: ['admin', 'categories'], queryFn: () => api<CategoryNode[]>('/admin/categories') })
