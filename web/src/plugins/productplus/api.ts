import { keepPreviousData, queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import { queryClient } from '@/api/query-client'
import type { Cell, FieldDTO, FieldRequest, FieldType, FormField, Grid, Impact, SaveResult } from './types'

const base = '/x/productplus'

export const fieldsQuery = (disabled: boolean) =>
  queryOptions({ queryKey: ['x', 'productplus', 'fields', disabled], queryFn: () => api<FieldDTO[]>(`${base}/fields${disabled ? '?disabled=1' : ''}`) })

export const impactQuery = (id: number, type: FieldType, options: string[]) =>
  queryOptions({
    queryKey: ['x', 'productplus', 'impact', id, type, options],
    queryFn: () => api<Impact>(`${base}/fields/${id}/impact?${new URLSearchParams([['type', type], ...options.map((o) => ['option', o])])}`),
    enabled: id > 0,
  })

export const formQuery = (productId: number, categoryId: number, where: 'form' | 'intake') =>
  queryOptions({
    queryKey: ['x', 'productplus', 'form', productId, categoryId, where],
    queryFn: () => api<FormField[]>(`${base}/form?${new URLSearchParams({ productId: String(productId), categoryId: String(categoryId), where })}`),
    enabled: categoryId > 0,
  })

export const gridQuery = (fields: number[], categoryId: number, missing: boolean, q: string, page: number) =>
  queryOptions({
    queryKey: ['x', 'productplus', 'grid', fields, categoryId, missing, q, page],
    queryFn: () => api<Grid>(`${base}/grid?${new URLSearchParams({ fields: fields.join(','), categoryId: String(categoryId), missing: missing ? '1' : '', q, page: String(page) })}`),
    enabled: fields.length > 0,
    placeholderData: keepPreviousData,
  })

/** Every write refreshes the plugin's data (and product lists: list lines change). */
async function write<T>(p: Promise<T>): Promise<T> {
  try {
    return await p
  } finally {
    void queryClient.invalidateQueries({ queryKey: ['x', 'productplus'] })
    void queryClient.invalidateQueries({ queryKey: ['admin'] })
  }
}

export const createField = (f: FieldRequest) => write(api<FieldDTO>(`${base}/fields`, { json: f }))
export const updateField = (id: number, f: FieldRequest) => write(api<FieldDTO>(`${base}/fields/${id}`, { method: 'PUT', json: f }))
export const setActive = (id: number, active: boolean) => write(api(`${base}/fields/${id}/active`, { json: { active } }))
export const moveField = (id: number, dir: 'up' | 'down') => write(api(`${base}/fields/${id}/move`, { json: { dir } }))
export const saveValues = (cells: Cell[]) => write(api<SaveResult>(`${base}/values`, { method: 'PUT', json: cells }))
