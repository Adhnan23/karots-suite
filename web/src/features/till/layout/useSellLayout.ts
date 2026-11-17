import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import type { SellLayout } from '@/api/types.gen'
import { meQuery } from '@/api/session'
import { configQuery } from '../api'
import { gridOf, pickLayout } from './arrangements'
import { readTrial } from './trial'

/** The window's size, kept current (resize, zoom, DevTools). */
function useWindowSize() {
  const [size, setSize] = useState(() => [window.innerWidth, window.innerHeight] as const)
  useEffect(() => {
    const on = () => setSize([window.innerWidth, window.innerHeight])
    window.addEventListener('resize', on)
    return () => window.removeEventListener('resize', on)
  }, [])
  return size
}

/** The layout saved for this screen size (Appearance → Sell page layout), with its
 * CSS grid; layout null = today's. A layout this build can't draw counts as none. */
export function useSellLayout(): { layout: SellLayout | null; grid: ReturnType<typeof gridOf>; trying: boolean } {
  const cfg = useSuspenseQuery(configQuery).data
  const system = useQuery(meQuery).data?.flags.isSystem ?? false
  const [w, h] = useWindowSize()
  // The system user trying an unsaved layout (Try here) sees it whatever the size.
  const trial = system ? readTrial() : null
  const picked = trial?.layout ?? pickLayout(cfg.sellLayouts, w, h)
  const grid = picked ? gridOf(picked) : null
  return grid ? { layout: picked, grid, trying: !!trial } : { layout: null, grid: null, trying: false }
}
