import 'fake-indexeddb/auto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { loadParts, saveCopy } from './store/copy'
import { addAction, pending } from './store/outbox'
import { savePhone } from './store/phone'
import { loadWho, saveWho } from './session'
import { getSale, clearRecent, recentCursor } from './store/recent'
import { checkPC, flush, refresh, refreshParts, refreshRecent, sendCall, startSync, state, tempRef, type SyncState } from './sync'

type Call = { url: string; init?: RequestInit }
let recentPages: Record<string, unknown>[] = []
const sale = (id: number) => ({ id, receiptNo: `R-${id}`, createdAt: new Date().toISOString(), customerName: '', lines: [] })
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
  if (url.includes('/feed-ticket')) return Promise.resolve(new Response(JSON.stringify({ ticket: 'tk1' }), { status: 200 }))
  if (url.includes('/actions')) {
    const body = JSON.parse(String(init?.body)) as { actions: { ref: string }[] }
    return Promise.resolve(new Response(JSON.stringify({ results: results(body.actions.map((a) => a.ref)) }), { status: 200 }))
  }
  if (url.includes('/recent')) {
    const page = recentPages.shift()
    if (!page) return Promise.reject(new TypeError('Failed to fetch'))
    return Promise.resolve(new Response(JSON.stringify({ sales: [], changed: [], nextSince: '2026-11-05T10:00:00Z', more: false, maxId: 0, reset: false, ...page }), { status: 200 }))
  }
  return Promise.resolve(new Response(null, { status: 204 }))
}

class FakeES {
  static last: FakeES | null = null
  listeners: Record<string, (e: MessageEvent) => void> = {}
  url: string
  constructor(url: string) { this.url = url; FakeES.last = this }
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
  it('a reopened app counts what still waits, even with the PC down', async () => {
    online = false
    await addAction({ kind: 'sale', userId: 1, body: {} })
    const seen: SyncState[] = []
    const stop = startSync((st) => seen.push(st), () => null)
    await vi.waitFor(() => expect(seen.at(-1)?.waiting).toBe(1))
    stop()
    online = true
    await flush(() => null)
  })

  it('a version event fetches the copy', async () => {
    const seen: SyncState[] = []
    const stop = startSync((s) => seen.push(s), () => null)
    await vi.waitFor(() => expect(FakeES.last).not.toBeNull())
    // The feed opens with a one-time ticket, never the phone's key (URLs land in logs).
    expect(FakeES.last!.url).toContain('ticket=tk1')
    expect(FakeES.last!.url).not.toContain('k'.repeat(40))
    copyVersion = 'v2'
    calls = []
    FakeES.last!.listeners.version(new MessageEvent('version', { data: 'v2' }))
    await vi.waitFor(() => expect(calls.some((c) => c.url.includes('/copy'))).toBe(true))
    stop()
  })
  it('a feed that opens again checks the copy (a change made while it was down has no event)', async () => {
    const stop = startSync(() => {}, () => null)
    await vi.waitFor(() => expect(FakeES.last).not.toBeNull())
    calls = []
    FakeES.last!.listeners.open(new MessageEvent('open'))
    await vi.waitFor(() => expect(calls.some((c) => c.url.includes('/copy'))).toBe(true))
    stop()
  })
})

describe('moved', () => {
  it('says where the person went, and nothing when the PC is down', async () => {
    const { movedTo } = await import('./sync')
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      calls.push({ url })
      return new Response(JSON.stringify({ movedTo: 'Counter PC' }), { status: 200 })
    }))
    expect(await movedTo(7)).toBe('Counter PC')
    expect(calls.at(-1)!.url).toContain('/moved?user=7')
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))))
    expect(await movedTo(7)).toBe('')
  })
})

