import { Search } from 'lucide-react'
import { Popover } from 'radix-ui'
import { useState } from 'react'
import { EMOJI, searchEmoji } from '@/lib/emoji'
import { cn } from '@/lib/utils'
import { INPUT } from './fields'

/** Pick an emoji without an emoji keyboard (a till PC has none): a searchable
 * grid by aisle. Typing a real emoji in the search box also works. '' = none. */
export function EmojiPicker({ value, onChange, label, className }: { value: string; onChange: (e: string) => void; label: string; className?: string }) {
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const hits = q.trim() ? searchEmoji(q) : null
  const pick = (e: string) => { onChange(e); setOpen(false); setQ('') }
  const cell = (e: string) => (
    <button key={e} type="button" aria-label={e} aria-pressed={value === e} onClick={() => pick(e)}
      className="grid h-9 w-9 place-items-center rounded-[10px] text-xl hover:bg-muted aria-pressed:bg-selected">{e}</button>
  )
  return (
    <Popover.Root open={open} onOpenChange={(o) => { setOpen(o); if (!o) setQ('') }}>
      <Popover.Trigger asChild>
        <button type="button" aria-label={label} className={cn('grid place-items-center rounded-[var(--radius-sm)] border border-border bg-muted text-xl hover:border-selected', className ?? 'h-[var(--control-h)] w-[var(--control-h)]')}>
          {value || <span className="text-[13px] text-faint">+</span>}
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content align="start" sideOffset={6} collisionPadding={12} onOpenAutoFocus={(e) => { e.preventDefault(); (e.currentTarget as HTMLElement).querySelector('input')?.focus() }}
          className="z-50 flex w-[min(340px,calc(100vw-24px))] flex-col gap-2 rounded-[var(--radius)] border border-border bg-popover p-3 text-popover-foreground shadow-[var(--shadow-float)]">
          <div className="relative">
            <Search size={15} className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-faint" />
            <input aria-label="Search emoji" placeholder="Search: milk, soap, pen…" value={q} onChange={(e) => setQ(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); const first = hits?.[0] ?? (/\p{Extended_Pictographic}/u.test(q) ? q.trim() : ''); if (first) pick(first) } }}
              className={cn(INPUT, 'h-9 pl-8 text-sm')} />
          </div>
          <div className="max-h-64 overflow-auto">
            {hits ? (
              hits.length ? <div className="grid grid-cols-8 gap-0.5">{hits.map(cell)}</div>
                : <p className="p-4 text-center text-sm text-faint">No emoji for “{q}”.</p>
            ) : EMOJI.map(([aisle, items]) => (
              <section key={aisle} className="mb-1.5">
                <h4 className="px-1 pb-1 text-[11px] font-bold uppercase tracking-wide text-faint">{aisle}</h4>
                <div className="grid grid-cols-8 gap-0.5">{items.map(([e]) => cell(e))}</div>
              </section>
            ))}
          </div>
          {value && <button type="button" onClick={() => pick('')} className="self-start rounded-full px-2 py-1 text-[12px] font-bold text-muted-foreground hover:text-foreground">Remove emoji</button>}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
