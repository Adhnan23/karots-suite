import { useEffect, useRef } from 'react'

export const KONAMI = ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a']

/** Rolling window of the last 10 keys — unlike a position counter, an extra
 *  ArrowUp (↑↑↑↓↓…) or any stray key before the sequence can't break it. */
export function konamiMatcher(onMatch: () => void) {
  let recent: string[] = []
  const target = KONAMI.join()
  return (key: string) => {
    const k = key.length === 1 ? key.toLowerCase() : key
    recent = [...recent, k].slice(-KONAMI.length)
    if (recent.join() === target) {
      recent = []
      onMatch()
    }
  }
}

/** Listens on window while enabled. Used by login (install id) and kiosk exit. */
export function useKonami(onMatch: () => void, enabled = true) {
  const cb = useRef(onMatch)
  cb.current = onMatch
  useEffect(() => {
    if (!enabled) return
    const feed = konamiMatcher(() => cb.current())
    const onKey = (e: KeyboardEvent) => feed(e.key)
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [enabled])
}
