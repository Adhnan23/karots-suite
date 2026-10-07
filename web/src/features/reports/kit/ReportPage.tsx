import { Download, Printer } from 'lucide-react'
import type { ReactNode } from 'react'
import { PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { RangePicker } from '@/features/counter/RangePicker'
import type { Range } from '@/features/counter/range'

/** Every report's frame (karots-pos: "set the range, then Print / Save as PDF"):
 * title, Export (csv / xlsx / ods of every row for the live filters) and Print
 * in the head; the period and the report's own filters underneath. `range`
 * omitted = a live (not date-ranged) report. `exportUrl` is the report's API
 * URL with its query string; the format is added here. */
export function ReportPage({ title, sub, range, onRange, caption, filters, exportUrl, children }: {
  title: string; sub: ReactNode; range?: Range; onRange?: (r: Range) => void; caption?: string
  filters?: ReactNode; exportUrl?: string; children: ReactNode
}) {
  const sep = exportUrl?.includes('?') ? '&' : '?'
  return (
    <>
      <PageHead title={title} sub={sub}
        actions={(
          <>
            <Button variant="outline" className="h-10 gap-1.5 px-4 print:hidden" onClick={() => window.print()}><Printer size={16} /> Print</Button>
            {exportUrl && (
              <DropdownMenu>
                <DropdownMenuTrigger asChild><Button variant="outline" className="h-10 gap-1.5 px-4 print:hidden"><Download size={16} /> Export</Button></DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  {(['csv', 'xlsx', 'ods'] as const).map((f) => (
                    <DropdownMenuItem key={f} asChild><a href={`${exportUrl}${sep}format=${f}`}>{f.toUpperCase()}</a></DropdownMenuItem>
                  ))}
                </DropdownMenuContent>
              </DropdownMenu>
            )}
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
