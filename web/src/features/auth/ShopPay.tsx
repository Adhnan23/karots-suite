import { Delete } from 'lucide-react'
import { motion } from 'motion/react'
import { type ReactNode, type RefObject, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { NoteTray } from '@/features/till/pay/NoteTray'
import { num } from '@/lib/money'
import { cn } from '@/lib/utils'

// A device body in the theme's colours (light or dark, either palette).
const BODY = 'rounded-xl border border-border bg-muted p-3 shadow-[var(--shadow)] transition'
const KEY = 'grid h-11 place-items-center rounded-lg text-lg font-bold shadow-[inset_0_-3px_0_color-mix(in_srgb,var(--ink)_35%,transparent)] active:translate-y-px active:shadow-none disabled:opacity-40'

/** Drag something from the customer's side onto a machine; true when it's dropped on it. */
function useDragOnto(target: RefObject<HTMLElement | null>, onDrop: () => void) {
  const [drag, setDrag] = useState<{ x: number; y: number } | null>(null)
  const from = useRef({ x: 0, y: 0 })
  return {
    drag,
    item: {
      onPointerDown: (e: React.PointerEvent<HTMLElement>) => { from.current = { x: e.clientX, y: e.clientY }; setDrag({ x: 0, y: 0 }); e.currentTarget.setPointerCapture?.(e.pointerId) },
      style: { transform: drag ? `translate(${drag.x}px, ${drag.y}px) rotate(-4deg)` : undefined },
    },
    area: {
      onPointerMove: (e: React.PointerEvent) => drag && setDrag({ x: e.clientX - from.current.x, y: e.clientY - from.current.y }),
      onPointerUp: (e: React.PointerEvent) => {
        const r = target.current?.getBoundingClientRect()
        if (drag && r && e.clientX >= r.left && e.clientX <= r.right && e.clientY >= r.top && e.clientY <= r.bottom) onDrop()
        setDrag(null)
      },
    },
  }
}

/** What the customer holds out, shown on their side until it's dragged over. */
function InHand({ hand, children }: { hand: RefObject<HTMLElement | null>; children: ReactNode }) {
  return hand.current ? createPortal(children, hand.current) : null
}

/** Shop game, cash: the customer's money waits on their side; drag it into the cash
 * drawer, which slides open on the shop's own notes and coins (the till's tray) to
 * pick the change from. The change is yours to work out — nothing adds it up. */
export function CashDrawer({ hand, given, notes, coins, onGive }: {
  hand: RefObject<HTMLElement | null>; given: number; notes: number[]; coins: number[]; onGive: (v: number) => void
}) {
  const [open, setOpen] = useState(false)
  const [change, setChange] = useState('')
  const drawer = useRef<HTMLDivElement>(null)
  const d = useDragOnto(drawer, () => setOpen(true))
  return (
    <div className="flex flex-col items-center gap-2" {...d.area}>
      <div ref={drawer} aria-label="Cash drawer" className={cn(BODY, 'w-full', d.drag && 'ring-2 ring-brand')}>
        <div className="mb-2 flex items-center justify-between px-1 text-[11px] font-bold tracking-widest text-muted-foreground uppercase">
          <span>Cash drawer</span>
          {open && <span className="text-foreground normal-case tracking-normal">In: Rs {given}</span>}
        </div>
        {open ? (
          <motion.div initial={{ height: 0, opacity: 0.4 }} animate={{ height: 'auto', opacity: 1 }} transition={{ duration: 0.35 /* not a UI transition: the game */ }} className="overflow-hidden">
            <div className="rounded-lg bg-card p-2.5 text-foreground shadow-[inset_0_3px_8px_color-mix(in_srgb,var(--ink)_30%,transparent)]">
              <NoteTray notes={notes} coins={coins} amount={change} onChange={setChange} switcher={null} />
            </div>
          </motion.div>
        ) : (
          // The closed drawer front: a handle and a slot for the money.
          <div className="grid h-24 place-items-center rounded-lg border border-border bg-card text-sm font-semibold text-muted-foreground">
            <span className="h-2 w-24 rounded-full bg-[color-mix(in_srgb,var(--ink)_70%,transparent)]" />
            Put the customer's money in
          </div>
        )}
      </div>
      {open ? (
        <button type="button" onClick={() => onGive(num(change))} className="h-12 w-full rounded-[var(--radius-sm)] bg-brand font-bold text-brand-foreground">
          Give change
        </button>
      ) : <p className="text-xs text-muted-foreground">Drag the customer's money into the drawer</p>}
      {!open && (
        <InHand hand={hand}>
          <div aria-label="The customer's money" {...d.item}
            className={cn('relative z-20 mx-auto flex h-20 w-44 cursor-grab touch-none items-center justify-center rounded-md border-[1.5px] border-[color-mix(in_srgb,var(--chart-2)_60%,transparent)] bg-[color-mix(in_srgb,var(--chart-2)_22%,var(--paper))] text-ink shadow-[var(--shadow)]',
              d.drag ? 'cursor-grabbing' : 'transition-transform')}>
            <span className="pointer-events-none absolute inset-1.5 rounded-sm border border-dashed border-[color-mix(in_srgb,var(--chart-2)_60%,transparent)]" />
            <span className="absolute top-1.5 left-2.5 text-[10px] font-bold opacity-60">Rs</span>
            <b className="font-display text-2xl tabular-nums">{given.toLocaleString()}</b>
          </div>
        </InHand>
      )}
    </div>
  )
}

/** Shop game, card: the customer's card waits on their side; drag it into the card
 * machine's slot, then key the amount on the machine and press OK. */
export function CardPay({ hand, onDone }: { hand: RefObject<HTMLElement | null>; onDone: (amount: number) => void }) {
  const [inserted, setInserted] = useState(false)
  const [typed, setTyped] = useState('')
  const machine = useRef<HTMLDivElement>(null)
  const d = useDragOnto(machine, () => setInserted(true))
  const press = (k: string) => setTyped((t) => (k === 'back' ? t.slice(0, -1) : k === 'clear' ? '' : (t + k).replace(/^0+/, '').slice(0, 6)))

  return (
    <div className="flex flex-col items-center gap-2" {...d.area}>
      <div ref={machine} aria-label="Card machine" className={cn(BODY, 'w-[230px]', d.drag && 'ring-2 ring-brand')}>
        <div className={cn('mx-auto mb-3 h-2 w-28 rounded-full bg-ink', d.drag && 'shadow-[0_0_10px_var(--brand)]')} title="Card slot" />
        <div className="mb-3 rounded-lg bg-paper px-3 py-2.5 font-mono text-ink shadow-[inset_0_2px_6px_color-mix(in_srgb,var(--ink)_35%,transparent)]">
          <div className="text-[11px] font-bold tracking-wide uppercase">{inserted ? 'Sale — enter amount' : 'Insert card'}</div>
          <div className="text-right text-2xl font-bold tabular-nums">{inserted ? `Rs ${typed || '0'}` : '— — —'}</div>
        </div>
        <div className="grid grid-cols-3 gap-1.5 text-foreground">
          {['1', '2', '3', '4', '5', '6', '7', '8', '9', '00', '0', 'back'].map((k) => (
            <button key={k} type="button" disabled={!inserted} aria-label={k === 'back' ? 'Delete' : k} onClick={() => press(k)} className={cn(KEY, 'border border-border bg-card')}>
              {k === 'back' ? <Delete size={18} /> : k}
            </button>
          ))}
          <button type="button" disabled={!inserted} aria-label="Cancel" onClick={() => press('clear')} className={cn(KEY, 'bg-destructive text-sm text-[var(--paper)]')}>✕</button>
          <button type="button" disabled={!inserted} aria-label="Back" onClick={() => press('back')} className={cn(KEY, 'bg-warning text-sm text-ink')}>←</button>
          <button type="button" disabled={!inserted} aria-label="OK" onClick={() => onDone(Number(typed || 0))} className={cn(KEY, 'bg-success text-sm text-[var(--paper)]')}>OK</button>
        </div>
      </div>
      <p className="text-xs text-muted-foreground">{inserted ? 'Key in the amount, then OK' : "Drag the customer's card into the machine"}</p>
      {!inserted && (
        <InHand hand={hand}>
          <div aria-label="The customer's card" {...d.item}
            className={cn('relative z-20 mx-auto h-24 w-40 cursor-grab touch-none rounded-xl bg-brand p-3 text-brand-foreground shadow-[var(--shadow)]', d.drag ? 'cursor-grabbing' : 'transition-transform')}>
            <span className="block h-5 w-7 rounded bg-[color-mix(in_srgb,var(--paper)_55%,transparent)]" />
            <span className="absolute bottom-3 left-3 font-mono text-[11px] font-bold tracking-widest">•••• 4821</span>
          </div>
        </InHand>
      )}
    </div>
  )
}
