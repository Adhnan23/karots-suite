import { useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { paletteItems, type NavArea, type PaletteEntry } from './nav'

export type { PaletteEntry }

/** Ctrl/Cmd K command palette (theme.md §6): every ported page and "Open the
 * till" by default; the till passes its own tabs. Controlled from the shell,
 * which also owns the Ctrl K key listener. Enter jumps to the top match. */
export function CommandPalette({ open, onOpenChange, entries: given, areas = [] }: { open: boolean; onOpenChange: (open: boolean) => void; entries?: PaletteEntry[]; areas?: NavArea[] }) {
  const [q, setQ] = useState('')
  const navigate = useNavigate()

  const entries: PaletteEntry[] = given ?? paletteItems(areas)
  const filtered = q ? entries.filter((e) => e.label.toLowerCase().includes(q.toLowerCase())) : entries

  const go = (to: string) => {
    onOpenChange(false)
    setQ('')
    navigate({ to })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-md gap-0 p-0" showCloseButton={false}>
        <DialogTitle className="sr-only">Search pages</DialogTitle>
        <div className="flex items-center gap-2 border-b border-border px-3 py-2.5">
          <input
            autoFocus
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && filtered[0] && go(filtered[0].to)}
            placeholder="Search pages, products, bills"
            className="flex-1 bg-transparent text-sm outline-none"
          />
          <kbd className="rounded border border-border bg-muted px-1.5 py-0.5 text-[11px] font-semibold text-muted-foreground">Esc</kbd>
        </div>
        <div className="max-h-80 overflow-auto p-1.5">
          {filtered.length === 0 && <p className="p-4 text-center text-sm text-muted-foreground">No matches.</p>}
          {filtered.map((e) => (
            <button
              key={e.key}
              type="button"
              onClick={() => go(e.to)}
              className="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-left text-sm hover:bg-muted"
            >
              <e.icon size={16} className="text-muted-foreground" />
              {e.label}
            </button>
          ))}
        </div>
      </DialogContent>
    </Dialog>
  )
}
