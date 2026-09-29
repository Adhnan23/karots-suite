import { keepPreviousData, useQuery } from '@tanstack/react-query'
import JsBarcode from 'jsbarcode'
import { useDeferredValue, useMemo } from 'react'
import { api } from '@/api/client'
import type { LabelLayout, LabelRequest } from '@/api/types.gen'

const JS_FORMAT: Record<string, string> = { '': 'CODE128', CODE128: 'CODE128', EAN13: 'EAN13', EAN8: 'EAN8', UPC: 'UPC', CODE39: 'CODE39' }

/** Bars (with their digits) as an image, drawn at the printer's module width so
 * the preview's bar width matches the sticker. null = not valid for the format. */
function barcodeImage(l: LabelLayout): { href: string; w: number; h: number } | null {
  const b = l.barcode
  if (!b.code) return null
  try {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
    JsBarcode(svg, b.code, {
      format: JS_FORMAT[b.format] ?? 'CODE128', width: b.narrow, height: b.height, margin: 0,
      displayValue: true, fontSize: b.readableH - 6, textMargin: 2, font: 'monospace', background: 'transparent', lineColor: getComputedStyle(document.documentElement).getPropertyValue('--ink').trim() || 'black',
    })
    return {
      href: `data:image/svg+xml;utf8,${encodeURIComponent(new XMLSerializer().serializeToString(svg))}`,
      w: parseFloat(svg.getAttribute('width') ?? '0'), h: parseFloat(svg.getAttribute('height') ?? '0'),
    }
  } catch {
    return null
  }
}

/** The label as the printer will lay it out (server geometry, 8 dots/mm). */
export function LabelPreview({ req }: { req: LabelRequest }) {
  const deferred = useDeferredValue(req)
  const l = useQuery({
    queryKey: ['till', 'label-preview', deferred],
    queryFn: () => api<LabelLayout>('/till/labels/preview', { json: deferred }),
    placeholderData: keepPreviousData,
  }).data
  const bars = useMemo(() => (l ? barcodeImage(l) : null), [l])
  if (!l) return <div className="h-32" />
  return (
    <figure className="flex flex-col items-center gap-2">
      <svg aria-label="Label preview" viewBox={`0 0 ${l.widthDots} ${l.heightDots}`}
        className="w-[min(300px,80%)] rounded-md bg-paper shadow-[var(--shadow)]" style={{ aspectRatio: `${l.widthDots} / ${l.heightDots}` }}>
        {l.texts.map((t, i) => (
          <text key={i} x={t.x} y={t.y + t.glyphH} fontFamily="monospace" fontSize={t.glyphH} fill="var(--ink)" fontWeight={700}
            textLength={t.value.length * t.charW} lengthAdjust="spacingAndGlyphs">{t.value}</text>
        ))}
        {bars
          ? <image href={bars.href} x={l.barcode.x} y={l.barcode.y} width={bars.w} height={bars.h} />
          : l.barcode.code && <text x={l.widthDots / 2} y={l.barcode.y + 40} textAnchor="middle" fontSize={20} fill="var(--danger)">not a valid {l.barcode.format || 'barcode'}</text>}
      </svg>
      <figcaption className="text-[12px] text-muted-foreground">{l.widthMm} × {l.heightMm} mm · as it prints</figcaption>
    </figure>
  )
}
