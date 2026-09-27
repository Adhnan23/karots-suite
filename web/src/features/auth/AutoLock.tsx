import { useEffect, useRef } from 'react'
import { api } from '@/api/client'

const IDLE_EVENTS = ['mousemove', 'mousedown', 'keydown', 'touchstart', 'scroll', 'click'] as const

/** Locks the session after `minutes` of no input. Off (renders nothing) when minutes <= 0. */
export function AutoLock({ minutes }: { minutes: number }) {
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)

  useEffect(() => {
    if (minutes <= 0) return

    const lock = () => {
      api('/auth/lock', { method: 'POST' })
        .catch(() => {})
        .finally(() => window.location.assign('/lock'))
    }
    const reset = () => {
      clearTimeout(timer.current)
      timer.current = setTimeout(lock, minutes * 60_000)
    }

    reset()
    for (const ev of IDLE_EVENTS) window.addEventListener(ev, reset)
    return () => {
      clearTimeout(timer.current)
      for (const ev of IDLE_EVENTS) window.removeEventListener(ev, reset)
    }
  }, [minutes])

  return null
}
