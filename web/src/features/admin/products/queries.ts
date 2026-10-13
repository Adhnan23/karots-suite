import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AdminProduct, AdminProductRow, Count, Option, ProductBarcode, ProductList } from '@/api/types.gen'

export type ProductFilter = { q: string; category: number; low: boolean; disabled: boolean; page: number }

export const productsQuery = (f: ProductFilter) =>
  queryOptions({
    queryKey: ['admin', 'products', f],
    queryFn: () => api<ProductList>(`/admin/products?q=${encodeURIComponent(f.q)}&category=${f.category}&low=${f.low ? 1 : 0}&disabled=${f.disabled ? 1 : 0}&page=${f.page}`),
  })
export const productQuery = (id: number) => queryOptions({ queryKey: ['admin', 'product', id], queryFn: () => api<AdminProduct>(`/admin/products/${id}`) })
export const reviewQuery = queryOptions({ queryKey: ['admin', 'products', 'review'], queryFn: () => api<AdminProductRow[]>('/admin/products/review') })
export const reviewCountQuery = queryOptions({ queryKey: ['admin', 'products', 'review-count'], queryFn: () => api<Count>('/admin/products/review/count') })
export const supplierOptionsQuery = queryOptions({ queryKey: ['admin', 'supplier-options'], queryFn: () => api<Option[]>('/admin/supplier-options') })
export const barcodesQuery = (id: number) =>
  queryOptions({ queryKey: ['admin', 'product', id, 'barcodes'], queryFn: () => api<ProductBarcode[]>(`/admin/products/${id}/barcodes`) })
