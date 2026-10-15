import { motion } from 'motion/react'
import { type ReactNode, type RefObject, useState } from 'react'
import type { ShopHeader } from '@/api/types.gen'
import { Paper } from '@/features/counter/receipts/Paper'
import { cn } from '@/lib/utils'

export type ReceiptLine = { name: string; qty: number; price: number }

const Row = ({ l, r, b }: { l: ReactNode; r: ReactNode; b?: boolean }) => (
  <div className={cn('flex justify-between gap-3', b && 'font-bold')}><span>{l}</span><span className="tabular-nums">{r}</span></div>
)
const Rule = () => <hr className="my-2 border-dashed border-ink" />

/** Shop game: the receipt feeds out of the printer (the shop's real receipt look),
 * then it's dragged onto the customer to finish the sale. */
export function ReceiptOut({ shop, no, lines, total, card, given, target, onDragging, onDelivered }: {
  shop: ShopHeader; no: number; lines: ReceiptLine[]; total: number; card: boolean; given: number
  target: RefObject<HTMLElement | null>; onDragging: (on: boolean) => void; onDelivered: () => void
}) {
  const [printed, setPrinted] = useState(false)
  const [drag, setDrag] = useState<{ x: number; y: number } | null>(null)
  const [from, setFrom] = useState({ x: 0, y: 0 })
  const now = new Date().toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' })

  return (
    <div className="flex flex-col items-center gap-2">
      <p className="text-sm text-hero-muted">{printed ? 'Hand the receipt over — drag it to the customer' : 'Printing…'}</p>
      {/* The printer's mouth; the paper grows out of it. */}
      <div className="h-3 w-[240px] rounded-full bg-black/40 shadow-[inset_0_2px_4px_rgba(0,0,0,0.6)]" />
      <motion.div initial={{ height: 0 }} animate={{ height: 'auto' }} transition={{ duration: 1.4, ease: 'linear' }}
        onAnimationComplete={() => setPrinted(true)}
        className={cn('-mt-2 w-[220px] touch-none', printed ? 'cursor-grab' : 'overflow-hidden', drag && 'relative z-10 cursor-grabbing')}
        style={{ transform: drag ? `translate(${drag.x}px, ${drag.y}px) rotate(-3deg)` : undefined }}
        onPointerDown={(e) => {
          if (!printed) return
          setFrom({ x: e.clientX, y: e.clientY }); setDrag({ x: 0, y: 0 }); onDragging(true)
          ;(e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId)
        }}
        onPointerMove={(e) => drag && setDrag({ x: e.clientX - from.x, y: e.clientY - from.y })}
        onPointerUp={(e) => {
          if (!drag) return
          const r = target.current?.getBoundingClientRect()
          setDrag(null); onDragging(false)
          if (r && e.clientX >= r.left && e.clientX <= r.right && e.clientY >= r.top && e.clientY <= r.bottom) onDelivered()
        }}>
        <Paper shop={shop} narrow className="pointer-events-none select-none">
          <Row l={`#${String(no).padStart(5, '0')}`} r={now} />
          <Rule />
          {lines.map((l) => (
            <div key={l.name} className="mb-1">
              <span className="block">{l.name}</span>
              <Row l={`  ${l.qty} × ${l.price.toFixed(2)}`} r={(l.qty * l.price).toFixed(2)} />
            </div>
          ))}
          <Rule />
          <Row l="TOTAL" r={`Rs ${total.toFixed(2)}`} b />
          {card ? <Row l="Card" r={total.toFixed(2)} /> : (
            <>
              <Row l="Cash" r={given.toFixed(2)} />
              <Row l="Change" r={(given - total).toFixed(2)} />
            </>
          )}
        </Paper>
      </motion.div>
    </div>
  )
}
