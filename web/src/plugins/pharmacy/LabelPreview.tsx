import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useDeferredValue } from 'react'
import { api } from '@/api/client'
import type { LabelLayout } from '@/api/types.gen'
import { LabelSvg } from '@/features/counter/labels/LabelPreview'

/** A dose label exactly as the label printer lays it out (the shop's label size) —
 * with a warning when the words don't fit. Works with no printer attached. */
export function DoseLabelPreview({ lines, className }: { lines: string[]; className?: string }) {
  const deferred = useDeferredValue(lines)
  const ok = deferred.some((l) => l.trim())
  const l = useQuery({
    queryKey: ['till', 'text-label-preview', deferred],
    queryFn: () => api<LabelLayout>('/till/labels/text/preview', { json: { lines: deferred, count: 1, size: '' } }),
    placeholderData: keepPreviousData,
    enabled: ok,
  }).data
  if (!ok || !l) return null
  return (
    <figure className={className ?? 'flex flex-col items-start gap-1.5'}>
      <LabelSvg l={l} bars={null} label="Dose label" className="w-[min(260px,100%)] rounded-md bg-paper shadow-[var(--shadow)]" />
      <figcaption className="text-[12px] text-muted-foreground">
        {l.widthMm} × {l.heightMm} mm · as it prints
        {l.cut && <b className="ml-1.5 text-danger">— doesn’t fit: shorten the words</b>}
      </figcaption>
    </figure>
  )
}
