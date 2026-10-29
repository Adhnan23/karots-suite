import { Printer } from 'lucide-react'
import { type ReactNode, useEffect, useRef, useState } from 'react'
import { PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { RangePicker } from '@/features/counter/RangePicker'
import type { Range } from '@/features/counter/range'
import { ExportMenu } from './ExportMenu'

/** A paged row list (a useInfiniteQuery result): Print loads the rest first. */
export type Loadable = { hasNextPage: boolean; fetchNextPage: () => Promise<Loadable> }

/** Every report's frame:
 * title, Export (csv / xlsx / ods of every row for the live filters) and Print
 * in the head; the period and the report's own filters underneath. `range`
 * omitted = a live (not date-ranged) report. `exportUrl` is the report's API
 * URL with its query string; the format is added here. `rows` is the report's
 * paged list: Print (and Ctrl+P) loads every row first, so paper and PDF hold
 * them all, not only the 50 on screen. */
export function ReportPage({ title, sub, range, onRange, caption, filters, exportUrl, rows, children }: {
  title: string; sub: ReactNode; range?: Range; onRange?: (r: Range) => void; caption?: string
  filters?: ReactNode; exportUrl?: string; rows?: Loadable; children: ReactNode
}) {
  const [loading, setLoading] = useState(false)
  const latest = useRef(rows)
  useEffect(() => { latest.current = rows })
  async function print() {
    setLoading(true)
    try {
      // ponytail: one page per request; a range of thousands of rows takes a moment
      for (let r = latest.current; r?.hasNextPage;) r = await r.fetchNextPage()
    } finally {
      setLoading(false)
    }
    await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r))) // let the rows render
    window.print()
  }
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'p' && latest.current?.hasNextPage) {
        e.preventDefault()
        void print()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])
  return (
    <>
      <PageHead title={title} sub={sub}
        actions={(
          <>
            <Button variant="outline" className="h-10 gap-1.5 px-4 print:hidden" disabled={loading} onClick={() => void print()}><Printer size={16} /> {loading ? 'Loading every row…' : 'Print'}</Button>
            {exportUrl && <ExportMenu url={exportUrl} />}
          </>
        )} />
      {(range || filters) && (
        <div className="mb-4 grid gap-2 md:flex md:flex-wrap md:items-center print:hidden">
          {range && onRange && <RangePicker full noLatest value={range} onChange={(r) => onRange({ ...r, preset: r.preset || 'this-month' })} />}
          {filters}
          {caption && <small className="text-[13px] text-muted-foreground md:ml-auto">{caption}</small>}
        </div>
      )}
      {caption && <p className="mb-3 hidden text-sm print:block">{caption}</p>}
      <div className="flex flex-col gap-4">{children}</div>
    </>
  )
}
