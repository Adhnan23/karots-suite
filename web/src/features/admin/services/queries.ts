import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { Recipe, ServiceRow } from '@/api/types.gen'

export const servicesQuery = queryOptions({ queryKey: ['admin', 'services'], queryFn: () => api<ServiceRow[]>('/admin/services') })
export const recipeQuery = (id: number) => queryOptions({ queryKey: ['admin', 'recipe', id], queryFn: () => api<Recipe>(`/admin/products/${id}/recipe`) })
