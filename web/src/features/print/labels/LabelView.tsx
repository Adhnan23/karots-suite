import { useQuery } from '@tanstack/react-query'
import { useEffect, type ReactNode } from 'react'
import { configQuery } from '@/features/till/api'
import { paperOf } from './kit'
import { labelPictureQuery } from './picture'

/** A label preview: in Image mode the exact picture that prints (`style` only to
 * preview an unsaved choice), in Classic `classic` (the printer-geometry SVG).
 * `mode` overrides the saved switch (Settings previews before Save). */
export function LabelView({ kind, data, size, classic, mode, style, className = 'w-[min(300px,80%)]', onCut }: {
  kind: string
  data: unknown
  size: string
  classic: ReactNode
  mode?: string
  style?: string
  className?: string
  onCut?: (cut: boolean) => void
}) {
  const cfg = useQuery(configQuery).data
  const image = (mode ?? cfg?.labelPrintAs) === 'image'
  const paper = paperOf(size, cfg?.labelSize)
  const q = labelPictureQuery(kind, data, paper, style)
  const pic = useQuery({ ...q, enabled: image && !!cfg }).data
  const cut = image && !!pic?.cut
  useEffect(() => onCut?.(cut), [cut, onCut])
  if (!image) return <>{classic}</>
  return (
    <figure className="flex flex-col items-center gap-2">
      {pic
        ? <img src={pic.url} alt="Label as it prints" className={`${className} rounded-md bg-paper shadow-[var(--shadow)] [image-rendering:pixelated]`} style={{ aspectRatio: `${paper.w} / ${paper.h}` }} />
        : <div className={`${className} rounded-md bg-paper`} style={{ aspectRatio: `${paper.w} / ${paper.h}` }} />}
      <figcaption className="text-[12px] text-muted-foreground">{paper.wMm} × {paper.hMm} mm · as it prints</figcaption>
    </figure>
  )
}
