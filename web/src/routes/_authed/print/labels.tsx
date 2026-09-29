import { searchStr } from '@/lib/search'
import { createFileRoute } from '@tanstack/react-router'
import type { LabelRequest } from '@/api/types.gen'
import { LabelSvg, useLabelLayout } from '@/features/counter/labels/LabelPreview'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'

const str = searchStr

/** A browser-printable label sheet (karots-pos /admin/labels/print): the same
 * true-to-print label, `count` times, at real size — for shops printing labels
 * on an ordinary printer. */
export const Route = createFileRoute('/_authed/print/labels')({
  validateSearch: (s: Record<string, unknown>): LabelRequest => ({
    productId: Number(s.productId) || 0, code: str(s.code), format: str(s.format), top: str(s.top), bottom: str(s.bottom),
    count: Math.min(200, Math.max(1, Number(s.count) || 1)), size: str(s.size),
  }),
  component: function LabelSheet() {
    const req = Route.useSearch()
    const { l, bars } = useLabelLayout(req)
    return (
      <PrintFrame title={`${req.top || req.code} — ${req.count} labels`}>
        {l && (
          <div className="mx-auto flex max-w-[190mm] flex-wrap gap-[2mm] bg-paper p-[4mm] print:max-w-none print:p-0">
            {Array.from({ length: req.count }, (_, i) => (
              <LabelSvg key={i} l={l} bars={bars} label={`Label ${i + 1}`} className="break-inside-avoid outline outline-1 outline-[#ccc] print:outline-none"
                style={{ width: `${l.widthMm}mm`, height: `${l.heightMm}mm` }} />
            ))}
          </div>
        )}
      </PrintFrame>
    )
  },
})
