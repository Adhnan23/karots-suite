import { useSuspenseQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import type { SellLayout } from '@/api/types.gen'
import { configQuery } from '../api'
import { gridOf, pickLayout } from './arrangements'

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
export function useSellLayout(): { layout: SellLayout | null; grid: ReturnType<typeof gridOf> } {
  const cfg = useSuspenseQuery(configQuery).data
  const [w, h] = useWindowSize()
  const picked = pickLayout(cfg.sellLayouts, w, h)
  const grid = picked ? gridOf(picked) : null
  return grid ? { layout: picked, grid } : { layout: null, grid: null }
}
