import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { ChequeDuplicate } from '@/api/types.gen'

/** Bank names to suggest: the built-in Sri Lankan list plus names used before. */
export const chequeBanksQuery = queryOptions({ queryKey: ['cheques', 'banks'], queryFn: () => api<string[]>('/cheques/banks'), staleTime: 5 * 60_000 })

/** Has this cheque been seen before? Received: same bank + number; given: same number on the bank place. A warning, never a block. */
export async function chequeSeen(dir: 'in' | 'out', number: string, bank = '', lockerId = 0): Promise<boolean> {
  if (!number.trim()) return false
  const q = new URLSearchParams({ dir, number: number.trim(), bank, lockerId: String(lockerId) })
  return (await api<ChequeDuplicate>(`/cheques/duplicate?${q}`)).duplicate
}

export const today = () => {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
