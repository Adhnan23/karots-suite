import { Delete } from 'lucide-react'
import { useRef, useState } from 'react'
import { NoteTray } from '@/features/till/pay/NoteTray'
import { num } from '@/lib/money'
import { cn } from '@/lib/utils'

const BTN = 'rounded-[var(--radius-sm)] bg-white/10 font-bold active:scale-95 hover:bg-white/15'

/** Shop game, cash: the customer's money on top, the shop's own notes and coins
 * (the till's note tray) to pick the change from. */
export function CashDrawer({ given, notes, coins, onGive }: { given: number; notes: number[]; coins: number[]; onGive: (v: number) => void }) {
  const [change, setChange] = useState('')
  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-hero-muted">They hand you <b className="font-display text-lg text-hero-foreground">Rs {given}</b> — pick the change from the drawer:</p>
      <div className="rounded-[var(--radius-sm)] bg-background p-2.5 text-foreground">
        <NoteTray notes={notes} coins={coins} amount={change} onChange={setChange} switcher={null} />
      </div>
      <button type="button" onClick={() => onGive(num(change))}
        className="h-12 rounded-[var(--radius-sm)] bg-brand font-bold text-brand-foreground">
        Give change · Rs {num(change)}
      </button>
    </div>
  )
}

/** Shop game, card: drag the card into the reader, then key in the amount. */
export function CardPay({ onDone }: { onDone: (amount: number) => void }) {
  const [swiped, setSwiped] = useState(false)
  const [typed, setTyped] = useState('')
  const [drag, setDrag] = useState<{ x: number; y: number } | null>(null)
  const reader = useRef<HTMLDivElement>(null)
  const from = useRef({ x: 0, y: 0 })

  if (!swiped) {
    return (
      <div className="flex flex-col items-center gap-4 py-2" onPointerMove={(e) => drag && setDrag({ x: e.clientX - from.current.x, y: e.clientY - from.current.y })}
        onPointerUp={(e) => {
          const r = reader.current?.getBoundingClientRect()
          if (drag && r && e.clientX >= r.left && e.clientX <= r.right && e.clientY >= r.top && e.clientY <= r.bottom) setSwiped(true)
          setDrag(null)
        }}>
        <div ref={reader} className={cn('grid h-20 w-full place-items-center rounded-[var(--radius-sm)] border-2 border-dashed text-sm font-semibold',
          drag ? 'border-brand text-hero-foreground' : 'border-white/25 text-hero-muted')}>
          Card reader — drop the card here
        </div>
        <div aria-label="The customer's card" onPointerDown={(e) => { from.current = { x: e.clientX, y: e.clientY }; setDrag({ x: 0, y: 0 }); (e.target as HTMLElement).setPointerCapture?.(e.pointerId) }}
          style={{ transform: drag ? `translate(${drag.x}px, ${drag.y}px) rotate(-4deg)` : undefined }}
          className={cn('relative h-24 w-40 cursor-grab touch-none rounded-xl bg-brand p-3 text-brand-foreground shadow-[var(--shadow)]', drag && 'cursor-grabbing transition-none', !drag && 'transition-transform')}>
          <span className="block h-5 w-7 rounded bg-white/50" />
          <span className="absolute bottom-3 left-3 font-mono text-[11px] font-bold tracking-widest">•••• 4821</span>
        </div>
        <p className="text-xs text-hero-muted">Drag the card up into the reader</p>
      </div>
    )
  }
  const press = (k: string) => setTyped((t) => (k === 'back' ? t.slice(0, -1) : (t + k).replace(/^0+/, '').slice(0, 6)))
  return (
    <div className="flex flex-col gap-2.5">
      <p className="text-sm text-hero-muted">Card in ✓ — key in the amount to charge:</p>
      <div className="rounded-[var(--radius-sm)] bg-white/10 px-4 py-2 text-right font-display text-2xl font-bold tabular-nums">Rs {typed || '0'}</div>
      <div className="grid grid-cols-3 gap-1.5">
        {['1', '2', '3', '4', '5', '6', '7', '8', '9', 'back', '0', '00'].map((k) => (
          <button key={k} type="button" aria-label={k === 'back' ? 'Delete' : k} onClick={() => press(k)} className={cn(BTN, 'grid h-11 place-items-center text-lg')}>
            {k === 'back' ? <Delete size={18} /> : k}
          </button>
        ))}
      </div>
      <button type="button" onClick={() => onDone(Number(typed || 0))} className="h-12 rounded-[var(--radius-sm)] bg-brand font-bold text-brand-foreground">OK · charge the card</button>
    </div>
  )
}
