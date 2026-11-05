import { loadWho } from '../session'
import { loadCopy, loadParts } from '../store/copy'
import { pending, type Action } from '../store/outbox'
import { sendNow, state, type CallInput } from '../sync'
import type { TillCopy } from '@/api/types.gen'

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

async function answerLocally(hit: NonNullable<ReturnType<typeof find>>, method: string, url: URL, init?: RequestInit): Promise<Response> {
  const body = bodyOf(init)
  const ctx: RouteCtx = {
    parts: await loadParts(), copy: await loadCopy(), params: hit.params, query: url.searchParams, body,
    ref: crypto.randomUUID(), out: await pending(), userId: loadWho()?.id ?? 0,
  }
  const answer = await hit.route.answer(ctx)
  if (hit.route.queue) {
    const call: CallInput = { method, path: url.pathname, body, label: hit.route.label?.(ctx) ?? hit.route.path, ...hit.route.call?.(ctx) }
    // Not awaited: with the PC off the send can take as long as the network gives up.
    void sendNow({ ref: ctx.ref, kind: 'call', userId: ctx.userId, body: call }).catch(() => {})
  }
  return reply(answer)
}

/** Lets the plugins' till screens run unchanged on the phone: while the PC answers
 * their calls go to it, otherwise they're answered from the phone's copy. Returns
 * uninstall.
 * ponytail: wrapping fetch keeps core and the plugins untouched; a seam in api() if another caller needs it. */
export function installOfflineFetch(): () => void {
  const real = window.fetch
  window.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(input instanceof Request ? input.url : String(input), window.location.href)
    const method = (init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase()
    const path = url.pathname
    if (!path.startsWith(`${API}/`) || path.startsWith(`${OWN}/`)) return real(input, init)
    const hit = find(method, path.slice(API.length))
    if (!hit) return real(input, init)
    if (state().live) {
      try {
        const res = await real(input, init)
        // Signed in while the PC was down, so no session yet: carry on from the copy.
        if (res.status !== 401) return res
      } catch (e) {
        if ((e as Error).name === 'AbortError') throw e
      }
    }
    return answerLocally(hit, method, url, init)
  }
  return () => { window.fetch = real }
}
