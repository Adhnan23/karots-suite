import { useQuery } from '@tanstack/react-query'
import { Minus, Plus, Tag, X } from 'lucide-react'
import { motion } from 'motion/react'
import { useEffect, useState } from 'react'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { priceOptionsQuery } from '../api'
import { Seg } from '../bits'
import { lotsFor } from '../cart/lines'
import { lineDiscount, lineGross } from '../cart/totals'
import type { CartLine } from '../cart/types'
import { useTillApi } from '../TillUi'
import { PackChips } from './PackChips'
import { useTill } from '../TillProvider'
import { FIELD } from '../ui'

function DiscountChip({ line, sym }: { line: CartLine; sym: string }) {
  const { dispatch } = useTill()
  const [open, setOpen] = useState(false)
  const on = num(line.discount) > 0
  const set = (discount: string, discountType = line.discountType) => dispatch({ type: 'lineDiscount', key: line.key, discount, discountType })
  if (!open) {
    return (
      <button type="button" onClick={() => setOpen(true)}
        className={cn('flex h-7 items-center gap-1 rounded-pill border border-dashed border-border px-2.5 text-[12.5px] font-semibold',
          on && 'border-solid border-transparent bg-[color-mix(in_srgb,var(--success)_18%,transparent)] text-success')}>
        <Tag size={14} />
        {on ? (line.discountType === 'percent' ? `${line.discount}% off` : `${money(sym, line.discount)} off each`) : 'Discount'}
      </button>
    )
  }
  return (
    <span className="flex items-center gap-1">
      <input autoFocus aria-label={`Discount for ${line.name}`} inputMode="decimal" autoComplete="off" value={line.discount}
        onChange={(e) => set(e.target.value.replace(/[^\d.]/g, ''))}
        onKeyDown={(e) => (e.key === 'Enter' || e.key === 'Escape') && setOpen(false)} className={cn(FIELD, 'h-7 w-16 px-2 text-[13px]')} />
      <Seg label="Discount type" value={line.discountType} options={[['fixed', sym], ['percent', '%']]} onChange={(t) => set(line.discount, t)} />
      <button type="button" onClick={() => setOpen(false)} className="px-1 text-[12px] font-semibold text-muted-foreground">Done</button>
    </span>
  )
}

export function CartLineRow({ line, sym }: { line: CartLine; sym: string }) {
  const { state, dispatch } = useTill()
  const till = useTillApi()
  const opts = useQuery(priceOptionsQuery).data
  const [text, setText] = useState(qtyText(line.qty))
  useEffect(() => setText(qtyText(line.qty)), [line.qty])
  const setQty = (qty: number) => dispatch({ type: 'setQty', key: line.key, qty, lots: lotsFor(opts, line.productId) })
  const gross = lineGross(line)
  const disc = lineDiscount(line)
  const expired = line.batchLabel.startsWith('exp ') && line.batchLabel.slice(4) < new Date().toLocaleDateString('en-CA')
  return (
    <motion.div initial={{ backgroundColor: 'var(--brand-soft)' }} animate={{ backgroundColor: 'rgba(0,0,0,0)' }} transition={{ duration: 0.7 /* not a UI transition: the new-line flash */ }}
      className="grid grid-cols-[minmax(0,1fr)_auto_auto] items-center gap-x-3 gap-y-1.5 rounded-[var(--radius-sm)] border-t border-dashed border-border px-2 py-2.5 first:border-t-0">
      <div className="min-w-0">
        <div className="text-[14px] font-semibold leading-snug">{line.description || line.name}</div>
      </div>
      {line.fixedQty ? (
        <span aria-label={`Quantity of ${line.name}`} className="px-3 text-[14px] font-bold tabular-nums text-muted-foreground">×{qtyText(line.qty)}</span>
      ) : <div className="flex items-center gap-0.5 rounded-pill border border-border bg-muted p-0.5">
        <button type="button" aria-label={`Fewer ${line.name}`} onClick={() => setQty(line.qty - 1)} className="grid h-7 w-7 place-items-center rounded-full hover:bg-border"><Minus size={14} /></button>
        <input aria-label={`Quantity of ${line.name}`} inputMode="decimal" autoComplete="off" value={text}
          onChange={(e) => setText(e.target.value.replace(/[^\d.]/g, ''))} onBlur={() => setQty(num(text))}
          onKeyDown={(e) => e.key === 'Enter' && e.currentTarget.blur()}
          className="w-10 bg-transparent text-center text-[14px] font-bold tabular-nums outline-none" />
        <button type="button" aria-label={`More ${line.name}`} onClick={() => setQty(line.qty + 1)} className="grid h-7 w-7 place-items-center rounded-full hover:bg-border"><Plus size={14} /></button>
      </div>}
      <div className="min-w-[84px] text-right text-[14.5px] font-bold tabular-nums">
        {disc > 0 && <s className="block text-[11.5px] font-medium text-faint">{money(sym, gross)}</s>}
        {gross < 0 ? `−${money(sym, -gross)}` : money(sym, gross - disc)}
      </div>
      {slots.tillLineHints.length > 0 && (
        <div className="col-span-full empty:hidden">
          {slots.tillLineHints.map(({ key, Hint }) => <Hint key={key} till={till} line={line} lines={state.lines} />)}
        </div>
      )}
      <div className="col-span-full flex flex-wrap items-center gap-1.5 text-[12px] text-muted-foreground">
        {line.payout ? <span className="font-semibold text-warning">Given to the customer</span> : <span className="tabular-nums">
          {money(sym, line.unitPrice)} each
          {line.packs?.map((k) => k.price > 0 && k.price < k.size * line.unitPrice && <span key={k.id}> · {k.name} {money(sym, k.price)}</span>)}
        </span>}
        {line.batchLabel && (
          <span className={cn('rounded-md px-1.5 font-semibold', expired ? 'bg-[color-mix(in_srgb,var(--danger)_14%,transparent)] text-danger' : 'bg-muted text-muted-foreground')}>
            {line.batchLabel}
          </span>
        )}
        {line.warrantyMonths > 0 && <span className="rounded-md bg-muted px-1.5">{line.warrantyMonths} mo warranty</span>}
        {line.allowOversell && <span className="font-semibold text-warning">{line.isService ? 'material count corrected' : 'found at till'}</span>}
        <PackChips line={line} sym={sym} onQty={setQty} />
        {!line.payout && !line.noDiscount && <DiscountChip line={line} sym={sym} />}
        <button type="button" aria-label={`Remove ${line.name}`} onClick={() => dispatch({ type: 'remove', key: line.key })}
          className="ml-auto grid size-8 place-items-center rounded-full text-faint hover:bg-muted hover:text-danger"><X size={17} /></button>
      </div>
      {line.trackSerial && line.serials.length > 0 && (
        <div className="col-span-full grid grid-cols-2 gap-1.5">
          {line.serials.map((s, i) => (
            <input key={i} aria-label={`Serial ${i + 1} for ${line.name}`} placeholder={`Serial ${i + 1}`} autoComplete="off" value={s}
              onChange={(e) => dispatch({ type: 'serial', key: line.key, index: i, value: e.target.value })} className={cn(FIELD, 'h-8 text-[13px]')} />
          ))}
        </div>
      )}
    </motion.div>
  )
}
