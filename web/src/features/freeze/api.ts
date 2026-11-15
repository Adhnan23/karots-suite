import { queryOptions, type QueryClient } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { FrozenListDTO, TillProduct } from '@/api/types.gen'

/** Stock freeze: a frozen product sells without touching its stock (the front
 * shelf isn't counted). Every till user may unfreeze or open a box onto the shelf. */
export const frozenQuery = queryOptions({ queryKey: ['till', 'frozen'], queryFn: () => api<FrozenListDTO>('/till/frozen') })
export const tillProductQuery = (id: number) =>
  queryOptions({ queryKey: ['till', 'product', id], queryFn: () => api<TillProduct>(`/till/products/${id}`) })

export const refreshStock = (qc: QueryClient) => {
  void qc.invalidateQueries({ queryKey: ['till'] })
  void qc.invalidateQueries({ queryKey: ['admin'] })
}

/** Unfreeze with the shelf count; answers the product's stock now. */
export async function unfreeze(id: number, shelfQty: string): Promise<TillProduct> {
  await api(`/till/stock/${id}/unfreeze`, { json: { shelfQty } })
  return api<TillProduct>(`/till/products/${id}`)
}
