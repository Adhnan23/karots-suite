import { Minus, Plus, X } from 'lucide-react'
import { motion } from 'motion/react'
import { useState } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { money, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { lineDiscount, lineGross } from '../cart/totals'
import type { CartLine } from '../cart/types'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'

/** Lines a plugin added together: one header (sets − / +, saving, remove), then the
 * members without their own qty or discount; removing one asks to break the group. */
export function CartGroup({ id, label, count, lines, sym }: { id: number; label: string; count: number; lines: CartLine[]; sym: string }) {
  const { dispatch } = useTill()
  const [breaking, setBreaking] = useState<CartLine | null>(null)
  const save = lines.reduce((s, l) => s + lineDiscount(l), 0)
  const net = lines.reduce((s, l) => s + lineGross(l) - lineDiscount(l), 0)
  const setCount = (n: number) => dispatch({ type: 'groupCount', id, count: n })
  return (
    <motion.div initial={{ opacity: 0.4 }} animate={{ opacity: 1 }} transition={{ duration: 0.4 }}
      className="my-1 overflow-hidden rounded-[var(--radius-sm)] border-[1.5px] border-brand">
      <div className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-3 bg-brand-soft px-2 py-2">
        <div className="min-w-0">
          <div className="text-[14px] font-bold leading-snug">{label}</div>
          {save > 0 && <div className="text-[12px] font-bold text-success tabular-nums">save {money(sym, save)}</div>}
        </div>
        <div className="flex items-center gap-0.5 rounded-full border border-border bg-card p-0.5">
          <button type="button" aria-label={`Fewer ${label}`} onClick={() => setCount(count - 1)} className="grid h-7 w-7 place-items-center rounded-full hover:bg-border"><Minus size={14} /></button>
          <span aria-label={`Sets of ${label}`} className="w-8 text-center text-[14px] font-bold tabular-nums">{count}</span>
          <button type="button" aria-label={`More ${label}`} onClick={() => setCount(count + 1)} className="grid h-7 w-7 place-items-center rounded-full hover:bg-border"><Plus size={14} /></button>
        </div>
        <div className="flex items-center gap-1">
          <span className="min-w-[72px] text-right text-[14.5px] font-bold tabular-nums">{money(sym, net)}</span>
          <button type="button" aria-label={`Remove ${label}`} onClick={() => dispatch({ type: 'removeGroup', id })} className="grid size-8 place-items-center rounded-full text-faint hover:bg-muted hover:text-danger"><X size={17} /></button>
        </div>
      </div>
      {lines.map((l) => (
        <div key={l.key} className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-x-3 gap-y-1.5 border-t border-border py-1.5 pl-5 pr-2">
          <div className="min-w-0 text-[13.5px] font-semibold">{l.name} <span className="font-semibold text-muted-foreground tabular-nums">× {qtyText(l.qty)}</span></div>
          <div className="text-right text-[13.5px] tabular-nums">
            <s className="mr-1.5 text-[11.5px] text-faint">{money(sym, lineGross(l))}</s>{money(sym, lineGross(l) - lineDiscount(l))}
          </div>
          <button type="button" aria-label={`Remove ${l.name} from ${label}`} onClick={() => setBreaking(l)} className="grid size-8 place-items-center rounded-full text-faint hover:bg-muted hover:text-danger"><X size={17} /></button>
          {l.trackSerial && l.serials.length > 0 && (
            <div className="col-span-full grid grid-cols-2 gap-1.5">
              {l.serials.map((s, i) => (
                <input key={i} aria-label={`Serial ${i + 1} for ${l.name}`} placeholder={`Serial ${i + 1}`} autoComplete="off" value={s}
                  onChange={(e) => dispatch({ type: 'serial', key: l.key, index: i, value: e.target.value })} className={cn(FIELD, 'h-8 text-[13px]')} />
              ))}
            </div>
          )}
        </div>
      ))}
      <Dialog open={!!breaking} onOpenChange={(o) => !o && setBreaking(null)}>
        <DialogContent className="sm:max-w-[420px]">
          <DialogHeader>
            <DialogTitle>Break the {label}?</DialogTitle>
            <DialogDescription>{breaking?.name} comes out of the cart. The other items stay, back at full price.</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="ghost" autoFocus onClick={() => setBreaking(null)}>Keep it</Button>
            <Button variant="destructive" onClick={() => {
              if (breaking) dispatch({ type: 'breakGroup', id, key: breaking.key })
              setBreaking(null)
            }}>Break it</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </motion.div>
  )
}
