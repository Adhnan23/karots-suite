import { keepPreviousData, useQuery } from '@tanstack/react-query'
import JsBarcode from 'jsbarcode'
import { useDeferredValue, useMemo } from 'react'
import { api } from '@/api/client'
import type { LabelLayout, LabelRequest } from '@/api/types.gen'

const JS_FORMAT: Record<string, string> = { '': 'CODE128', CODE128: 'CODE128', EAN13: 'EAN13', EAN8: 'EAN8', UPC: 'UPC', CODE39: 'CODE39' }

/** Bars (with their digits) as inline SVG markup, drawn at the printer's module
 * width so the preview's bar width matches the sticker. Inline, not a data-URI
 * <image>: it draws at once and prints in every browser. null = not valid for the format. */
function barcodeImage(l: LabelLayout): { inner: string; w: number; h: number } | null {
  const b = l.barcode
  if (!b.code) return null
  try {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
    JsBarcode(svg, b.code, {
      format: JS_FORMAT[b.format] ?? 'CODE128', width: b.narrow, height: b.height, margin: 0,
      displayValue: true, fontSize: b.readableH - 6, textMargin: 2, font: 'monospace', background: 'transparent', lineColor: getComputedStyle(document.documentElement).getPropertyValue('--ink').trim() || 'black',
    })
    return {
      inner: svg.innerHTML, // JsBarcode builds text nodes, so the code is escaped here
      w: parseFloat(svg.getAttribute('width') ?? '0'), h: parseFloat(svg.getAttribute('height') ?? '0'),
    }
  } catch {
    return null
  }
}

/** Where the printer puts everything for this request (server geometry, 8 dots/mm) and the bars image. */
export function useLabelLayout(req: LabelRequest) {
  const deferred = useDeferredValue(req)
  const l = useQuery({
    queryKey: ['till', 'label-preview', deferred],
    queryFn: () => api<LabelLayout>('/till/labels/preview', { json: deferred }),
    placeholderData: keepPreviousData,
  }).data
  const bars = useMemo(() => (l ? barcodeImage(l) : null), [l])
  return { l, bars }
}

/** One label drawn from its layout; size it with className/style (the sheet uses real mm). */
export function LabelSvg({ l, bars, className, style, label = 'Label preview' }: {
  l: LabelLayout
  bars: ReturnType<typeof barcodeImage>
  className?: string
  style?: React.CSSProperties
  label?: string
}) {
  return (
      <svg aria-label={label} viewBox={`0 0 ${l.widthDots} ${l.heightDots}`} className={className} style={{ aspectRatio: `${l.widthDots} / ${l.heightDots}`, ...style }}>
        {l.texts.map((t, i) => (
          <text key={i} x={t.x} y={t.y + t.glyphH} fontFamily="monospace" fontSize={t.glyphH} fill="var(--ink)" fontWeight={700}
            textLength={t.value.length * t.charW} lengthAdjust="spacingAndGlyphs">{t.value}</text>
        ))}
        {bars
          ? <g transform={`translate(${l.barcode.x} ${l.barcode.y})`} dangerouslySetInnerHTML={{ __html: bars.inner }} />
          : l.barcode.code && <text x={l.widthDots / 2} y={l.barcode.y + 40} textAnchor="middle" fontSize={20} fill="var(--danger)">not a valid {l.barcode.format || 'barcode'}</text>}
      </svg>
  )
}

/** The label as the printer will lay it out. */
export function LabelPreview({ req }: { req: LabelRequest }) {
  const { l, bars } = useLabelLayout(req)
  if (!l) return <div className="h-32" />
  return (
    <figure className="flex flex-col items-center gap-2">
      <LabelSvg l={l} bars={bars} className="w-[min(300px,80%)] rounded-md bg-paper shadow-[var(--shadow)]" />
      <figcaption className="text-[12px] text-muted-foreground">{l.widthMm} × {l.heightMm} mm · as it prints</figcaption>
    </figure>
  )
}
