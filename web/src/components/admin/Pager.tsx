import { ChevronLeft, ChevronRight } from 'lucide-react'

/** "21–40 of 132" with previous / next. Hidden when everything fits one page. */
export function Pager({ page, pageSize, total, onPage }: { page: number; pageSize: number; total: number; onPage: (p: number) => void }) {
  if (total <= pageSize) return null
  const last = Math.ceil(total / pageSize)
  const btn = 'grid h-9 w-9 place-items-center rounded-full border border-border bg-card enabled:hover:bg-muted disabled:opacity-40'
  return (
    <div className="flex items-center justify-end gap-2 border-t border-border px-4 py-2.5 text-[13px] text-muted-foreground">
      <span className="tabular-nums">{(page - 1) * pageSize + 1}–{Math.min(page * pageSize, total)} of {total}</span>
      <button type="button" aria-label="Previous page" disabled={page <= 1} onClick={() => onPage(page - 1)} className={btn}><ChevronLeft size={16} /></button>
      <button type="button" aria-label="Next page" disabled={page >= last} onClick={() => onPage(page + 1)} className={btn}><ChevronRight size={16} /></button>
    </div>
  )
}
