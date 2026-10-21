import { Check, ChevronDown, Minus, Pencil, Play, Plus, Printer, Trash2 } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { useState } from 'react'
import { INPUT, Toggle } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { CHIP, TILE } from '@/features/till/ui'
import { tx } from '@/theme/motion'

/** More preview surfaces for the palette editor, built from the real kit pieces so
 *  they look exactly like the screens: buttons in every state, form controls, a list,
 *  a menu and the till. */
const BOX = 'flex flex-col gap-3 rounded-[var(--radius)] border border-border bg-card p-4 text-card-foreground shadow-[var(--shadow)] [backdrop-filter:var(--glass)]'
const TITLE = 'text-xs font-bold tracking-wide text-faint uppercase'

export function ButtonsSample() {
  return (
    <div className={BOX} aria-label="Sample buttons">
      <p className={TITLE}>Buttons</p>
      <div className="flex flex-wrap gap-2">
        <Button className="h-9 px-3">Normal</Button>
        <Button className="h-9 border-ring px-3 ring-3 ring-ring/50">Focused</Button>
        <Button className="h-9 px-3" disabled>Disabled</Button>
        <Button variant="outline" className="h-9 px-3">Outline</Button>
        <Button variant="ghost" className="h-9 px-3">Ghost</Button>
        <Button variant="destructive" className="h-9 px-3">Delete</Button>
      </div>
    </div>
  )
}

export function FormSample() {
  const [on, setOn] = useState(true)
  const [tick, setTick] = useState(true)
  return (
    <div className={BOX} aria-label="Sample form">
      <p className={TITLE}>Form</p>
      <input aria-label="Sample filled input" defaultValue="Anchor milk 400 g" className={INPUT} />
      <input aria-label="Sample focused input" defaultValue="Focused" className={`${INPUT} ring-2 ring-ring`} />
      <div className="relative">
        <select aria-label="Sample select" className={`${INPUT} appearance-none pr-9`} defaultValue="b">
          <option value="a">Cash</option>
          <option value="b">Card</option>
        </select>
        <ChevronDown className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-muted-foreground" />
      </div>
      <Toggle label="Ask before printing" hint="A switch" checked={on} onChange={setOn} />
      <label className="flex items-center gap-2 text-sm font-semibold">
        <input type="checkbox" checked={tick} onChange={(e) => setTick(e.target.checked)} className="size-4 accent-(--brand)" /> A tick box
      </label>
    </div>
  )
}

export function ListSample() {
  const rows = [['Anchor milk', '24', 'Rs. 480'], ['Sunlight soap', '3', 'Rs. 120'], ['Munchee crackers', '0', 'Rs. 260']]
  return (
    <div className={`${BOX} gap-0 p-0`} aria-label="Sample list">
      <div className="grid grid-cols-[1fr_auto_auto] gap-3 border-b border-border px-4 py-2 text-xs font-bold text-muted-foreground">
        <span>Product</span><span>Stock</span><span>Price</span>
      </div>
      {rows.map(([name, qty, price], i) => (
        <div key={name} className={['grid grid-cols-[1fr_auto_auto] gap-3 border-b border-border px-4 py-2.5 text-sm last:border-b-0',
          i === 1 ? 'bg-muted' : '', i === 2 ? 'bg-brand-soft' : ''].join(' ')}>
          <span className="font-semibold">{name} <span className="text-xs font-normal text-faint">{i === 1 ? '· hover' : i === 2 ? '· selected' : ''}</span></span>
          <span className={qty === '0' ? 'font-bold text-danger' : 'tabular-nums'}>{qty}</span>
          <span className="tabular-nums">{price}</span>
        </div>
      ))}
    </div>
  )
}

