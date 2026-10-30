import 'fake-indexeddb/auto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { saveCopy } from './store/copy'
import { addAction, pending } from './store/outbox'
import { savePhone } from './store/phone'
import { flush, refresh, startSync, state, type SyncState } from './sync'

type Call = { url: string; init?: RequestInit }
let calls: Call[] = []
let online = true
let copyVersion = 'v1'
let results: (refs: string[]) => { ref: string; status: string }[] = (refs) => refs.map((ref) => ({ ref, status: 'applied' }))

function fakeFetch(url: string, init?: RequestInit) {
  calls.push({ url, init })
  if (!online) return Promise.reject(new TypeError('Failed to fetch'))
  if (url.includes('/copy')) {
    const tag = `"${copyVersion}"`
    if ((init?.headers as Record<string, string>)?.['If-None-Match'] === tag) return Promise.resolve(new Response(null, { status: 304 }))
    return Promise.resolve(new Response(JSON.stringify({ version: copyVersion, takenAt: new Date().toISOString(), people: [] }), { status: 200, headers: { ETag: tag } }))
  }
  if (url.includes('/actions')) {
    const body = JSON.parse(String(init?.body)) as { actions: { ref: string }[] }
    return Promise.resolve(new Response(JSON.stringify({ results: results(body.actions.map((a) => a.ref)) }), { status: 200 }))
  }
  return Promise.resolve(new Response(null, { status: 204 }))
}

class FakeES {
  static last: FakeES | null = null
  listeners: Record<string, (e: MessageEvent) => void> = {}
  constructor(public url: string) { FakeES.last = this }
  addEventListener(k: string, fn: (e: MessageEvent) => void) { this.listeners[k] = fn }
  close() {}
}

beforeEach(async () => {
  calls = []
  online = true
  vi.stubGlobal('fetch', vi.fn(fakeFetch))
  vi.stubGlobal('EventSource', FakeES)
  await savePhone({ secret: 'k'.repeat(40), name: 'Phone 2', device: 'd1' })
  results = (refs) => refs.map((ref) => ({ ref, status: 'applied' }))
  online = true
  await flush(() => null)
})
afterEach(() => vi.unstubAllGlobals())

describe('sync', () => {
  it('offline: not live, actions wait', async () => {
    online = false
    await addAction({ kind: 'sale', userId: 1, body: {} })
    await refresh()
    await flush(() => null)
    expect(state().live).toBe(false)
    expect(await pending()).toHaveLength(1)
    expect(state().waiting).toBe(1)
  })
  it('back online: one POST, oldest first, then nothing waits', async () => {
    online = false
    const a = await addAction({ kind: 'open', userId: 1, body: {} })
    const b = await addAction({ kind: 'sale', userId: 1, body: {} })
    online = true
    calls = []
    await flush(() => 1)
    const posts = calls.filter((c) => c.url.includes('/actions'))
    expect(posts).toHaveLength(1)
    const sent = JSON.parse(String(posts[0].init?.body))
    expect(sent.actions.map((x: { ref: string }) => x.ref)).toEqual([a.ref, b.ref])
    expect(sent.signedIn).toBe(1)
    expect(state().waiting).toBe(0)
    expect(state().live).toBe(true)
  })
  it('a failed action is acknowledged too', async () => {
    results = (refs) => refs.map((ref) => ({ ref, status: 'failed' }))
    await addAction({ kind: 'sale', userId: 1, body: {} })
    await flush(() => null)
    expect(await pending()).toHaveLength(0)
  })
  it('a 304 keeps the copy', async () => {
    await saveCopy({ version: 'v1', people: [{ id: 9 }] } as never)
    await refresh()
    expect(calls.some((c) => c.url.includes('/copy'))).toBe(true)
    const { loadCopy } = await import('./store/copy')
    expect((await loadCopy())?.people).toEqual([{ id: 9 }])
  })
  it('a version event fetches the copy', async () => {
    const seen: SyncState[] = []
    const stop = startSync((s) => seen.push(s), () => null)
    await vi.waitFor(() => expect(FakeES.last).not.toBeNull())
    copyVersion = 'v2'
    calls = []
    FakeES.last!.listeners.version(new MessageEvent('version', { data: 'v2' }))
    await vi.waitFor(() => expect(calls.some((c) => c.url.includes('/copy'))).toBe(true))
    stop()
  })
})
