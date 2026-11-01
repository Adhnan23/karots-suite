import { useEffect, useRef } from 'react'

const EVENTS = ['touchstart', 'pointerdown', 'keydown', 'scroll'] as const

/** Calls onIdle after `minutes` with no touch (the shop's lock time, as the PC's
 * AutoLock). 0 = never. */
export function useIdle(minutes: number, onIdle: () => void) {
  const latest = useRef(onIdle)
  useEffect(() => { latest.current = onIdle })
  useEffect(() => {
    if (minutes <= 0) return
    let t: ReturnType<typeof setTimeout>
    const reset = () => { clearTimeout(t); t = setTimeout(() => latest.current(), minutes * 60_000) }
    reset()
    for (const ev of EVENTS) window.addEventListener(ev, reset, { passive: true, capture: true })
    return () => {
      clearTimeout(t)
      for (const ev of EVENTS) window.removeEventListener(ev, reset, { capture: true })
    }
  }, [minutes])
}
