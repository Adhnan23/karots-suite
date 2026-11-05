import { onlineManager } from '@tanstack/react-query'
import { loadWho } from '../session'
import { loadCopy, loadParts } from '../store/copy'
import { pending, type Action } from '../store/outbox'
import { queueAction, sendNow, state, type CallInput } from '../sync'
import type { TillCopy } from '@/api/types.gen'

/** What something made offline is called until the PC gives it a real id: a number,
 * because plugin screens only load a record with a positive one, and far above any
 * real row. The same ref always gives the same number. */
export function tempNumber(ref: string): number {
  return 9e15 + parseInt(ref.replace(/[^0-9a-f]/gi, '').slice(0, 10) || '0', 16) % 7e12
}

const TEMP_NUMBER = /\/9\d{15}(\/|$)/
const API = '/api/v1'
const OWN = `${API}/x/pocketcashier` // the phone's own calls always go to the PC

export type RouteCtx = {
  parts: Record<string, unknown>
  copy: TillCopy | null
  params: Record<string, string>
  query: URLSearchParams
  body: unknown
  ref: string
  out: Action[]
  userId: number
}

/** How a plugin's or the till's call is answered with the PC down. `queue` also
 * saves it to send when the PC is back; `call` adds what the PC needs to settle it
 * (cash that moved, the balance shown). */
export type OfflineRoute = {
  method: string
  path: string
  answer: (ctx: RouteCtx) => unknown
  queue?: boolean
  label?: (ctx: RouteCtx) => string
  call?: (ctx: RouteCtx) => Partial<CallInput>
}

/** A route's "no", as the PC would say it. */
export class RouteError extends Error {}

type Compiled = OfflineRoute & { re: RegExp; names: string[] }

let routes: Compiled[] = []

export function addRoutes(list: OfflineRoute[]) {
  for (const r of list) {
    const names: string[] = []
    const re = new RegExp(`^${r.path.replace(/:[a-zA-Z]+/g, (m) => { names.push(m.slice(1)); return '([^/]+)' })}$`)
    routes.push({ ...r, re, names })
  }
}

export function clearRoutesForTest() { routes = [] }

function find(method: string, path: string): { route: Compiled; params: Record<string, string> } | null {
  for (const route of routes) {
    if (route.method !== method) continue
    const m = route.re.exec(path)
    if (m) return { route, params: Object.fromEntries(route.names.map((n, i) => [n, decodeURIComponent(m[i + 1])])) }
  }
  return null
}

function bodyOf(init?: RequestInit): unknown {
  if (typeof init?.body !== 'string') return undefined
  try {
    return JSON.parse(init.body)
  } catch {
    return undefined
  }
}

const reply = (v: unknown) => new Response(JSON.stringify(v ?? null), { status: 200, headers: { 'Content-Type': 'application/json' } })

const refusal = (message: string) => new Response(JSON.stringify({ error: { code: 'VALIDATION', message } }), { status: 422, headers: { 'Content-Type': 'application/json' } })

/** A write always goes through the outbox, with a one-time ref: if the PC's reply is
 * lost, the retry finds the first result instead of doing it twice. While the PC
 * answers, the screen gets the route's own reply. */
async function answer(hit: NonNullable<ReturnType<typeof find>>, method: string, url: URL, init?: RequestInit): Promise<Response> {
  const body = bodyOf(init)
  const ctx: RouteCtx = {
    parts: await loadParts(), copy: await loadCopy(), params: hit.params, query: url.searchParams, body,
    ref: crypto.randomUUID(), out: await pending(), userId: loadWho()?.id ?? 0,
  }
  let local: unknown
  try {
    local = await hit.route.answer(ctx)
  } catch (e) {
    if (!(e instanceof RouteError)) throw e
    return refusal(e.message)
  }
  if (!hit.route.queue) return reply(local)
  const call: CallInput = { method, path: url.pathname, body, label: hit.route.label?.(ctx) ?? hit.route.path, tempId: tempNumber(ctx.ref), ...hit.route.call?.(ctx) }
  const action = { ref: ctx.ref, kind: 'call' as const, userId: ctx.userId, body: call }
  if (!state().live) {
    await queueAction(action)
    return reply(local)
  }
  const r = await sendNow(action)
  if (r?.status === 'failed') return refusal(r.note || 'The PC could not do this')
  return reply(r?.reply ?? local)
}

/** Lets the plugins' till screens run unchanged on the phone: while the PC answers
 * their calls go to it, otherwise they're answered from the phone's copy. Returns
 * uninstall.
 * ponytail: wrapping fetch keeps core and the plugins untouched; a seam in api() if another caller needs it. */
export function installOfflineFetch(): () => void {
  // Queries pause with no network by default, and in a cut the router is off too.
  onlineManager.setEventListener(() => undefined)
  onlineManager.setOnline(true)
  const real = window.fetch
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(input instanceof Request ? input.url : String(input), window.location.href)
    const method = (init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase()
    const path = url.pathname
    if (!path.startsWith(`${API}/`) || path.startsWith(`${OWN}/`)) return real(input, init)
    const hit = find(method, path.slice(API.length))
    if (!hit) return real(input, init)
    const local = () => answer(hit, method, url, init)
    // A write goes through the outbox. A record with a temporary number is only here. So
    // are reads when the phone's session isn't this person's: signed in during a cut, the
    // cookie is someone else's or none, and their drawer mustn't be read for this one's.
    if (hit.route.queue || TEMP_NUMBER.test(path) || !state().live || !loadWho()?.session) return local()
    try {
      const res = await real(input, init)
      if (res.status !== 401 && res.status !== 403) return res
    } catch (e) {
      if ((e as Error).name === 'AbortError') throw e
    }
    return local()
  }
  return () => { window.fetch = real }
}
