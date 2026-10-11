import { describe, expect, it, vi } from 'vitest'
import { queryClient } from '@/api/query-client'

vi.mock('@/api/client', () => ({ api: vi.fn(() => Promise.reject(new Error('product 7 is gone'))) }))

describe('clearance writes', () => {
  it('refresh the page even when the call fails part-way (Disable all)', async () => {
    const spy = vi.spyOn(queryClient, 'invalidateQueries').mockResolvedValue()
    const { disable } = await import('./api')
    await expect(disable([5, 7])).rejects.toThrow('product 7 is gone')
    expect(spy).toHaveBeenCalledWith({ queryKey: ['x', 'clearance'] })
  })
})
