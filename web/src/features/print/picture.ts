import { createElement, type ReactElement } from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { SlipPreview } from '@/api/types.gen'
import { queryClient } from '@/api/query-client'
import { DecodedSlip } from './DecodedSlip'
import { rasterize } from './raster'

export type SlipPicture = { blob: Blob; url: string; dots: number }

const withQuery = (path: string, q: Record<string, string | undefined>) => {
  const p = new URLSearchParams(Object.entries(q).filter((e): e is [string, string] => !!e[1]))
  return `${path}?${p}`
}

/** Draws a node off screen, waits for it to settle and rasterises it. */
async function shoot(el: ReactElement, dots: number): Promise<Blob> {
  const host = document.createElement('div')
  host.style.cssText = 'position:fixed;left:-10000px;top:0;pointer-events:none'
  document.body.appendChild(host)
  const root = createRoot(host)
  try {
    flushSync(() => root.render(el))
    return await rasterize(host.firstElementChild as HTMLElement, dots)
  } finally {
    root.unmount()
    host.remove()
  }
}

async function make(path: string, size?: string): Promise<SlipPicture> {
  const preview = await api<SlipPreview>(withQuery(path, { preview: '1', size }), { method: 'POST' })
  const blob = await shoot(createElement(DecodedSlip, { preview }), preview.dots)
  // ponytail: object URLs live as long as the page; a till prints a few hundred a day.
  return { blob, url: URL.createObjectURL(blob), dots: preview.dots }
}

/** The slip at `path` as the exact picture the printer gets. Cached briefly, so
 * the Print / Skip preview and the print that follows send the same pixels. */
export const slipPictureQuery = (path: string, size?: string) =>
  queryOptions({ queryKey: ['slip-picture', path, size ?? ''], queryFn: () => make(path, size), staleTime: 60_000, retry: false })
export const slipPicture = (path: string, size?: string) => queryClient.fetchQuery(slipPictureQuery(path, size))
