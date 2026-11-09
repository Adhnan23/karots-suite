import { describe, expect, it } from 'vitest'
import { queryClient } from '@/api/query-client'
import { configQuery } from '@/features/till/api'
import type { TillConfig } from '@/api/types.gen'
import { slipPictureQuery } from './picture'

describe('slipPictureQuery', () => {
  it("keys the picture by the shop's saved style, so a new style draws anew", () => {
    queryClient.setQueryData(configQuery.queryKey, { receiptStyle: 'bold' } as TillConfig)
    expect(slipPictureQuery('/till/sales/1/print').queryKey).toEqual(['slip-picture', '/till/sales/1/print', '', 'bold', 100])
    queryClient.setQueryData(configQuery.queryKey, { receiptStyle: 'ticket' } as TillConfig)
    expect(slipPictureQuery('/till/sales/1/print').queryKey).toContain('ticket')
  })
  it('an explicit style (the Appearance preview) wins', () => {
    expect(slipPictureQuery('/x', '58', 'grid').queryKey).toEqual(['slip-picture', '/x', '58', 'grid', 100])
  })
  it("keys it by the shop's text size too, and an explicit size wins", () => {
    queryClient.setQueryData(configQuery.queryKey, { receiptStyle: 'bold', receiptTextSize: 130 } as TillConfig)
    expect(slipPictureQuery('/x').queryKey).toEqual(['slip-picture', '/x', '', 'bold', 130])
    expect(slipPictureQuery('/x', '80', 'bold', 115).queryKey).toEqual(['slip-picture', '/x', '80', 'bold', 115])
  })
})
