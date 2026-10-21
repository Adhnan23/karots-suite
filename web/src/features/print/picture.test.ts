import { describe, expect, it } from 'vitest'
import { queryClient } from '@/api/query-client'
import { configQuery } from '@/features/till/api'
import type { TillConfig } from '@/api/types.gen'
import { slipPictureQuery } from './picture'

describe('slipPictureQuery', () => {
  it("keys the picture by the shop's saved style, so a new style draws anew", () => {
    queryClient.setQueryData(configQuery.queryKey, { receiptStyle: 'bold' } as TillConfig)
    expect(slipPictureQuery('/till/sales/1/print').queryKey).toEqual(['slip-picture', '/till/sales/1/print', '', 'bold'])
    queryClient.setQueryData(configQuery.queryKey, { receiptStyle: 'ticket' } as TillConfig)
    expect(slipPictureQuery('/till/sales/1/print').queryKey).toContain('ticket')
  })
  it('an explicit style (the Appearance preview) wins', () => {
    expect(slipPictureQuery('/x', '58', 'grid').queryKey).toEqual(['slip-picture', '/x', '58', 'grid'])
  })
})
