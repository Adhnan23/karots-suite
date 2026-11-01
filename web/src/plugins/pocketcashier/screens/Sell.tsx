import { Info, ScanBarcode, Search } from 'lucide-react'
import { useCallback, useState } from 'react'
import type { CopyProduct, Lot, TillCopy } from '@/api/types.gen'
import type { Action } from '@/features/till/cart/store'
import type { TillState } from '@/features/till/cart/types'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD, GLASS } from '../bits'
import { byBarcode, search, sellable } from '../catalog'
import { addFlow } from '../addFlow'
import { InfoSheet } from './InfoSheet'
import { PackSheet } from './PackSheet'
import { PriceSheet } from './PriceSheet'
import { Scanner } from './Scanner'

type Pick = { kind: 'price' | 'pack'; product: CopyProduct; lots: Lot[]; units?: number; lot?: Lot }

/** Search, scan or tap a category, tap to add. Stock is the copy's ("about")
 * when the PC is down. */
export function Sell({ copy, cart, dispatch, live }: { copy: TillCopy; cart: TillState; dispatch: (a: Action) => void; live: boolean }) {
  const [text, setText] = useState('')
  const [cat, setCat] = useState(0)
  const [scan, setScan] = useState(false)
  const [miss, setMiss] = useState('')
  const [pick, setPick] = useState<Pick | null>(null)
  const [info, setInfo] = useState<CopyProduct | null>(null)
  const add = useCallback((p: CopyProduct, packId?: number) => {
    const step = addFlow(copy, p, packId)
    if (step.kind === 'add') dispatch({ type: 'add', product: p, lots: step.lots, units: step.units })
    else setPick({ ...step, product: p })
  }, [copy, dispatch])
  // After Which price?, a product sold by the pack still asks how (as the PC).
  const lotPicked = (pk: Pick, lot: Lot) => {
    if (!pk.units && pk.product.packs.length && !pk.product.isService) return setPick({ kind: 'pack', product: pk.product, lots: pk.lots, lot })
    setPick(null)
    dispatch({ type: 'add', product: pk.product, lot, lots: pk.lots, units: pk.units })
  }
  const code = useCallback((c: string) => {
    setScan(false)
    const hit = byBarcode(copy, c)
    if (hit && !sellable(hit.product)) setMiss(`${hit.product.name}: sell this one at the counter`)
    else if (hit) {
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
            <li key={p.id} className="relative">
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
                <span className="w-7 shrink-0" />
              </button>
              <button type="button" aria-label={`About ${p.name}`} onClick={() => setInfo(p)}
                className="absolute top-1/2 right-1.5 grid size-7 -translate-y-1/2 place-items-center rounded-full text-faint"><Info size={16} /></button>
            </li>
          )
        })}
        {!rows.length && <li className="py-8 text-center text-sm text-muted-foreground">Nothing matches “{text}”</li>}
      </ul>
      {scan && <Scanner onCode={code} onClose={() => setScan(false)} />}
      {info && <InfoSheet product={info} copy={copy} live={live} onAdd={() => add(info)} onClose={() => setInfo(null)} />}
      {pick?.kind === 'price' && <PriceSheet product={pick.product} lots={pick.lots} sym={copy.symbol} onPick={(l) => lotPicked(pick, l)} onClose={() => setPick(null)} />}
      {pick?.kind === 'pack' && (
        <PackSheet product={pick.product} lot={pick.lot} saleType={cart.saleType} sym={copy.symbol} onClose={() => setPick(null)}
          onUnits={(units) => { setPick(null); dispatch({ type: 'add', product: pick.product, lot: pick.lot, lots: pick.lots, units }) }} />
      )}
    </>
  )
}
