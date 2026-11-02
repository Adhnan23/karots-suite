import { useRouterState } from '@tanstack/react-router'
import { useEffect } from 'react'
import { api } from '@/api/client'
import type { PresenceBeat, PresenceReply } from '@/api/types.gen'
import { holdOnMove } from '@/features/till/cart/held'
import { cartKey, loadCart } from '@/features/till/cart/persist'
import { saleTotal } from '@/features/till/cart/totals'

const EVERY = 20_000
const INPUT = ['mousemove', 'mousedown', 'keydown', 'touchstart', 'scroll'] as const
let lastInput = Date.now()
let note = ''
let kick: (() => void) | null = null
let sending = false
let again = false

/** A short word on what this screen is doing (e.g. "Playing Blocks"), sent at once and
 * with every heartbeat until cleared with ''. */
export function setPresenceNote(n: string) {
  note = n
  kick?.()
}

/** Tells the server this screen is here (the admin's "Who's on now"): on mount, on every
 * page change, when the tab comes back and every 20 s — where, idle for how long, and
 * the saved cart. Best effort: a failed beat is simply the next one's job. */
export function usePresence(userId: number) {
  const path = useRouterState({ select: (s) => s.location.pathname })
  useEffect(() => {
    // One at a time, or two close together can arrive swapped and the older note wins.
    const beat = () => {
      if (sending) { again = true; return }
      sending = true
      const cart = loadCart(userId)
      const body: PresenceBeat = {
        path: window.location.pathname,
        idleSec: Math.floor((Date.now() - lastInput) / 1000),
        cartItems: cart?.lines.reduce((n, l) => n + l.qty, 0) ?? 0,
        cartTotal: cart?.lines.length ? saleTotal(cart).toFixed(2) : '',
        note,
      }
      api<PresenceReply>('/presence', { json: body })
        .then((r) => {
          // An admin locked this screen from Who's on now.
          if (r.lock) void api('/auth/lock', { method: 'POST' }).catch(() => {}).finally(() => window.location.assign('/lock'))
          // The person signed in on another device: hold this cart, then lock here.
          else if (r.moved && !window.location.pathname.startsWith('/lock')) {
            void holdOnMove(loadCart(userId))
              .then(() => { try { sessionStorage.removeItem(cartKey(userId)) } catch { /* ignore */ } }, () => {})
              .then(() => api('/auth/lock', { method: 'POST' }).catch(() => {}))
              .finally(() => window.location.assign('/lock'))
          }
        })
        .catch(() => {})
        .finally(() => {
          sending = false
          if (again) { again = false; (kick ?? beat)() }
        })
    }
    const input = () => { lastInput = Date.now() }
    const shown = () => { if (document.visibilityState === 'visible') beat() }
    kick = beat
    beat()
    const id = setInterval(beat, EVERY)
    for (const ev of INPUT) window.addEventListener(ev, input, { passive: true })
    document.addEventListener('visibilitychange', shown)
    return () => {
      clearInterval(id)
      kick = null
      for (const ev of INPUT) window.removeEventListener(ev, input)
      document.removeEventListener('visibilitychange', shown)
    }
  }, [userId, path])
}
