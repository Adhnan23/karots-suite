import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/client'
import { queryClient } from '@/api/query-client'
import { configQuery } from '@/features/till/api'
import { printLabel } from './print'

const api = vi.hoisted(() => vi.fn())
const draw = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', async (orig) => ({ ...(await orig<object>()), api }))
vi.mock('./picture', () => ({ labelPictureQuery: () => ({ queryKey: ['pic', Math.random()], queryFn: draw, retry: false }) }))

const run = () => printLabel({ kind: 'price', data: {}, size: '50x25', count: 2, classic: { path: '/till/labels', body: { code: '1' } } })
const classicCalls = () => api.mock.calls.filter(([, o]) => o?.json)

describe('printLabel in Image mode', () => {
  beforeEach(() => {
    api.mockReset()
    draw.mockReset().mockResolvedValue({ blob: new Blob(['png']) })
    queryClient.setQueryData(configQuery.queryKey, { labelPrintAs: 'image', labelSize: '50x25' } as never)
  })
  it('sends the picture once, no Classic', async () => {
    api.mockResolvedValue(undefined)
    await run()
    expect(api).toHaveBeenCalledTimes(1)
    expect(api.mock.calls[0][0]).toBe('/till/labels?count=2&size=50x25')
  })
  it('a picture that cannot be drawn prints Classic', async () => {
    draw.mockRejectedValue(new Error('no canvas'))
    api.mockResolvedValue(undefined)
    await run()
    expect(classicCalls()).toHaveLength(1)
  })
  it('a picture the server refuses (BAD_IMAGE) prints Classic', async () => {
    api.mockRejectedValueOnce(new ApiError(422, 'BAD_IMAGE', 'bad', '/till/labels')).mockResolvedValue(undefined)
    await run()
    expect(classicCalls()).toHaveLength(1)
  })
  it('a lost connection after sending the picture never prints a second, Classic run', async () => {
    api.mockRejectedValueOnce(new TypeError('Failed to fetch'))
    await expect(run()).rejects.toThrow('Failed to fetch')
    expect(classicCalls()).toHaveLength(0)
  })
})
