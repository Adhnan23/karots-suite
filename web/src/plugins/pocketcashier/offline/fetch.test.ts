import 'fake-indexeddb/auto'
import { onlineManager } from '@tanstack/react-query'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { saveWho } from '../session'
import { ack, pending } from '../store/outbox'
import { addRoutes, clearRoutesForTest, installOfflineFetch, RouteError, tempNumber } from './fetch'

const real = vi.fn()
let liveNow = false
vi.mock('../sync', async (orig) => ({ ...(await orig<object>()), state: () => ({ live: liveNow }) }))

const json = (v: unknown, status = 200) => new Response(JSON.stringify(v), { status, headers: { 'Content-Type': 'application/json' } })

describe('plugin calls with the PC down', () => {
  let uninstall: () => void
  beforeEach(async () => {
    real.mockReset().mockImplementation(async () => json({ from: 'pc' }))
    window.fetch = real as never
    liveNow = false
    saveWho({ id: 7, name: 'Nimal', role: 'cashier' })
    await ack((await pending()).map((a) => a.ref))
    clearRoutesForTest()
    addRoutes([
      { method: 'GET', path: '/x/t/things/:id', answer: ({ params, query }) => ({ id: params.id, q: query.get('q') }) },
      { method: 'POST', path: '/x/t/things', queue: true, label: () => 'New thing', answer: ({ ref, body }) => ({ id: `temp:${ref}`, ...(body as object) }) },
    ])
    uninstall = installOfflineFetch()
  })
  afterEach(() => { uninstall(); saveWho(null) })

  it('live calls go to the network', async () => {
    liveNow = true
    const r = await window.fetch('/api/v1/x/t/things/1')
    expect(await r.json()).toEqual({ from: 'pc' })
    expect(real).toHaveBeenCalled()
  })

  it('offline, answers from the route without waiting on the network', async () => {
    const r = await window.fetch('/api/v1/x/t/things/5?q=abc')
    expect(await r.json()).toEqual({ id: '5', q: 'abc' })
    expect(real).not.toHaveBeenCalled()
  })

  it('live but the network fails: answers from the route', async () => {
    liveNow = true
    real.mockRejectedValue(new TypeError('Failed to fetch'))
    expect(await (await window.fetch('/api/v1/x/t/things/5')).json()).toMatchObject({ id: '5' })
  })

  it('signed in while the PC was down, so no session yet: answers from the route', async () => {
    liveNow = true
    real.mockResolvedValue(json({ error: { code: 'UNAUTHORIZED' } }, 401))
    expect(await (await window.fetch('/api/v1/x/t/things/5')).json()).toMatchObject({ id: '5' })
  })

  it('a queued write saves a call as this person and answers with a temporary id', async () => {
    const r = await window.fetch('/api/v1/x/t/things', { method: 'POST', body: JSON.stringify({ name: 'A' }) })
    const made = await r.json() as { id: string; name: string }
    expect(made.name).toBe('A')
    await vi.waitFor(async () => expect(await pending()).toHaveLength(1))
    const [a] = await pending()
    expect(made.id).toBe(`temp:${a.ref}`)
    expect(a).toMatchObject({ kind: 'call', userId: 7, body: { method: 'POST', path: '/api/v1/x/t/things', body: { name: 'A' }, label: 'New thing', tempId: tempNumber(a.ref) } })
  })

  it('an unknown route still fails as the network does', async () => {
    real.mockRejectedValue(new TypeError('Failed to fetch'))
    await expect(window.fetch('/api/v1/x/t/other')).rejects.toThrow('Failed to fetch')
  })

  it('the phone\'s own calls are never answered locally', async () => {
    addRoutes([{ method: 'GET', path: '/x/pocketcashier/copy', answer: () => ({ fake: true }) }])
    await window.fetch('/api/v1/x/pocketcashier/copy')
    expect(real).toHaveBeenCalled()
  })

  it('a route\'s no reads as the PC\'s', async () => {
    addRoutes([{ method: 'GET', path: '/x/t/no', answer: () => { throw new RouteError('add a part first') } }])
    const r = await window.fetch('/api/v1/x/t/no')
    expect(r.status).toBe(422)
    expect(await r.json()).toMatchObject({ error: { message: 'add a part first' } })
  })

  it('screens keep asking with no network: the answers are on the phone', () => {
    onlineManager.setOnline(false)
    uninstall()
    uninstall = installOfflineFetch()
    expect(onlineManager.isOnline()).toBe(true)
    window.dispatchEvent(new Event('offline'))
    expect(onlineManager.isOnline()).toBe(true)
  })

  it('uninstall puts fetch back', () => {
    uninstall()
    expect(window.fetch).toBe(real)
    uninstall = () => {}
  })
})

describe('temporary numbers', () => {
  it('are positive, safe, far above any real row, and the same for the same ref', () => {
    const n = tempNumber('3f2a9c10-aaaa-4bbb-8ccc-1234567890ab')
    expect(n).toBeGreaterThanOrEqual(9e15)
    expect(Number.isSafeInteger(n)).toBe(true)
    expect(String(n)).toMatch(/^9\d{15}$/)
    expect(tempNumber('3f2a9c10-aaaa-4bbb-8ccc-1234567890ab')).toBe(n)
    expect(tempNumber('ffffffff-ffff-4fff-8fff-ffffffffffff')).toBeLessThanOrEqual(Number.MAX_SAFE_INTEGER)
  })
})
