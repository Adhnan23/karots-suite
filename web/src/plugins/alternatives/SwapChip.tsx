import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { configQuery } from '@/features/till/api'
import type { CartLine } from '@/features/till/cart/types'
import { money, num, qtyText } from '@/lib/money'
import type { TillApi } from '../slots'
import { tillAltsQuery } from './api'
import { isShort } from './labels'

/** In-stock alternatives for a short line, best tier first; tap one to put it on
 * the line instead (same qty). Under the cart line (gone once found-at-till was
 * confirmed) and inside the found-at-till prompt (inPrompt: swapping closes it). */
export function SwapChip({ till, line, onDone }: { till: TillApi; line: CartLine; onDone?: () => void }) {
  const inPrompt = !!onDone
  const short = isShort(line) && !line.allowOversell
  const alts = useQuery({ ...tillAltsQuery(line.productId), enabled: short }).data
  const sym = useQuery(configQuery).data?.symbol ?? ''
  if (!short || !alts?.length) return null
  const swap = async (id: number, name: string) => {
    const err = await till.swapLine(line.key, id)
    if (err) return void toast.error(err)
    toast.success(`Swapped to ${name}`)
    onDone?.()
  }
  return (
    <div className={inPrompt ? 'flex flex-col items-stretch gap-1.5 text-[13px]' : 'mt-1 flex flex-wrap items-center gap-1 text-[11.5px]'}>
      <span className="font-semibold text-warning">{inPrompt ? 'In stock instead — swap, no count change:' : `${line.stock <= 0 ? 'Out of stock' : `Only ${qtyText(line.stock)} left`} — swap for`}</span>
      {alts.map((a) => {
        const price = till.saleType === 'wholesale' && num(a.wholesalePrice) > 0 ? a.wholesalePrice : a.sellingPrice
        return (
          <button key={a.productId} type="button" onClick={() => void swap(a.productId, a.name)}
            className={inPrompt ? 'rounded-[var(--radius-sm)] border border-brand bg-brand-soft px-3 py-2 text-left font-semibold hover:bg-brand hover:text-brand-foreground'
              : 'max-w-full rounded-md border border-dashed border-brand bg-brand-soft px-2 py-0.5 text-left font-semibold hover:border-solid'}>
            ⇄ {a.name}{a.tier && ` · ${a.tier}`} · {qtyText(a.stock)} left · {money(sym, price)}
          </button>
        )
      })}
    </div>
  )
}
