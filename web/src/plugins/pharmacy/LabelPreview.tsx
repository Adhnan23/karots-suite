import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useDeferredValue, useState } from 'react'
import { api } from '@/api/client'
import type { LabelLayout } from '@/api/types.gen'
import { LabelSvg } from '@/features/counter/labels/LabelPreview'
import { LabelView } from '@/features/print/labels/LabelView'
import { doseLabel, type DoseLabel } from './label'

const CUT = <b className="ml-1.5 text-danger">— doesn’t fit: shorten the words</b>

/** A dose label as it prints (the shop's label size): the picture in Image mode,
 * the printer's text layout in Classic — with a warning when the words don't fit.
 * Works with no printer attached. */
export function DoseLabelPreview({ d, className }: { d: DoseLabel; className?: string }) {
  const [cut, setCut] = useState(false)
  return (
    <div className={className ?? 'flex flex-col items-start gap-1.5'}>
      <LabelView kind="pharmacy.dose" data={d} size="" className="w-[min(260px,100%)]" onCut={setCut} classic={<ClassicPreview lines={doseLabel(d)} />} />
      {cut && <p className="text-[12px]">{CUT}</p>}
    </div>
  )
}

function ClassicPreview({ lines }: { lines: string[] }) {
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
    <figure className="flex flex-col items-start gap-1.5">
      <LabelSvg l={l} bars={null} label="Dose label" className="w-[min(260px,100%)] rounded-md bg-paper shadow-[var(--shadow)]" />
      <figcaption className="text-[12px] text-muted-foreground">
        {l.widthMm} × {l.heightMm} mm · as it prints
        {l.cut && CUT}
      </figcaption>
    </figure>
  )
}
