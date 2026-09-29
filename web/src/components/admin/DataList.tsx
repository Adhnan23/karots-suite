import type { ReactNode } from 'react'
import { cn } from '@/lib/utils'
import { SURFACE } from './PageHead'

export type Column<T> = {
  key: string
  header: string
  cell: (row: T) => ReactNode
  /** Right-aligned, tabular (money, qty). */
  num?: boolean
  /** Hidden from the phone card (the card shows the first column as its title). */
  wide?: boolean
  /** Holds its own buttons: sits above the row's stretched click target. */
  interactive?: boolean
  className?: string
}

/** A list that is a table from 768px and a stack of cards below it (spec §6.3).
 * The first column is the card's title; the rest become label · value lines. */
export function DataList<T>({ rows, columns, rowKey, onRow, empty, label, footer }: {
  rows: T[]
  columns: Column<T>[]
  rowKey: (row: T) => string | number
  onRow?: (row: T) => void
  empty?: ReactNode
  label: string
  footer?: ReactNode
}) {
  const [first, ...rest] = columns
  return (
    <div className={cn(SURFACE, 'overflow-hidden')}>
      {rows.length === 0 ? empty : (
        <>
          <table aria-label={label} className="hidden w-full text-sm md:table">
            <thead>
              <tr className="border-b border-border text-left text-[12px] font-bold uppercase tracking-wide text-faint">
                {columns.map((c) => <th key={c.key} className={cn('px-4 py-2.5 font-bold', c.num && 'text-right', c.className)}>{c.header}</th>)}
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={rowKey(r)} className={cn('relative border-b border-border last:border-0', onRow && 'cursor-pointer hover:bg-muted focus-within:bg-muted')}>
                  {columns.map((c, i) => (
                    <td key={c.key} className={cn('px-4 py-2.5', c.num && 'text-right tabular-nums', c.interactive && 'relative z-10', c.className)}>
                      {i === 0 && onRow ? (
                        // Stretched over the row: the whole row clicks, and it's one Tab stop with Enter.
                        <button type="button" onClick={() => onRow(r)} className="text-left outline-none after:absolute after:inset-0 focus-visible:underline">{c.cell(r)}</button>
                      ) : c.cell(r)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
          <ul aria-label={label} className="divide-y divide-border md:hidden">
            {rows.map((r) => (
              <li key={rowKey(r)}>
                <button type="button" disabled={!onRow} onClick={onRow && (() => onRow(r))}
                  className="flex w-full flex-col gap-1 px-4 pt-3 pb-3 text-left enabled:active:bg-muted">
                  <span className="font-semibold">{first.cell(r)}</span>
                  {rest.filter((c) => !c.wide && !c.interactive).map((c) => (
                    <span key={c.key} className="flex justify-between gap-3 text-[13px]">
                      <span className="text-faint">{c.header}</span>
                      <span className={cn('text-right', c.num && 'tabular-nums')}>{c.cell(r)}</span>
                    </span>
                  ))}
                </button>
                {/* Action columns (Run, Adjust, Looks fine…) stay reachable on a phone. */}
                {rest.some((c) => c.interactive) && (
                  <div className="flex justify-end px-3 pb-2.5">{rest.filter((c) => c.interactive).map((c) => <span key={c.key}>{c.cell(r)}</span>)}</div>
                )}
              </li>
            ))}
          </ul>
        </>
      )}
      {footer}
    </div>
  )
}
