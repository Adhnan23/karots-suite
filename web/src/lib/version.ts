/** Follows the server's version (X-Karots-Version on every API reply): after
 * an update the page reloads itself so it runs the new SPA — but only when no
 * guard objects (the till: only with an empty cart, so a sale is never cut). */

const BANNER_MS = 1_500
const RETRY_MS = 5_000

let seen: string | null = null
let pending = false
let reloading = false
let retry: ReturnType<typeof setTimeout> | undefined
let reload = () => window.location.reload()
const guards = new Set<() => boolean>()
const listeners = new Set<() => void>()

/** True when `now` is a different version from the one this page started on
 * and every guard allows a reload. */
export function shouldReload(seenV: string | null, now: string | null, guard: () => boolean): boolean {
  return !!seenV && !!now && now !== seenV && guard()
}

const allow = () => [...guards].every((g) => g())

function tryReload() {
  if (!pending || reloading) return
  clearTimeout(retry)
  if (!allow()) {
    retry = setTimeout(tryReload, RETRY_MS)
    return
  }
  reloading = true
  listeners.forEach((l) => l())
  setTimeout(reload, BANNER_MS)
}

/** client.ts calls this with each reply's X-Karots-Version. */
export function noteVersion(v: string | null): void {
  if (!v) return
  if (seen === null) seen = v
  else if (shouldReload(seen, v, () => true)) pending = true
  tryReload()
}

/** A screen that must not reload right now registers a guard; returns its
 * unregister. */
export function setReloadGuard(fn: () => boolean): () => void {
  guards.add(fn)
  return () => {
    guards.delete(fn)
    tryReload()
  }
}

/** The banner subscribes: called once, just before the reload. */
export function onReloading(fn: () => void): () => void {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

/** Tests only: forget the state and use `fn` instead of a real reload. */
export function _reset(fn: () => void): void {
  seen = null
  pending = reloading = false
  clearTimeout(retry)
  guards.clear()
  listeners.clear()
  reload = fn
}
