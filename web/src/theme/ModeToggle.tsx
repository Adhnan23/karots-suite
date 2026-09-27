import { Monitor, Moon, Sun } from 'lucide-react'
import { useState } from 'react'
import { getMode, setMode, type Mode } from './mode'

const OPTIONS: { mode: Mode; label: string; Icon: typeof Sun }[] = [
  { mode: 'light', label: 'Light', Icon: Sun },
  { mode: 'system', label: 'System', Icon: Monitor },
  { mode: 'dark', label: 'Dark', Icon: Moon },
]

/** Three-way sun/moon/monitor control for the per-device colour mode. */
export function ModeToggle() {
  const [mode, setLocalMode] = useState<Mode>(getMode)

  return (
    <div role="group" aria-label="Colour mode" className="inline-flex items-center gap-1 rounded-full bg-muted p-1">
      {OPTIONS.map(({ mode: m, label, Icon }) => (
        <button
          key={m}
          type="button"
          aria-label={label}
          aria-pressed={mode === m}
          onClick={() => {
            setMode(m)
            setLocalMode(m)
          }}
          className="flex h-7 w-7 items-center justify-center rounded-full text-muted-foreground transition-colors aria-pressed:bg-card aria-pressed:text-foreground aria-pressed:shadow-sm"
        >
          <Icon size={15} />
        </button>
      ))}
    </div>
  )
}
