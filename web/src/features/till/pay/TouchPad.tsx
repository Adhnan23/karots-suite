import { Delete } from 'lucide-react'
import { useEffect } from 'react'
import { cn } from '@/lib/utils'

/** Shows a touch-screen helper only where it helps: a touch screen (coarse
 * pointer) tall enough to fit it — never on a mouse-and-keyboard counter or a
 * short screen, where it would only push Complete sale away. */
export const TOUCH_ONLY = 'hidden [@media(any-pointer:coarse)_and_(min-height:760px)]:grid'

// The till box the keypad types into: the last one focused (search, amount, qty,
// discount…), the amount box until one is.
let target: HTMLInputElement | null = null
function onFocus(e: FocusEvent) {
  const el = e.target
  if (el instanceof HTMLInputElement && !el.readOnly && !el.disabled && ['text', 'search', 'tel', 'number', ''].includes(el.type)) target = el
}

/** Write as a typed key would: React's onChange sees it (native setter + input event). */
function typeInto(el: HTMLInputElement, next: string) {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')!.set!.call(el, next)
  el.dispatchEvent(new Event('input', { bubbles: true }))
}

const KEYS = ['7', '8', '9', 'back', '4', '5', '6', 'clear', '1', '2', '3', 'enter', '00', '0', '.'] as const
type Key = (typeof KEYS)[number]

/** A number keypad for touch-screen tills that types into the focused box; Enter is
 * the box's own Enter (adds the payment in the amount box, searches in the scan box). */
export function TouchPad() {
  useEffect(() => {
    document.addEventListener('focusin', onFocus)
    return () => document.removeEventListener('focusin', onFocus)
  }, [])
  function press(k: Key) {
    const el = target && document.contains(target) ? target : (document.getElementById('till-amount') as HTMLInputElement | null)
    if (!el) return
    el.focus()
    if (k === 'enter') return void el.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }))
    const v = el.value
    if (k === 'clear') return typeInto(el, '')
    if (k === 'back') return typeInto(el, v.slice(0, -1))
    if (k === '.' && v.includes('.')) return
    typeInto(el, v + k)
  }
  return (
    // pointerdown default would move focus to the key; keep it in the box.
    <div role="group" aria-label="Keypad" onPointerDown={(e) => e.preventDefault()} onMouseDown={(e) => e.preventDefault()}
      className={cn(TOUCH_ONLY, 'grid-cols-4 gap-1.5')}>
      {KEYS.map((k) => (
        <button key={k} type="button" onClick={() => press(k)}
          aria-label={k === 'back' ? 'Delete' : k === 'clear' ? 'Clear' : k === 'enter' ? 'Enter' : k}
          className={cn('h-12 rounded-[var(--radius-sm)] border border-border font-display text-lg font-bold tabular-nums active:scale-95',
            k === 'enter' ? 'row-span-2 h-auto bg-brand text-base text-brand-foreground' : k === 'back' || k === 'clear' ? 'bg-muted text-sm text-muted-foreground' : 'bg-card shadow-[var(--shadow)]')}>
          {k === 'back' ? <Delete size={18} className="mx-auto" /> : k === 'clear' ? 'Clear' : k === 'enter' ? 'Enter' : k}
        </button>
      ))}
    </div>
  )
}
