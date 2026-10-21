import { createElement, type ReactElement } from 'react'
import { createRoot } from 'react-dom/client'
import { flushSync } from 'react-dom'
import { queryOptions } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { SlipData, SlipPreview } from '@/api/types.gen'
import { queryClient } from '@/api/query-client'
import { DecodedSlip } from './DecodedSlip'
import { rasterize } from './raster'
import { slipElement } from './templates'
import { styleOf } from './styles'
import { PAPER_FONTS } from './paper'
import { configQuery } from '@/features/till/api'

export type SlipPicture = { blob: Blob; url: string; dots: number }

const withQuery = (path: string, q: Record<string, string | undefined>) => {
  const p = new URLSearchParams(Object.entries(q).filter((e): e is [string, string] => !!e[1]))
  return `${path}?${p}`
}

/** Draws a node off screen, waits for it to settle and rasterises it. */
async function shoot(el: ReactElement, dots: number, fonts: readonly string[]): Promise<Blob> {
  const host = document.createElement('div')
  host.style.cssText = 'position:fixed;left:-10000px;top:0;pointer-events:none'
  document.body.appendChild(host)
  const root = createRoot(host)
  try {
    flushSync(() => root.render(el))
    return await rasterize(host.firstElementChild as HTMLElement, dots, fonts)
  } finally {
    root.unmount()
    host.remove()
  }
}

// The slip's page: its kind's template in the style, fed by ?data=1, else its
// decoded Classic lines. One call asks for both: a path that has no ?data=1
// answers its preview (never a print), and the kind comes from the server, not the path.
async function page(path: string, size: string | undefined, style: string): Promise<{ el: ReactElement; dots: number; fonts: readonly string[] }> {
  const res = await api<SlipData | SlipPreview>(withQuery(path, { data: '1', preview: '1', size }), { method: 'POST' })
  if ('kind' in res) {
    const el = slipElement(res, style)
    if (el) return { el, dots: res.dots, fonts: styleOf(style).kit.fonts }
  }
  const preview = 'lines' in res ? res : await api<SlipPreview>(withQuery(path, { preview: '1', size }), { method: 'POST' })
  return { el: createElement(DecodedSlip, { preview }), dots: preview.dots, fonts: PAPER_FONTS }
}

async function make(path: string, size: string | undefined, style: string): Promise<SlipPicture> {
  const { el, dots, fonts } = await page(path, size, style)
  const blob = await shoot(el, dots, fonts)
  // ponytail: object URLs live as long as the page; a till prints a few hundred a day.
  return { blob, url: URL.createObjectURL(blob), dots }
}

/** The shop's saved receipt style (the one that prints). */
const savedStyle = () => queryClient.getQueryData(configQuery.queryKey)?.receiptStyle ?? 'classic'

/** The slip at `path` as the exact picture the printer gets, in the shop's style
 * (`style` only for the Appearance preview — never printed). Cached briefly, so
 * the Print / Skip preview and the print that follows send the same pixels; the
 * key carries the style, so a new style draws a new picture. */
export const slipPictureQuery = (path: string, size?: string, style = savedStyle()) =>
  queryOptions({ queryKey: ['slip-picture', path, size ?? '', style], queryFn: () => make(path, size, style), staleTime: 60_000, retry: false })
export const slipPicture = (path: string, size?: string) => queryClient.fetchQuery(slipPictureQuery(path, size))
