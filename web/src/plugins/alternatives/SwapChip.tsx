import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { configQuery } from '@/features/till/api'
import type { CartLine } from '@/features/till/cart/types'
import { money, num, qtyText } from '@/lib/money'
import type { TillApi } from '../slots'
import { tillAltsQuery } from './api'
import { AskQty } from './AskQty'
import { isShort } from './labels'
import type { Alt } from './types'

/** In-stock alternatives for a short line, best tier first; tap one to put it on
 * the line instead (same qty; one sold by another unit shows it and asks how many). Under the cart line (gone once found-at-till was
 * confirmed) and inside the found-at-till prompt (inPrompt: swapping closes it). */
export function SwapChip({ till, line, onDone }: { till: TillApi; line: CartLine; onDone?: () => void }) {
  const inPrompt = !!onDone
  const short = isShort(line) && !line.allowOversell
  const alts = useQuery({ ...tillAltsQuery(line.productId), enabled: short }).data
  const sym = useQuery(configQuery).data?.symbol ?? ''
  // In the prompt the best swap takes the focus from "Sell N" (an effect runs
  // after the dialog's autoFocus), so Enter swaps instead of faking the count.
  const first = useRef<HTMLButtonElement>(null)
  const [ask, setAsk] = useState<Alt | null>(null)
  const hasAlts = !!alts?.length
  useEffect(() => { if (inPrompt && hasAlts) first.current?.focus() }, [inPrompt, hasAlts])
  if (!short || !alts?.length) return null
  const swap = async (id: number, name: string, qty?: number) => {
    const err = await till.swapLine(line.key, id, qty)
    if (err) return void toast.error(err)
    toast.success(`Swapped to ${name}`)
    onDone?.()
  }
  return (
    <div className={inPrompt ? 'flex flex-col items-stretch gap-1.5 text-[13px]' : 'mt-1 flex flex-wrap items-center gap-1 text-[11.5px]'}>
      <span className="font-semibold text-warning">{inPrompt ? 'In stock instead — swap, no count change:' : `${line.stock <= 0 ? 'Out of stock' : `Only ${qtyText(line.stock)} left`} — swap for`}</span>
      {alts.map((a, i) => {
        const price = till.saleType === 'wholesale' && num(a.wholesalePrice) > 0 ? a.wholesalePrice : a.sellingPrice
        return (
          <button key={a.productId} ref={i === 0 ? first : undefined} type="button" onClick={() => (a.sameUnit ? void swap(a.productId, a.name) : setAsk(a))}
            className={inPrompt ? 'rounded-[var(--radius-sm)] border border-brand bg-brand-soft px-3 py-2 text-left font-semibold hover:bg-brand hover:text-brand-foreground'
              : 'max-w-full rounded-md border border-dashed border-brand bg-brand-soft px-2 py-0.5 text-left font-semibold hover:border-solid'}>
            ⇄ {a.name}{a.tier && ` · ${a.tier}`} · {qtyText(a.stock)}{a.sameUnit ? '' : ` ${a.unit}`} left · {money(sym, price)}{!a.sameUnit && `/${a.unit}`}
          </button>
        )
      })}
      {ask && <AskQty alt={ask} onCancel={() => setAsk(null)} onSwap={(q) => { setAsk(null); void swap(ask.productId, ask.name, q) }} />}
    </div>
  )
}
