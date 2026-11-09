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
  return `${path}${path.includes('?') ? '&' : '?'}${p}`
}

/** Draws a node off screen, waits for it to settle and rasterises it (`rows`:
 * exactly that tall; `inspect` looks at the drawn node before it goes). */
export async function shoot(el: ReactElement, dots: number, fonts: readonly string[], rows?: number, inspect?: (node: HTMLElement) => void, out = dots): Promise<Blob> {
  // Out of React's own render first: a query started while React commits would
  // make flushSync a no-op and leave nothing to draw.
  await Promise.resolve()
  const host = document.createElement('div')
  host.style.cssText = 'position:fixed;left:-10000px;top:0;pointer-events:none'
  // The till zooms itself to fit its screen (lib/autofit): undo that here, or the slip is
  // measured at one size and drawn at another, and "TOTAL" no longer fits its box.
  host.style.setProperty('zoom', String(1 / (parseFloat(getComputedStyle(document.documentElement).zoom) || 1)))
  document.body.appendChild(host)
  const root = createRoot(host)
  try {
    flushSync(() => root.render(el))
    const node = host.firstElementChild as HTMLElement
    const blob = await rasterize(node, dots, fonts, rows, out)
    inspect?.(node)
    return blob
  } finally {
    root.unmount()
    host.remove()
  }
}

// The slip's page: its kind's template in the style, fed by ?data=1, else its
// decoded Classic lines. One call asks for both: a path that has no ?data=1
// answers its preview (never a print), and the kind comes from the server, not the path.
// Bigger text (`text` percent): the template lays the slip out on a narrower page
// (fewer dots and columns, so lines wrap instead of overflowing) and the picture
// is scaled up to the paper. Decoded Classic lines keep their size: their columns are fixed.
async function page(path: string, size: string | undefined, style: string, text: number): Promise<{ el: ReactElement; dots: number; out: number; fonts: readonly string[] }> {
  const res = await api<SlipData | SlipPreview>(withQuery(path, { data: '1', preview: '1', size }), { method: 'POST' })
  if ('kind' in res) {
    const narrow = { ...res, dots: Math.round((res.dots * 100) / text), columns: Math.floor((res.columns * 100) / text) }
    const el = slipElement(narrow, style)
    if (el) return { el, dots: narrow.dots, out: res.dots, fonts: styleOf(style).kit.fonts }
  }
  const preview = 'lines' in res ? res : await api<SlipPreview>(withQuery(path, { preview: '1', size }), { method: 'POST' })
  return { el: createElement(DecodedSlip, { preview }), dots: preview.dots, out: preview.dots, fonts: PAPER_FONTS }
}

async function make(path: string, size: string | undefined, style: string, text: number): Promise<SlipPicture> {
  const { el, dots, out, fonts } = await page(path, size, style, text)
  const blob = await shoot(el, dots, fonts, undefined, undefined, out)
  // ponytail: object URLs live as long as the page; a till prints a few hundred a day.
  return { blob, url: URL.createObjectURL(blob), dots: out }
}

/** The shop's saved receipt style and text size (the ones that print). */
const savedStyle = () => queryClient.getQueryData(configQuery.queryKey)?.receiptStyle ?? 'classic'
const savedText = () => queryClient.getQueryData(configQuery.queryKey)?.receiptTextSize || 100

/** The slip at `path` as the exact picture the printer gets, in the shop's style
 * (`style` only for the Appearance preview — never printed). Cached briefly, so
 * the Print / Skip preview and the print that follows send the same pixels; the
 * key carries the style and text size, so a change draws a new picture. */
export const slipPictureQuery = (path: string, size?: string, style = savedStyle(), text = savedText()) =>
  queryOptions({ queryKey: ['slip-picture', path, size ?? '', style, text], queryFn: () => make(path, size, style, text), staleTime: 60_000, retry: false })
export const slipPicture = (path: string, size?: string) => queryClient.fetchQuery(slipPictureQuery(path, size))
