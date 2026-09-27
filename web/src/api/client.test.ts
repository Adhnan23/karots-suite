import { afterEach, describe, expect, it, vi } from 'vitest'
import { api, ApiError } from './client'

const reply = (status: number, body?: unknown) =>
  vi.stubGlobal('fetch', vi.fn(async () => new Response(body === undefined ? null : JSON.stringify(body), { status })))
afterEach(() => vi.unstubAllGlobals())

describe('api', () => {
  it('prefixes /api/v1, sends JSON, returns body', async () => {
    reply(200, { ok: 1 })
    expect(await api<{ ok: number }>('/me', { method: 'POST', json: { a: 1 } })).toEqual({ ok: 1 })
    const [url, init] = (fetch as unknown as ReturnType<typeof vi.fn>).mock.calls[0]
    expect(url).toBe('/api/v1/me')
    expect(init.headers['Content-Type']).toBe('application/json')
    expect(init.body).toBe('{"a":1}')
    expect(init.credentials).toBe('same-origin')
  })
  it('returns undefined on 204', async () => {
    reply(204)
    expect(await api('/auth/lock', { method: 'POST' })).toBeUndefined()
  })
  it('throws ApiError with server code', async () => {
    reply(403, { error: { code: 'SESSION_LOCKED', message: 'Session locked' } })
    await expect(api('/x')).rejects.toMatchObject({ status: 403, code: 'SESSION_LOCKED', message: 'Session locked', path: '/x' })
  })
  it('survives a non-JSON error body', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response('<html>502</html>', { status: 502 })))
    const e = await api('/x').catch((x) => x)
    expect(e).toBeInstanceOf(ApiError)
    expect(e.code).toBe('HTTP_ERROR')
  })
})
