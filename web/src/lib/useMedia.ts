import { useSyncExternalStore } from 'react'

/** Live media-query match (e.g. '(max-width: 767px)'). */
export function useMedia(query: string): boolean {
  return useSyncExternalStore(
    (cb) => {
      const m = window.matchMedia(query)
      m.addEventListener('change', cb)
      return () => m.removeEventListener('change', cb)
    },
    () => window.matchMedia(query).matches,
  )
}

export const useIsPhone = () => useMedia('(max-width: 767px)')
