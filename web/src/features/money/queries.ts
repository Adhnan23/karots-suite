import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { AdminLockerList, AdminLockerView, LockerLedgerRow } from '@/api/types.gen'
import { paged } from '@/features/counter/queries'
import { type Range, rangeQS } from '@/features/counter/range'

export const lockersQuery = (disabled: boolean) =>
  queryOptions({ queryKey: ['admin', 'lockers', disabled], queryFn: () => api<AdminLockerList>(`/admin/lockers${disabled ? '?disabled=1' : ''}`) })
export const lockerQuery = (id: number, r: Range) =>
  queryOptions({ queryKey: ['admin', 'locker', id, r], queryFn: () => api<AdminLockerView>(`/admin/lockers/${id}?${rangeQS(r).slice(1)}`) })
export const lockerLedgerQuery = (id: number, r: Range) =>
  paged<LockerLedgerRow>(['admin', 'locker-ledger', id, r], `/admin/lockers/${id}/ledger?${rangeQS(r).slice(1)}`)
