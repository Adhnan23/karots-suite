import { Info, ScanBarcode, Search } from 'lucide-react'
import { useCallback, useState } from 'react'
import type { CopyProduct, Lot, TillCopy } from '@/api/types.gen'
import type { Action } from '@/features/till/cart/store'
import type { TillState } from '@/features/till/cart/types'
import { money, num, qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD, GLASS } from '../bits'
import { byBarcode, search, sellable } from '../catalog'
import { refresh } from '../sync'
import { addFlow } from '../addFlow'
import { usePhoneTill } from '../tillApi'
import { slots, type TillMenuRoot } from '@/plugins/slots'
import { InfoSheet } from './InfoSheet'
import { PackSheet } from './PackSheet'
import { PriceSheet } from './PriceSheet'
import { QuickItemSheet } from './QuickItemSheet'
import { UnknownCodeSheet } from './UnknownCodeSheet'
import { Scanner } from './Scanner'
import { PluginPanel } from './PluginPanel'
import { phoneActions, type PhoneAction } from './phoneActions'

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
  const till = usePhoneTill()
  const [menu, setMenu] = useState<{ root: TillMenuRoot; product?: CopyProduct } | null>(null)
  const [action, setAction] = useState<{ item: PhoneAction; text: string } | null>(null)
  const add = useCallback((p: CopyProduct, packId?: number) => {
    const claimed = slots.tillMenu.find((m) => m.claims?.(p))
    if (claimed) return setMenu({ root: claimed, product: p })
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
  const [quick, setQuick] = useState<{ name: string; barcode: string } | null>(null)
  // A code nothing has: an item's other barcode, or a new item (the PC's prompt).
  const [unknown, setUnknown] = useState('')
  const code = useCallback((c: string) => {
    setScan(false)
    const ticket = phoneActions().find((a) => a.claimsText?.(c))
    if (ticket) {
      setText('')
      return setAction({ item: ticket, text: c })
    }
    const hit = byBarcode(copy, c)
    const found = hit ? [] : search(copy, c)
    if (hit && !sellable(hit.product)) setMiss(`${hit.product.name}: sell this one at the counter`)
    else if (hit || found.length === 1) {
      if (hit) add(hit.product, hit.packId)
      else add(found[0])
      setText('')
      setMiss('')
    } else if (!found.length) {
      setText('')
      setMiss('')
      // A code: an item's other barcode, or a new item. A name: a new item (as the PC).
      if (/^\d+$/.test(c.trim())) setUnknown(c.trim())
      else setQuick({ name: c.trim(), barcode: '' })
    }
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
      <div className="mx-3 mt-1.5 flex items-center gap-2">
        {miss && <p role="alert" className="min-w-0 flex-1 text-sm font-semibold text-destructive">{miss}</p>}
        <button type="button" onClick={() => { setQuick({ name: '', barcode: '' }); setMiss('') }}
          title="Sell an item that isn't in the catalogue yet"
          className="ml-auto shrink-0 rounded-pill bg-muted px-3 py-1.5 text-xs font-bold text-muted-foreground">
          + Quick item
        </button>
      </div>
      {slots.tillMenu.length > 0 && (
        <div role="group" aria-label="Plugin menus" className="flex gap-1.5 overflow-x-auto px-3 pt-2.5">
          {slots.tillMenu.map((m) => (
            <button key={m.key} type="button" onClick={() => setMenu({ root: m })} className="min-h-9 shrink-0 whitespace-nowrap rounded-pill bg-brand-soft px-3 text-[13px] font-bold">
              {m.emoji} {m.label}
            </button>
          ))}
        </div>
      )}
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
      {menu && till && (
        <PluginPanel title={menu.root.label} onClose={() => setMenu(null)}>
          <menu.root.Panel till={till} product={menu.product} onDone={() => setMenu(null)} />
        </PluginPanel>
      )}
      {action && till && (
        <PluginPanel title={action.item.label} onClose={() => setAction(null)}>
          <action.item.Panel till={till} text={action.text} onClose={() => setAction(null)} />
        </PluginPanel>
      )}
      {scan && <Scanner onCode={code} onClose={() => setScan(false)} />}
      {unknown && (
        <UnknownCodeSheet code={unknown} copy={copy} live={live} onClose={() => setUnknown('')}
          onNew={() => { setQuick({ name: '', barcode: unknown }); setUnknown('') }}
          onGiven={(p) => { add(p); void refresh() }} />
      )}
      {quick && (
        <QuickItemSheet {...quick} live={live} onClose={() => setQuick(null)}
          onAdded={(p, qty) => { dispatch({ type: 'quickSet', product: p, qty }); setText(''); void refresh() }} />
      )}
      {info && <InfoSheet product={info} copy={copy} live={live} onAdd={() => add(info)} onClose={() => setInfo(null)} />}
      {pick?.kind === 'price' && <PriceSheet product={pick.product} lots={pick.lots} sym={copy.symbol} onPick={(l) => lotPicked(pick, l)} onClose={() => setPick(null)} />}
      {pick?.kind === 'pack' && (
        <PackSheet product={pick.product} lot={pick.lot} saleType={cart.saleType} sym={copy.symbol} onClose={() => setPick(null)}
          onUnits={(units) => { setPick(null); dispatch({ type: 'add', product: pick.product, lot: pick.lot, lots: pick.lots, units }) }} />
      )}
    </>
  )
}
