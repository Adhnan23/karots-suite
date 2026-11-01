import { Delete } from 'lucide-react'
import { cn } from '@/lib/utils'

const KEYS = ['7', '8', '9', '4', '5', '6', '1', '2', '3', '.', '0', 'back'] as const

/** A thumb keypad that edits one value. The PC's TouchPad types into whatever box
 * has focus (a keyboard till); a phone sheet has no focused box, so this one is
 * driven by props. The reducer rounds a whole-only unit, as on the PC. */
export function Keypad({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  function press(k: (typeof KEYS)[number]) {
    if (k === 'back') return onChange(value.slice(0, -1))
    if (k === '.' && value.includes('.')) return
    onChange(value === '0' && k !== '.' ? k : value + k)
  }
  return (
    <div role="group" aria-label="Keypad" className="grid grid-cols-3 gap-1.5">
      {KEYS.map((k) => (
        <button key={k} type="button" onClick={() => press(k)} aria-label={k === 'back' ? 'Delete' : k}
          className={cn('h-12 rounded-[var(--radius-sm)] border border-border font-display text-lg font-bold tabular-nums active:scale-95',
            k === 'back' ? 'bg-muted text-muted-foreground' : 'bg-card shadow-[var(--shadow)]')}>
          {k === 'back' ? <Delete size={18} className="mx-auto" /> : k}
        </button>
      ))}
      <button type="button" onClick={() => onChange('')} aria-label="Clear"
        className="col-span-3 h-9 rounded-[var(--radius-sm)] bg-muted text-sm font-semibold text-muted-foreground">Clear</button>
    </div>
  )
}
