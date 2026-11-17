import { useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
import type { SellLayout } from '@/api/types.gen'
import { ARRANGEMENTS, GAP, PAD, type Area } from '@/features/till/layout/arrangements'
import { cn } from '@/lib/utils'
import { dragBar, innerOf, minsOf } from './layoutDrag'

/** The till's top bar, in real px. */
const TOP = 60
/** Below these heights a stacked part gets cramped (real px). */
const SHORT: Record<Area, number> = { cat: 260, cart: 240, pay: 430 }
export const AREA_NAME: Record<Area, string> = { cat: 'Products', cart: 'Cart', pay: 'Payment' }

/** Sample words, drawn at the till's own type scale so Text size shows. */
const ITEMS = ['Panadol 500mg', 'Vitamin C 1000', 'Cetirizine 10mg', 'Omega 3', 'Bandage 5cm', 'Hand wash', 'Zinc 20mg', 'Amoxil 250mg', 'Glucose 500g']
const price = (i: number) => `Rs ${(120 + i * 85).toLocaleString('en')}.00`

/** A rough drawing of what each section holds, at the chosen card size. */
function Body({ area, card }: { area: Area; card: string }) {
  if (area === 'cat') {
    const n = card === 'T' ? 10 : card === 'S' ? 18 : card === 'M' ? 12 : 6
    return (
      <div className="flex h-full flex-col gap-3">
        <div className="flex h-12 items-center rounded-[var(--radius-sm)] border border-border bg-card px-4 text-till-body text-muted-foreground">Search products…</div>
        {card === 'T' ? (
          <div className="flex flex-col overflow-hidden rounded-[var(--radius-sm)] border border-border bg-card">
            {Array.from({ length: n }, (_, i) => (
              <div key={i} className="flex h-10 items-center justify-between gap-2 border-b border-border px-3">
                <span className="truncate text-till-body font-semibold">{ITEMS[i % ITEMS.length]}</span>
                <span className="shrink-0 text-till-body font-bold">{price(i)}</span>
              </div>
            ))}
          </div>
        ) : (
          <div className={cn('grid gap-2.5', card === 'S' ? 'grid-cols-[repeat(auto-fill,minmax(140px,1fr))]' : card === 'M' ? 'grid-cols-[repeat(auto-fill,minmax(170px,1fr))]' : 'grid-cols-[repeat(auto-fill,minmax(190px,1fr))]')}>
            {Array.from({ length: n }, (_, i) => (
              <div key={i} className={cn('flex flex-col justify-end overflow-hidden rounded-[var(--radius-sm)] border border-border bg-card p-2', card === 'S' ? 'h-16' : card === 'M' ? 'h-24' : 'h-40')}>
                <span className="truncate text-till-body font-semibold">{ITEMS[i % ITEMS.length]}</span>
                <span className="text-till-lead font-bold">{price(i)}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    )
  }
  if (area === 'cart')
    return (
      <div className="flex flex-col">
        <div className="flex h-14 items-center border-b border-border px-4 text-till-title font-bold">Cart · 7 items</div>
        {Array.from({ length: 7 }, (_, i) => (
          <div key={i} className="mx-4 flex h-20 items-center justify-between gap-2 border-b border-dashed border-border">
            <span className="flex min-w-0 flex-col"><span className="truncate text-till-body font-semibold">{ITEMS[i]}</span><span className="text-till-sm text-muted-foreground">1 × {price(i)}</span></span>
            <span className="shrink-0 text-till-price font-bold">{price(i)}</span>
          </div>
        ))}
      </div>
    )
  return (
    <div className="flex h-full flex-col gap-3 p-4">
      <div className="flex h-14 items-center justify-between rounded-[var(--radius-sm)] bg-muted px-4"><span className="text-till-lead">Total</span><span className="text-till-total font-bold">Rs 3,265.00</span></div>
      <div className="grid grid-cols-4 gap-2">{['Cash', 'Card', 'Credit', 'Cheque'].map((m) => <div key={m} className="grid h-14 place-items-center rounded-[var(--radius-sm)] bg-muted text-till-sm font-semibold">{m}</div>)}</div>
      <div className="flex h-12 items-center rounded-[var(--radius-sm)] bg-muted px-4 text-till-body text-muted-foreground">Amount given</div>
      <div className="mt-auto grid h-14 place-items-center rounded-[var(--radius-sm)] bg-brand text-till-xl font-bold text-brand-foreground">Pay</div>
    </div>
  )
}

function Section({ area, card, style, short, size }: { area: Area; card: string; style: object; short: boolean; size: string }) {
  return (
    <div className={cn('relative min-h-0 overflow-hidden', area !== 'cat' && 'rounded-[var(--radius)] border border-border bg-card')} style={style}>
      <Body area={area} card={card} />
      <span className="absolute top-3 right-3 flex flex-col items-end gap-1">
        <span className="rounded-pill bg-foreground px-3 py-1 text-xl font-bold text-background">{AREA_NAME[area]}</span>
        <span className="rounded-pill bg-brand px-3 py-1 text-xl font-bold tabular-nums text-brand-foreground">{size}</span>
      </span>
      {short && <span className="absolute inset-x-3 bottom-3 rounded-[var(--radius-sm)] bg-danger px-3 py-2 text-center text-xl font-bold text-background">Too short for {AREA_NAME[area]}</span>}
    </div>
  )
}

export function Bar({ vertical, onDrag, label }: { vertical: boolean; onDrag: (d: number) => void; label: string }): ReactNode {
  const from = useRef<number | null>(null)
  return (
    <div role="separator" aria-label={label} aria-orientation={vertical ? 'vertical' : 'horizontal'}
      className={cn('group grid shrink-0 touch-none place-items-center', vertical ? 'cursor-col-resize' : 'cursor-row-resize')}
      style={vertical ? { width: GAP } : { height: GAP }}
      onPointerDown={(e) => { from.current = vertical ? e.clientX : e.clientY; e.currentTarget.setPointerCapture(e.pointerId) }}
      onPointerMove={(e) => {
        if (from.current === null) return
        const at = vertical ? e.clientX : e.clientY
        onDrag(at - from.current)
        from.current = at
      }}
      onPointerUp={() => { from.current = null }}
      onPointerCancel={() => { from.current = null }}
      onLostPointerCapture={() => { from.current = null }}>
      <span className={cn('rounded-pill bg-brand opacity-60 group-hover:opacity-100', vertical ? 'h-24 w-2' : 'h-2 w-24')} />
    </div>
  )
}

/** The Sell page at the size's real shape, scaled to fit: drag the bars between
 * sections; nothing goes below its minimum (the server checks it again). */
export function LayoutPreview({ w, h, layout, onChange }: { w: number; h: number; layout: SellLayout; onChange: (l: SellLayout) => void }) {
  const box = useRef<HTMLDivElement>(null)
  const [room, setRoom] = useState(800)
  useEffect(() => {
    const el = box.current
    if (!el) return
    const ro = new ResizeObserver(() => setRoom(el.clientWidth))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  const a = ARRANGEMENTS.find((x) => x.id === layout.arr) ?? ARRANGEMENTS[0]
  const scale = Math.min(room / w, 560 / h)
  const inner = innerOf(a.id, w)
  const mins = minsOf(a.id)
  const hIn = h - TOP - 2 * PAD - GAP
  const split = layout.split || 0.5
  const sizeOf = (i: number) => `${Math.round(layout.fr[i] * inner)} px · ${Math.round(layout.fr[i] * 100)}%`

  return (
    <div ref={box} className="w-full">
      <div className="relative mx-auto overflow-hidden rounded-[var(--radius-sm)] border border-border bg-background" style={{ width: w * scale, height: h * scale }}>
        <div className="absolute top-0 left-0 flex origin-top-left flex-col" style={{ width: w, height: h, transform: `scale(${scale})`, '--till-text': (layout.text || 100) / 100 } as CSSProperties}>
          <div className="flex shrink-0 items-center gap-4 border-b border-border bg-card px-5 text-2xl font-bold" style={{ height: TOP }}>
            Sell <span className="font-medium text-muted-foreground">Returns · Credit · Receipts</span>
            <span className="ml-auto rounded-pill bg-foreground px-3 py-1 text-xl text-background">{w} × {h}</span>
          </div>
          <div className="flex min-h-0 flex-1" style={{ padding: PAD }}>
            {a.cols.map((col, i) => (
              <div key={`${a.id}-${i}`} className="contents">
                {i > 0 && <Bar vertical label={`Between ${col.map((x) => AREA_NAME[x]).join(' and ')} and the column before`}
                  onDrag={(d) => onChange({ ...layout, fr: dragBar(layout.fr, i, d / scale, inner, mins) })} />}
                <div className="flex min-h-0 flex-col" style={{ width: layout.fr[i] * inner }}>
                  {col.length === 1 ? (
                    <Section area={col[0]} card={layout.card} style={{ flex: 1 }} short={false} size={sizeOf(i)} />
                  ) : (
                    <>
                      <Section area={col[0]} card={layout.card} style={{ height: split * hIn }} short={split * hIn < SHORT[col[0]]}
                        size={`${sizeOf(i)} · ${Math.round(split * hIn)} px high`} />
                      <Bar vertical={false} label={`Between ${AREA_NAME[col[0]]} and ${AREA_NAME[col[1]]}`}
                        onDrag={(d) => onChange({ ...layout, split: Math.min(0.8, Math.max(0.2, split + d / scale / hIn)) })} />
                      <Section area={col[1]} card={layout.card} style={{ flex: 1 }} short={(1 - split) * hIn < SHORT[col[1]]}
                        size={`${sizeOf(i)} · ${Math.round((1 - split) * hIn)} px high`} />
                    </>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
