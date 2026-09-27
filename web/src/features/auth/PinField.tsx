import { cn } from 'cn'
import { useState, type Ref } from 'react'

type Props = {
  id: string
  value: string
  onChange: (v: string) => void
  onFocus?: () => void
  onEnter?: () => void
  shake?: boolean
  inputRef?: Ref<HTMLInputElement>
}

function isCoarsePointer() {
  try {
    return window.matchMedia('(pointer: coarse)').matches
  } catch {
    return false
  }
}

/** The PIN box: dots for display, a real (invisible) input underneath so a
 * physical keyboard types into it and Enter submits. No browser memory: it is
 * `type="text"` with `-webkit-text-security: disc`, never `type="password"`,
 * so a password manager never offers to save a shared counter's PIN. */
export function PinField({ id, value, onChange, onFocus, onEnter, shake, inputRef }: Props) {
  const [inputMode] = useState<'numeric' | 'none'>(() => (isCoarsePointer() ? 'none' : 'numeric'))

  return (
    <div
      className={cn(
        'relative flex h-[54px] items-center gap-3 rounded-[var(--radius-sm)] border-[1.5px] border-border bg-card px-4 focus-within:border-brand',
        shake && 'animate-[shake_0.35s_ease]',
      )}
    >
      <input
        ref={inputRef}
        id={id}
        name="ks-pn"
        type="text"
        inputMode={inputMode}
        autoComplete="off"
        autoCorrect="off"
        spellCheck={false}
        maxLength={6}
        value={value}
        onChange={(e) => onChange(e.target.value.replace(/\D/g, '').slice(0, 6))}
        onFocus={onFocus}
        onKeyDown={(e) => {
          if (e.key === 'Enter') {
            e.preventDefault()
            onEnter?.()
          }
        }}
        className="absolute inset-0 h-full w-full cursor-text border-0 bg-transparent opacity-0 outline-none"
        style={{ WebkitTextSecurity: 'disc' } as React.CSSProperties}
      />
      {Array.from({ length: Math.max(4, value.length) }).map((_, i) => (
        <i
          key={i}
          className={cn(
            'pointer-events-none h-4 w-4 rounded-full border-2 border-faint transition-transform',
            i < value.length && 'scale-110 border-foreground bg-foreground',
          )}
        />
      ))}
      <span className="pointer-events-none ml-auto text-xs text-faint">Type or tap</span>
    </div>
  )
}
