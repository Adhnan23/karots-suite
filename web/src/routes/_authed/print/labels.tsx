import { searchStr } from '@/lib/search'
import { useQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import type { LabelRequest } from '@/api/types.gen'
import { LabelSvg, useLabelLayout } from '@/features/counter/labels/LabelPreview'
import { PrintFrame } from '@/features/counter/receipts/PrintFrame'
import { DOTS_PER_MM, paperOf } from '@/features/print/labels/kit'
import { LabelPage, savedLabelStyle } from '@/features/print/labels/picture'
import { toPriceLabel } from '@/features/print/labels/price'
import { configQuery } from '@/features/till/api'

const str = searchStr
/** CSS px per printer dot: 96 px to the inch, 25.4 mm to the inch, 8 dots to the mm. */
const PX_PER_DOT = 96 / 25.4 / DOTS_PER_MM

/** A browser-printable label sheet (karots-pos /admin/labels/print): the same
 * true-to-print label, `count` times, at real size — for shops printing labels
 * on an ordinary printer. Image mode draws the shop's label style as vector. */
export const Route = createFileRoute('/_authed/print/labels')({
  validateSearch: (s: Record<string, unknown>): LabelRequest => ({
    productId: Number(s.productId) || 0, code: str(s.code), format: str(s.format), top: str(s.top), bottom: str(s.bottom), topKey: str(s.topKey), bottomKey: str(s.bottomKey),
    count: Math.min(200, Math.max(1, Number(s.count) || 1)), size: str(s.size),
  }),
  component: function LabelSheet() {
    const req = Route.useSearch()
    const cfg = useQuery(configQuery).data
    const image = cfg?.labelPrintAs === 'image'
    const { l, bars } = useLabelLayout(req)
    const paper = paperOf(req.size, cfg?.labelSize)
    const one = (i: number) => image && cfg
      ? (
          <div key={i} aria-label={`Label ${i + 1}`} role="img" className="break-inside-avoid overflow-hidden outline outline-1 outline-[#ccc] print:outline-none" style={{ width: `${paper.wMm}mm`, height: `${paper.hMm}mm` }}>
            <div style={{ transform: `scale(${PX_PER_DOT})`, transformOrigin: '0 0' }}>
              <LabelPage kind="price" data={toPriceLabel(req, cfg)} paper={paper} style={savedLabelStyle('price')} />
            </div>
          </div>
        )
      : l && <LabelSvg key={i} l={l} bars={bars} label={`Label ${i + 1}`} className="break-inside-avoid outline outline-1 outline-[#ccc] print:outline-none" style={{ width: `${l.widthMm}mm`, height: `${l.heightMm}mm` }} />
    return (
      <PrintFrame title={`${req.top || req.code} — ${req.count} labels`}>
        {cfg && (image || l) && (
          <div className="mx-auto flex max-w-[190mm] flex-wrap gap-[2mm] bg-paper p-[4mm] print:max-w-none print:p-0">
            {Array.from({ length: req.count }, (_, i) => one(i))}
          </div>
        )}
      </PrintFrame>
    )
  },
})