export function MenuSample() {
  return (
    <div className="relative h-40 overflow-hidden rounded-[var(--radius)] border border-border bg-background p-3" aria-label="Sample menu">
      <span className="inline-flex h-8 items-center gap-1 rounded-pill border border-border bg-card px-3 text-xs font-semibold">Actions <ChevronDown className="size-3.5" /></span>
      <div className="absolute top-12 left-3 w-48 rounded-[var(--radius-sm)] border border-border bg-popover p-1 text-sm text-popover-foreground shadow-[var(--shadow-float)]">
        <p className="flex items-center gap-2 rounded-[var(--radius-sm)] px-2 py-1.5"><Pencil className="size-4" /> Edit</p>
        <p className="flex items-center gap-2 rounded-[var(--radius-sm)] bg-accent px-2 py-1.5 text-accent-foreground"><Printer className="size-4" /> Print · hover</p>
        <p className="flex items-center gap-2 rounded-[var(--radius-sm)] px-2 py-1.5 text-danger"><Trash2 className="size-4" /> Delete</p>
      </div>
    </div>
  )
}

export function TillSample() {
  return (
    <div className={BOX} aria-label="Sample till">
      <p className={TITLE}>Till</p>
      <div className="flex gap-1.5">
        <span className={CHIP} aria-pressed="true">Frequently sold</span>
        <span className={CHIP}>Dairy</span>
      </div>
      <div className="grid grid-cols-2 gap-2">
        {[['Anchor milk', 'Rs. 480.00'], ['Sunlight soap', 'Rs. 120.00']].map(([n, p]) => (
          <div key={n} className={`${TILE} min-h-0!`}>
            <span className="text-sm font-semibold">{n}</span>
            <span className="text-sm font-bold tabular-nums">{p}</span>
          </div>
        ))}
      </div>
      <div className="flex items-center gap-2 rounded-[var(--radius-sm)] bg-brand-soft px-2 py-2">
        <span className="min-w-0 flex-1 truncate text-sm font-semibold">Anchor milk</span>
        <span className="flex items-center gap-1 rounded-pill border border-border bg-card p-0.5">
          <span className="grid size-6 place-items-center rounded-full hover:bg-muted"><Minus className="size-3.5" /></span>
          <span className="w-5 text-center text-sm font-bold">2</span>
          <span className="grid size-6 place-items-center rounded-full hover:bg-muted"><Plus className="size-3.5" /></span>
        </span>
        <span className="text-sm font-bold tabular-nums">Rs. 960</span>
      </div>
      <button type="button" className="flex h-12 items-center justify-center gap-2 rounded-[var(--radius-sm)] bg-brand font-bold text-brand-foreground shadow-[var(--glow)]">
        <Check className="size-5" /> Pay Rs. 960.00
      </button>
    </div>
  )
}

/** Plays the palette's motion: a pop-in (spring), a sheet rising (base) and tiles that
 *  lift on hover (--lift). Timings are read when it plays, so edits show on the next Play. */
export function MotionSample() {
  const [run, setRun] = useState(0)
  return (
    <div className={BOX} aria-label="Sample motion">
      <div className="flex items-center justify-between">
        <p className={TITLE}>Motion</p>
        <Button className="h-8 gap-1.5 px-3" onClick={() => setRun((n) => n + 1)}><Play className="size-3.5" /> Play</Button>
      </div>
      <div className="relative h-28 overflow-hidden rounded-[var(--radius-sm)] bg-muted">
        <AnimatePresence mode="popLayout">
          <motion.div key={`pop${run}`} initial={{ scale: 0.7, opacity: 0 }} animate={{ scale: 1, opacity: 1 }} transition={tx('slow', 'spring')}
            className="absolute top-3 left-3 rounded-[var(--radius-sm)] bg-popover px-3 py-2 text-xs font-bold shadow-[var(--shadow-float)]">
            Sale complete
          </motion.div>
          <motion.div key={`sheet${run}`} initial={{ y: '100%' }} animate={{ y: 0 }} transition={tx('base')}
            className="absolute inset-x-0 bottom-0 h-12 rounded-t-[var(--radius)] border-t border-border bg-card px-3 py-2 text-xs font-semibold shadow-[var(--shadow-float)]">
            A sheet rising
          </motion.div>
        </AnimatePresence>
      </div>
      <div className="grid grid-cols-3 gap-2">
        {['Hover', 'over', 'me'].map((w) => (
          <span key={w} className={`${TILE} min-h-0! items-center p-2 text-xs font-semibold transition-transform duration-(--dur-fast) ease-(--ease) hover:-translate-y-(--lift)`}>{w}</span>
        ))}
      </div>
      <p className="text-xs text-faint">Tiles rise on hover only with Lively motion.</p>
    </div>
  )
}
