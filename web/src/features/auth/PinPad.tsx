import { Delete } from 'lucide-react'

const KEYS = ['1', '2', '3', '4', '5', '6', '7', '8', '9', 'clear', '0', 'back'] as const
export type PinKey = (typeof KEYS)[number]

/** Numeric keypad that writes into whichever field last had focus (theme.md §5).
 *  mousedown preventDefault keeps focus in that field instead of moving to the button. */
export function PinPad({ onKey }: { onKey: (key: PinKey) => void }) {
  return (
    <div
      className="grid max-w-[360px] grid-cols-3 gap-2.5"
      onMouseDown={(e) => e.preventDefault()}
    >
      {KEYS.map((k) =>
        k === 'clear' ? (
          <button
            key={k}
            type="button"
            onClick={() => onKey(k)}
            className="h-[58px] rounded-[var(--radius-sm)] border border-border bg-muted text-sm font-bold text-muted-foreground"
          >
            Clear
          </button>
        ) : k === 'back' ? (
          <button
            key={k}
            type="button"
            aria-label="Delete"
            onClick={() => onKey(k)}
            className="flex h-[58px] items-center justify-center rounded-[var(--radius-sm)] border border-border bg-muted text-muted-foreground"
          >
            <Delete size={20} />
          </button>
        ) : (
          <button
            key={k}
            type="button"
            onClick={() => onKey(k)}
            className="h-[58px] rounded-[var(--radius-sm)] border border-border bg-card font-display text-xl font-bold shadow-[var(--shadow)] active:scale-95"
          >
            {k}
          </button>
        ),
      )}
    </div>
  )
}
