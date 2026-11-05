import type { RecentSales, TillCopy } from '@/api/types.gen'
import { BASE, phoneFetch } from './api'
import { loadCopy, saveCopy } from './store/copy'
import { ack, addAction, pending, type Action } from './store/outbox'
import { loadPhone } from './store/phone'
import { recentCursor, saveRecent } from './store/recent'

/** What the screens show: is the PC reachable, how fresh is the copy, how many
 * actions wait to be sent. */
export type SyncState = { live: boolean; copyAt: string | null; waiting: number; lastError?: string }

/** The server's answer for one action (see the plugin's replay). */
export type ActionResult = { ref: string; status: 'applied' | 'skipped' | 'failed'; saleId?: number; receiptNo?: string; heldId?: number; note?: string }

const REFRESH_EVERY = 60_000
const LIVE_FOR = 15_000
const BATCH = 50

let lastOk = 0
let s: SyncState = { live: false, copyAt: null, waiting: 0 }
let notify: (s: SyncState) => void = () => {}
let flushing: Promise<void> | null = null
const answered = new Map<string, ActionResult>()

export const state = () => s

function set(p: Partial<SyncState>) {
  s = { ...s, ...p, live: Date.now() - lastOk < LIVE_FOR }
  notify(s)
}

const ok = () => { lastOk = Date.now() }
const down = (e: unknown) => { lastOk = 0; set({ lastError: e instanceof Error ? e.message : String(e) }) }

/** Fetches the copy when it changed (ETag = its version); a 304 means ours is current. */
export async function refresh() {
  try {
    const have = await loadCopy()
    const res = await phoneFetch('/copy', { headers: have ? { 'If-None-Match': `"${have.version}"` } : {} })
    if (res.status === 304) {
      ok()
      return set({ copyAt: new Date().toISOString(), lastError: undefined })
    }
    if (!res.ok) throw new Error(`copy: ${res.status}`)
    const c = (await res.json()) as TillCopy
    await saveCopy(c)
    ok()
    set({ copyAt: new Date().toISOString(), lastError: undefined })
  } catch (e) {
    down(e)
  }
}

let fetchingRecent: Promise<void> | null = null

/** Brings the last 30 days of sales up to date, page by page. A failure just
 * stops here (the next tick asks on from the last good page); whether the PC
 * answers is the copy's call, so selling never waits on this. */
export function refreshRecent(): Promise<void> {
  fetchingRecent ??= (async () => {
    try {
      for (;;) {
        const c = await recentCursor()
        const qs = new URLSearchParams({ after: String(c.after) })
        if (c.since) qs.set('since', c.since)
        const res = await phoneFetch(`/recent?${qs}`)
        if (!res.ok) return
        const page = (await res.json()) as RecentSales
        await saveRecent(page, new Date())
        if (!page.more) return
      }
    } catch {
      // the PC went away, or a reply that isn't a page: try again on the next tick
    } finally {
      fetchingRecent = null
    }
  })()
  return fetchingRecent
}

/** Sends what waits, oldest first, 50 at a time; every answered ref (applied,
 * skipped or failed) is acknowledged and never sent again. */
export function flush(signedIn: () => number | null): Promise<void> {
  flushing ??= (async () => {
    try {
      for (;;) {
        const all = await pending()
        set({ waiting: all.length })
        if (!all.length) return
        const batch = all.slice(0, BATCH)
        const res = await phoneFetch('/actions', { method: 'POST', body: JSON.stringify({ signedIn: signedIn(), actions: batch }) })
        if (!res.ok) throw new Error(`send: ${res.status}`)
        const { results } = (await res.json()) as { results: ActionResult[] }
        ok()
        for (const r of results) answered.set(r.ref, r)
        await ack(results.map((r) => r.ref))
      }
    } catch (e) {
      down(e)
    } finally {
      set({ waiting: (await pending()).length })
      flushing = null
    }
  })()
  return flushing
}

/** Keeps the copy fresh and the outbox empty while the app is open: refresh on
 * start, every minute and when the server says something changed; send what
 * waits whenever the PC answers. Returns stop. */
export function startSync(onState: (s: SyncState) => void, signedIn: () => number | null): () => void {
  notify = onState
  let es: EventSource | null = null
  let stopped = false
  const tick = () => {
    void refresh().then(() => (s.waiting > 0 || s.live ? flush(signedIn) : undefined))
    void refreshRecent()
  }
  void loadCopy().then((c) => { if (c && !s.copyAt) set({ copyAt: c.takenAt }) })
  // A reopened app: what still waits from before (the count, and the 5 s retry, need it).
  void pending().then((a) => set({ waiting: a.length })).catch(() => {})
  tick()
  // The feed opens with a one-time ticket (an EventSource can't send the key as a
  // header, and URLs land in logs); a dropped feed opens again with a new one.
  const openFeed = async () => {
    if (stopped || !(await loadPhone())) return
    try {
      const res = await phoneFetch('/feed-ticket', { method: 'POST' })
      if (!res.ok) throw new Error(`feed ticket: ${res.status}`)
      const { ticket } = (await res.json()) as { ticket: string }
      if (stopped) return
      es = new EventSource(`${BASE}/feed?ticket=${encodeURIComponent(ticket)}`)
      es.addEventListener('version', () => tick())
      // Opened (again): what changed while it was down sent no event.
      es.addEventListener('open', () => tick())
      es.addEventListener('error', () => { es?.close(); es = null; setTimeout(() => void openFeed(), 5_000) })
    } catch {
      setTimeout(() => void openFeed(), 15_000)
    }
  }
  void openFeed()
  const every = setInterval(tick, REFRESH_EVERY)
  // A quick look while something waits, so a sale goes the moment the PC is back.
  const retry = setInterval(() => { if (s.waiting > 0) void flush(signedIn) }, 5_000)
  return () => {
    stopped = true
    clearInterval(every)
    clearInterval(retry)
    es?.close()
    notify = () => {}
  }
}

/** Saves an action and, when the PC answers, sends it at once: its result, or
 * null when it waits in the outbox (offline). */
export async function sendNow(a: Omit<Action, 'ref' | 'at'>, signedIn: () => number | null = () => a.userId): Promise<ActionResult | null> {
  const saved = await addAction(a)
  set({ waiting: s.waiting + 1 })
  await flush(signedIn)
  // A flush already under way may have taken the outbox before this was added.
  if (!answered.has(saved.ref) && s.live) await flush(signedIn)
  const r = answered.get(saved.ref) ?? null
  answered.delete(saved.ref)
  return r
}

/** Where the signed-in person went, when they signed in on another device (the
 * PC) since: its label, or '' (still here, or the PC isn't reachable). */
export async function movedTo(userId: number): Promise<string> {
  try {
    const res = await phoneFetch(`/moved?user=${userId}`)
    if (!res.ok) return ''
    return ((await res.json()) as { movedTo?: string }).movedTo ?? ''
  } catch {
    return ''
  }
}
