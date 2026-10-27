import { useQuery } from '@tanstack/react-query'
import { TriangleAlert } from 'lucide-react'
import { useState } from 'react'
import { LABEL_SIZES } from '@/features/counter/labels/slots'
import { paperOf } from '@/features/print/labels/kit'
import { SCREEN } from '@/features/print/labels/LabelView'
import { LabelPage, labelPictureQuery } from '@/features/print/labels/picture'
import { labelKinds, styleFor } from '@/features/print/labels/registry'
import { configQuery } from '@/features/till/api'
import { Seg } from './ReceiptStyles'

/** Appearance → Labels: a style per label kind (core's price label, then each
 * built-in plugin's), with live thumbnails at a chosen size and the exact picture
 * of the chosen one. Only `value` is saved (by the page). Thumbnails draw the
 * label itself, scaled as vector (an SVG foreignObject): a shrunken 1-bit picture
 * loses its thin strokes. */
export function LabelStyles({ value, onChange }: { value: Record<string, string>; onChange: (v: Record<string, string>) => void }) {
  const kinds = labelKinds()
  const { data: cfg } = useQuery(configQuery)
  const [kindKey, setKindKey] = useState(kinds[0].kind)
  const [size, setSize] = useState(cfg?.labelSize && LABEL_SIZES.includes(cfg.labelSize) ? cfg.labelSize : LABEL_SIZES[0])
  const kind = kinds.find((k) => k.kind === kindKey) ?? kinds[0]
  const paper = paperOf(size)
  const chosen = value[kind.kind] ?? 'classic'

  const pq = useQuery(labelPictureQuery(kind.kind, kind.sample, paper, chosen))
  const preview = pq.data

  return (
    <section aria-label="Label styles" className="flex flex-col gap-4 lg:flex-row lg:items-start">
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <span className="text-xs font-bold text-muted-foreground">Label style · Image mode</span>
        <Seg label="Label kind" options={kinds.map((k) => ({ label: k.label, key: k.kind }))} value={kind.kind} onChange={setKindKey} />
        <div role="group" aria-label="Label style" className="-mx-4 flex gap-3 overflow-x-auto px-4 pb-1 sm:mx-0 sm:grid sm:grid-cols-3 sm:overflow-visible sm:px-0 xl:grid-cols-4">
          {kind.styles.map((s) => {
            const draws = styleFor(kind, s.key, paper, kind.sample).style.key
            return (
              <button
                key={s.key}
                type="button"
                aria-pressed={chosen === s.key}
                onClick={() => onChange({ ...value, [kind.kind]: s.key })}
                className="group flex w-36 shrink-0 flex-col gap-1.5 rounded-[var(--radius)] border-2 border-transparent bg-card p-2 text-left shadow-[var(--shadow)] aria-pressed:border-brand sm:w-auto"
              >
                <div className="relative">
                  <svg viewBox={`0 0 ${paper.w} ${paper.h}`} aria-hidden className="block w-full overflow-hidden rounded-sm border border-border bg-paper">
                    <foreignObject width={paper.w} height={paper.h}>
                      <LabelPage kind={kind.kind} data={kind.sample} paper={paper} style={s.key} />
                    </foreignObject>
                  </svg>
                  {draws !== s.key && (
                    // A warning in the corner; what it means shows on hover, or once the card is picked on a touch screen.
                    <span className="absolute top-1 right-1">
                      <TriangleAlert size={18} aria-hidden className="rounded-pill bg-card p-0.5 text-warning shadow-[var(--shadow)]" />
                      <span role="note" className="pointer-events-none absolute top-6 right-0 z-10 hidden w-44 rounded-[var(--radius-sm)] bg-card px-2.5 py-1.5 text-left text-[12px] font-medium text-foreground shadow-[var(--shadow)] group-hover:block pointer-coarse:group-aria-pressed:block">
                        Too small for this label size — prints as {kind.styles.find((x) => x.key === draws)?.label}
                      </span>
                    </span>
                  )}
                </div>
                <span className="text-sm font-semibold">{s.label}</span>
              </button>
            )
          })}
        </div>
        {cfg?.labelPrintAs !== 'image' && (
          <p className="rounded-[var(--radius-sm)] bg-warning/15 px-3 py-2 text-sm">
            This shop prints labels as Classic — label styles apply once Settings → Printers → Print labels as is set to <b>Image</b>.
          </p>
        )}
      </div>

      <div className="flex flex-col gap-2 rounded-[var(--radius)] bg-card p-3 shadow-[var(--shadow)] lg:w-[460px] lg:shrink-0">
        <span className="text-xs font-bold text-muted-foreground">Preview · exactly as it prints</span>
        <Seg label="Label size" options={LABEL_SIZES.map((s) => ({ label: s.replace('x', ' × '), key: s }))} value={size} onChange={setSize} />
        <div className="flex min-h-40 items-center justify-center rounded-sm bg-muted p-3">
          {preview ? (
            <img src={preview.url} alt="Label preview" data-label-style={preview.style} className="h-auto max-w-full bg-paper shadow-[var(--shadow)]" style={{ width: paper.w * SCREEN }} />
          ) : (
            <span className="text-sm text-muted-foreground" title={pq.error?.message}>{pq.isError ? 'The preview failed to draw' : 'Drawing…'}</span>
          )}
        </div>
      </div>
    </section>
  )
}
