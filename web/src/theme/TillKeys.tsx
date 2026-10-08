import { Keyboard, Pin, X } from 'lucide-react'
import { useState } from 'react'
import { DropdownMenu, DropdownMenuContent, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { Kbd } from '@/features/till/bits'

const PIN_KEY = 'ks-till-keys-pinned'
const KEYS: [string, string][] = [
  ['F1', 'Menu'], ['F2', 'Search'], ['F4', 'Hold'], ['F9', 'Discount'], ['F10', 'Pay'],
  ['Enter', 'Complete the sale'], ['Esc', 'Close a dialog'], ['Ctrl K', 'Jump to a tab'],
]
const readPinned = () => {
  try { return localStorage.getItem(PIN_KEY) === '1' } catch { return false }
}

/** The sell screen's keyboard shortcuts: one button with the list, or pinned
 * inline on the bar (remembered on this computer). */
export function TillKeys() {
  const [pinned, setPinned] = useState(readPinned)
  const pin = (on: boolean) => {
    setPinned(on)
    try { localStorage.setItem(PIN_KEY, on ? '1' : '0') } catch { /* private mode */ }
  }
  if (pinned) {
    return (
      <div className="flex shrink-0 items-center gap-2 whitespace-nowrap text-[12px] text-muted-foreground" aria-label="Keyboard shortcuts">
        {KEYS.slice(0, 5).map(([k, l]) => <span key={k} className="flex items-center gap-1"><Kbd>{k}</Kbd>{l}</span>)}
        <button type="button" onClick={() => pin(false)} aria-label="Fold the shortcuts away" title="Fold the shortcuts away"
          className="grid h-7 w-7 place-items-center rounded-full text-faint hover:bg-muted hover:text-foreground"><X size={14} /></button>
      </div>
    )
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button type="button" aria-label="Keyboard shortcuts" title="Keyboard shortcuts"
          className="flex h-9 shrink-0 items-center gap-1.5 rounded-[var(--radius-sm)] px-3 text-[13px] font-semibold text-muted-foreground hover:bg-muted hover:text-foreground">
          <Keyboard size={16} />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-64 p-3">
        <p className="mb-2 text-[12px] font-semibold text-muted-foreground">Keyboard shortcuts</p>
        <dl className="grid grid-cols-[auto_1fr] items-center gap-x-3 gap-y-1.5 text-sm">
          {KEYS.map(([k, l]) => <div key={k} className="contents"><dt><Kbd>{k}</Kbd></dt><dd>{l}</dd></div>)}
        </dl>
        <button type="button" onClick={() => pin(true)} className="mt-3 flex w-full items-center gap-2 rounded-[var(--radius-sm)] px-1 py-1.5 text-[13px] text-muted-foreground hover:text-foreground">
          <Pin size={14} /> Keep them on the bar
        </button>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
