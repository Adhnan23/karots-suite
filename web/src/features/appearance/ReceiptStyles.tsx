import { useQueries, useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { slipPictureQuery } from '@/features/print/picture'
import { STYLES } from '@/features/print/styles'
import { configQuery } from '@/features/till/api'
import { slots } from '@/plugins/slots'

const SAMPLE = '/admin/settings/sample-slip'

const CORE_SLIPS = [
  { label: 'Sale', path: SAMPLE },
  { label: 'Refund', path: `${SAMPLE}?kind=refund` },
  { label: 'Cash in/out', path: `${SAMPLE}?kind=money` },
  { label: 'Credit', path: `${SAMPLE}?kind=debt` },
  { label: 'Warranty', path: `${SAMPLE}?kind=warranty` },
  { label: 'Test', path: `${SAMPLE}?kind=test` },
]

const SALES = [
  { label: 'Busy sale', key: 'busy' },
  { label: 'Simple', key: 'simple' },
  { label: 'On credit', key: 'credit' },
] as const

function Seg<T extends string>({ label, options, value, onChange }: { label: string; options: readonly { label: string; key: T }[]; value: T; onChange: (v: T) => void }) {
  return (
    <div role="group" aria-label={label} className="flex flex-wrap gap-0.5 rounded-pill bg-muted p-1">
      {options.map((o) => (
        <button
          key={o.key}
          type="button"
          aria-pressed={value === o.key}
          onClick={() => onChange(o.key)}
          className="rounded-pill px-2.5 py-1 text-xs font-semibold text-muted-foreground aria-pressed:bg-card aria-pressed:text-foreground aria-pressed:shadow-[var(--shadow)]"
        >
          {o.label}
        </button>
      ))}
    </div>
  )
}

/** Appearance → Receipt style: live thumbnails of every style and the exact
 * picture the printer would get for any slip. Only `value` is saved (by the page). */
export function ReceiptStyles({ value, onChange }: { value: string; onChange: (key: string) => void }) {
  const slips = [...CORE_SLIPS, ...slots.slipTemplates.filter((t) => t.sample).map((t) => ({ label: t.label, path: t.sample! }))]
  const [slip, setSlip] = useState(SAMPLE)
  const [size, setSize] = useState<'80' | '58'>('80')
  const [sale, setSale] = useState<(typeof SALES)[number]['key']>('busy')
  const path = slip === SAMPLE ? `${SAMPLE}?kind=sale&sample=${sale}` : slip

  const thumbs = useQueries({ queries: STYLES.map((s) => slipPictureQuery(`${SAMPLE}?kind=sale&sample=busy`, '80', s.key)) })
  const preview = useQuery(slipPictureQuery(path, size, value))
  const { data: cfg } = useQuery(configQuery)

  return (
    <section className="flex flex-col gap-4 lg:flex-row lg:items-start">
      <div className="flex min-w-0 flex-1 flex-col gap-2">
        <span className="text-xs font-bold text-muted-foreground">Receipt style · Image mode</span>
        <div role="group" aria-label="Receipt style" className="-mx-4 flex gap-3 overflow-x-auto px-4 pb-1 sm:mx-0 sm:grid sm:grid-cols-3 sm:overflow-visible sm:px-0 xl:grid-cols-5">
          {STYLES.map((s, i) => (
            <button
              key={s.key}
              type="button"
              aria-pressed={value === s.key}
              onClick={() => onChange(s.key)}
              className="flex w-28 shrink-0 flex-col gap-1.5 rounded-[var(--radius)] border-2 border-transparent bg-card p-2 text-left shadow-[var(--shadow)] aria-pressed:border-brand sm:w-auto"
            >
              <div className="h-32 overflow-hidden rounded-sm border border-border bg-paper">
                {thumbs[i].data && <img src={thumbs[i].data.url} alt={s.label} className="w-full" />}
              </div>
              <span className="text-sm font-semibold">{s.label}</span>
            </button>
          ))}
        </div>
        {cfg?.printAs === 'classic' && (
          <p className="rounded-[var(--radius-sm)] bg-warning/15 px-3 py-2 text-sm">
            This shop prints Classic — receipt styles apply once Settings → Printers → Print receipts as is set to <b>Image</b>.
          </p>
        )}
      </div>

      <div className="flex flex-col gap-2 rounded-[var(--radius)] bg-card p-3 shadow-[var(--shadow)] lg:w-[460px] lg:shrink-0">
        <span className="text-xs font-bold text-muted-foreground">Preview · exactly as it prints</span>
        <Seg label="Slip" options={slips.map((s) => ({ label: s.label, key: s.path }))} value={slip} onChange={setSlip} />
        <div className="flex flex-wrap gap-2">
          <Seg label="Roll width" options={[{ label: '80 mm', key: '80' }, { label: '58 mm', key: '58' }] as const} value={size} onChange={setSize} />
          {slip === SAMPLE && <Seg label="Sample" options={SALES} value={sale} onChange={setSale} />}
        </div>
        <div className="flex min-h-40 justify-center overflow-hidden rounded-sm bg-muted p-3">
          {preview.data ? (
            <img src={preview.data.url} alt="Receipt preview" className="h-auto max-w-full bg-paper shadow-[var(--shadow)]" style={{ width: preview.data.dots * 0.7 }} />
          ) : (
            <span className="self-center text-sm text-muted-foreground">{preview.isError ? 'The preview failed to draw' : 'Drawing…'}</span>
          )}
        </div>
      </div>
    </section>
  )
}
