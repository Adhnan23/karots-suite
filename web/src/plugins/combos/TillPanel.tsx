import { useQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { tx } from '@/theme/motion'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { configQuery } from '@/features/till/api'
import { TILE } from '@/features/till/ui'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import type { TillApi } from '@/plugins/slots'
import { tillCombosQuery } from './api'
import { setPrice } from './pricing'
import { addCombo } from './till'
import type { Combo } from './types'

/** Till → 🎁 Combos: today's combos as tiles; a tile opens what's in it and Add. */
export function TillPanel({ till }: { till: TillApi; onDone: () => void }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const list = useQuery(tillCombosQuery).data
  const [picked, setPicked] = useState<number | null>(null)
  const [busy, setBusy] = useState(false)
  const c = list?.find((x) => x.id === picked) ?? null
  const add = async (x: Combo) => {
    setBusy(true)
    const err = await addCombo(till, x, 1)
    setBusy(false)
    if (err) toast.error(err)
    else toast.success(`${x.name} added`)
  }
  if (list && list.length === 0) return <p className="p-8 text-center text-faint">No combos on today. Add them in Apps → Combos.</p>
  return (
    <div className="flex flex-col gap-4">
      <div className="grid grid-cols-[repeat(auto-fill,minmax(176px,1fr))] gap-2.5">
        {list?.map((x) => {
          const { full, price } = setPrice(x, till.saleType)
          return (
            <motion.button key={x.id} type="button" onClick={() => setPicked(x.id)} whileHover={{ y: -2 }} whileTap={{ scale: 0.97 }} transition={tx('fast')}
              aria-pressed={picked === x.id} className={cn(TILE, 'aria-pressed:border-selected aria-pressed:ring-1 aria-pressed:ring-selected', x.problem && 'opacity-60')}>
              <span className="text-[13.5px] font-semibold leading-tight">{x.name}</span>
              {x.problem && <span className="text-[11.5px] font-bold text-danger">{x.problem}</span>}
              <span className="flex items-baseline gap-1.5 tabular-nums">
                <b className="text-[15px]">{money(sym, price / 100)}</b>
                {full > price && <s className="text-[11.5px] text-faint">{money(sym, full / 100)}</s>}
              </span>
            </motion.button>
          )
        })}
      </div>
      {c && (() => {
        const { full, price } = setPrice(c, till.saleType)
        return (
          <section aria-label={c.name} className="flex max-w-md flex-col gap-2.5 border-t border-border pt-3">
            <b className="font-display text-[16px]">{c.name}</b>
            <ul className="flex flex-col gap-0.5 text-[13.5px]">
              {c.members.map((m) => (
                <li key={m.productId} className="flex justify-between gap-3">
                  <span className={cn(!m.active && 'text-danger')}>{m.name} <span className="text-muted-foreground">× {Number(m.qty)}</span></span>
                </li>
              ))}
            </ul>
            {c.problem && <p className="text-[13px] font-semibold text-danger">{c.problem} — can't add</p>}
            {c.warnings.map((w) => <p key={w} className="text-[12.5px] font-semibold text-warning">{w}</p>)}
            <Button className="h-11 justify-between font-bold" disabled={!!c.problem || busy} onClick={() => void add(c)}>
              <span>Add combo</span>
              <span className="tabular-nums">{full > price && <s className="mr-2 font-normal opacity-70">{money(sym, full / 100)}</s>}{money(sym, price / 100)}</span>
            </Button>
          </section>
        )
      })()}
    </div>
  )
}