describe('the last 30 days', () => {
  beforeEach(async () => { recentPages = []; await clearRecent() })

  it('takes pages until the PC says done', async () => {
    recentPages = [{ sales: [sale(1)], nextAfter: 1, more: true }, { sales: [sale(2)], nextAfter: 2 }]
    await refreshRecent()
    expect(await getSale(1)).not.toBeNull()
    expect(await getSale(2)).not.toBeNull()
    expect(await recentCursor()).toMatchObject({ after: 2, ready: true })
  })

  it('a dropped download keeps its pages and asks on from there, with live state untouched', async () => {
    recentPages = [{ sales: [sale(1)], nextAfter: 1, more: true }]
    await refresh()
    const live = state().live
    await refreshRecent()
    expect(await getSale(1)).not.toBeNull()
    expect(await recentCursor()).toMatchObject({ after: 1, ready: false })
    expect(state().live).toBe(live)
    calls = []
    recentPages = [{ sales: [sale(2)], nextAfter: 2 }]
    await refreshRecent()
    expect(calls.find((c) => c.url.includes('/recent'))?.url).toContain('after=1')
    expect(await recentCursor()).toMatchObject({ after: 2, ready: true })
  })

  it('a restored shop: what the phone held goes', async () => {
    recentPages = [{ sales: [sale(9)], nextAfter: 9 }]
    await refreshRecent()
    recentPages = [{ reset: true, sales: [sale(2)], nextAfter: 2 }]
    await refreshRecent()
    expect(await getSale(9)).toBeNull()
    expect(await getSale(2)).not.toBeNull()
  })

  it('two at once download once', async () => {
    recentPages = [{ sales: [sale(1)], nextAfter: 1 }]
    calls = []
    await Promise.all([refreshRecent(), refreshRecent()])
    expect(calls.filter((c) => c.url.includes('/recent'))).toHaveLength(1)
  })
})

describe('noticing the PC has gone', () => {
  it('a look that gets no answer stops the phone saying Live', async () => {
    await refresh()
    expect(state().live).toBe(true)
    online = false
    await checkPC()
    expect(state().live).toBe(false)
    online = true
    await checkPC()
    expect(state().live).toBe(true)
  })

  it('a PC that has lost power never answers: the look gives up', async () => {
    await refresh()
    vi.stubGlobal('fetch', vi.fn((_u: string, init?: RequestInit) => new Promise((_r, reject) => {
      init?.signal?.addEventListener('abort', () => reject(new DOMException('timed out', 'TimeoutError')))
    })))
    await checkPC(20)
    expect(state().live).toBe(false)
  })
})

describe('who the phone holds', () => {
  it('a send says when this person signed in', async () => {
    saveWho({ id: 7, name: 'Nimal', role: 'cashier' })
    const since = loadWho()?.since
    expect(since).toBeTruthy()
    await refresh()
    calls = []
    await addAction({ kind: 'sale', userId: 7, body: {} })
    await flush(() => 7)
    expect(JSON.parse(String(calls.find((c) => c.url.includes('/actions'))?.init?.body)).signedInAt).toBe(since)
    saveWho(null)
  })
})

describe('what plugins read offline', () => {
  it('saves the parts and asks with the tag next time', async () => {
    const headers = new Headers({ ETag: '"abc"' })
    const body = JSON.stringify({ t: { n: 1 } })
    vi.stubGlobal('fetch', vi.fn(async (url: string, init?: RequestInit) => {
      calls.push({ url, init })
      return (init?.headers as Record<string, string>)['If-None-Match'] ? new Response(null, { status: 304 }) : new Response(body, { status: 200, headers })
    }))
    calls = []
    await refreshParts()
    expect(await loadParts()).toEqual({ t: { n: 1 } })
    await refreshParts()
    expect((calls[1].init?.headers as Record<string, string>)['If-None-Match']).toBe('"abc"')
    expect(await loadParts()).toEqual({ t: { n: 1 } })
  })
})

describe('what plugins read offline, before the phone is approved', () => {
  it('says the refresh failed, so it is tried again soon, not in a minute', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 401 })))
    expect(await refreshParts()).toBe(false)
    vi.stubGlobal('fetch', vi.fn(async () => new Response(null, { status: 304 })))
    expect(await refreshParts()).toBe(true)
  })
})

describe('sendCall', () => {
  it('sends a call as an action, and gives the result when the PC answers', async () => {
    await refresh()
    calls = []
    const r = await sendCall(7, { method: 'POST', path: '/api/v1/till/customers/1/payment', body: { amount: '50' }, cashOut: '50', label: 'Credit' })
    expect(r?.status).toBe('applied')
    const sent = JSON.parse(String(calls.find((c) => c.url.includes('/actions'))?.init?.body))
    expect(sent.actions[0]).toMatchObject({ kind: 'call', userId: 7, body: { method: 'POST', path: '/api/v1/till/customers/1/payment', body: { amount: '50' }, cashOut: '50', label: 'Credit' } })
  })

  it('waits offline, and says so with null', async () => {
    online = false
    await refresh()
    expect(await sendCall(7, { method: 'POST', path: '/api/v1/till/x', label: 'Y' })).toBeNull()
    expect((await pending()).at(-1)).toMatchObject({ kind: 'call' })
  })

  it('names a temporary reference', () => {
    expect(tempRef('abc')).toBe('temp:abc')
  })
})
