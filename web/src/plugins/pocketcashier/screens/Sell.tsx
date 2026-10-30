import { ScanBarcode, Search } from 'lucide-react'
import { useCallback, useState } from 'react'
import type { CopyProduct, TillCopy } from '@/api/types.gen'
import type { Action } from '@/features/till/cart/store'
import type { TillState } from '@/features/till/cart/types'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD, GLASS } from '../bits'
import { byBarcode, search } from '../catalog'
import { Scanner } from './Scanner'

/** Search, scan or tap a category, tap to add. Stock is the copy's ("about")
 * when the PC is down. */
export function Sell({ copy, cart, dispatch, live }: { copy: TillCopy; cart: TillState; dispatch: (a: Action) => void; live: boolean }) {
  const [text, setText] = useState('')
  const [cat, setCat] = useState(0)
  const [scan, setScan] = useState(false)
  const [miss, setMiss] = useState('')
  const add = useCallback((p: CopyProduct, packId?: number) => {
    const pack = packId ? p.packs.find((k) => k.id === packId) : undefined
    dispatch({ type: 'add', product: p, lots: [], units: pack ? num(pack.size) : undefined })
  }, [dispatch])
  const code = useCallback((c: string) => {
    setScan(false)
    const hit = byBarcode(copy, c)
    if (hit) {
      add(hit.product, hit.packId)
      setText('')
      setMiss('')
    } else setMiss(`No product with code ${c}`)
  }, [copy, add])
  const inCart = (id: number) => cart.lines.filter((l) => l.productId === id).reduce((n, l) => n + l.qty, 0)
  const rows = search(copy, text, cat)

  return (
    <>
      <form className={cn(CARD, 'mx-3 flex h-[42px] items-center gap-2 px-3')} onSubmit={(e) => { e.preventDefault(); if (text.trim()) code(text) }}>
        <Search size={16} className="text-faint" />
        <input aria-label="Search or scan" placeholder="Search or scan…" value={text} onChange={(e) => { setText(e.target.value); setMiss('') }} enterKeyHint="search"
          className="h-full min-w-0 flex-1 bg-transparent text-[15px] outline-none placeholder:text-faint" />
        <button type="button" aria-label="Scan with the camera" onClick={() => setScan(true)} className="text-foreground"><ScanBarcode size={20} /></button>
      </form>
      {miss && <p role="alert" className="mx-3 mt-1 text-sm font-semibold text-destructive">{miss}</p>}
      <div className="flex gap-1.5 overflow-x-auto px-3 pt-2.5 pb-1">
        {[{ id: 0, name: 'All' }, ...copy.categories].map((c) => (
          <button key={c.id} type="button" aria-pressed={cat === c.id} onClick={() => setCat(c.id)}
            className="shrink-0 whitespace-nowrap rounded-pill bg-muted px-3 py-1.5 text-xs font-semibold text-muted-foreground aria-pressed:bg-selected aria-pressed:text-selected-foreground">{c.name}</button>
        ))}
      </div>
      <ul aria-label="Products" className="flex flex-1 flex-col gap-[7px] overflow-y-auto px-3 py-1.5">
        {rows.map((p) => {
          const n = inCart(p.id)
          const stock = num(p.stockQty)
          return (
            <li key={p.id}>
              <button type="button" onClick={() => add(p)} className={cn('flex w-full items-center gap-2.5 rounded-[var(--radius-sm)] border border-border bg-card px-2.5 py-2 text-left shadow-sm', GLASS)}>
                <span className="grid size-[34px] shrink-0 place-items-center rounded-[var(--radius-sm)] bg-brand-soft text-[17px]">{p.emoji || p.name.slice(0, 1)}</span>
                <span className="min-w-0">
                  <span className="block truncate text-[13.5px] font-bold">{p.name}</span>
                  {!p.isService && (
                    <span className={cn('block text-[11.5px] text-muted-foreground', p.lowStock && 'font-semibold text-warning')}>
                      Stock {live ? qtyText(stock) : `~${qtyText(stock)} (copy)`}{p.lowStock ? ' · low' : ''}
                    </span>
                  )}
                </span>
                <span className="ml-auto shrink-0 text-right text-[13.5px] font-bold tabular-nums">
                  {money(copy.symbol, cart.saleType === 'wholesale' && num(p.wholesalePrice) > 0 ? p.wholesalePrice : p.sellingPrice)}
                  {n > 0 && <span className="ml-1.5 inline-grid h-[22px] min-w-[22px] place-items-center rounded-pill bg-brand px-1 text-[11.5px] font-extrabold text-brand-foreground">{qtyText(n)}</span>}
                </span>
              </button>
            </li>
          )
        })}
        {!rows.length && <li className="py-8 text-center text-sm text-muted-foreground">Nothing matches “{text}”</li>}
      </ul>
      {scan && <Scanner onCode={code} onClose={() => setScan(false)} />}
    </>
  )
}
