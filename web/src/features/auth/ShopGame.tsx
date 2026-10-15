import { Heart, Minus, Plus, X } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useRef, useState } from 'react'
import { api } from '@/api/client'
import type { LoginInfo } from '@/api/types.gen'
import { LKR_NOTES } from '@/features/till/cart/totals'
import { cn } from '@/lib/utils'
import { readBest, saveBest } from './games'
import { CardPay, CashDrawer } from './ShopPay'
import { ReceiptOut } from './ShopReceipt'

const BEST = 'ks-shop-best'
const LIVES = 3
const PRODUCTS = [
  { id: 'milk', name: 'Milk', emoji: '🥛', price: 120 },
  { id: 'bread', name: 'Bread', emoji: '🍞', price: 90 },
  { id: 'eggs', name: 'Egg', emoji: '🥚', price: 30 },
  { id: 'rice', name: 'Rice', emoji: '🍚', price: 250 },
  { id: 'tea', name: 'Tea', emoji: '☕', price: 140 },
  { id: 'soda', name: 'Soda', emoji: '🥤', price: 110 },
  { id: 'biscuit', name: 'Biscuits', emoji: '🍪', price: 80 },
  { id: 'banana', name: 'Banana', emoji: '🍌', price: 20 },
  { id: 'choco', name: 'Chocolate', emoji: '🍫', price: 150 },
] as const
const NAMES = ['Nimal', 'Kumari', 'Fathima', 'Ravi', 'Saman', 'Priya', 'Rizwan', 'Dilani', 'Kasun', 'Malini']
const FACES = ['👨', '👩', '🧕', '👴', '👵', '🧑', '👨‍🦱', '👩‍🦱']
const LKR_COINS = [10, 5, 2, 1]

type Order = Record<string, number>
type Customer = { name: string; face: string; order: Order; total: number; card: boolean; given: number; time: number }
type Phase = 'ready' | 'ring' | 'pay' | 'cash' | 'card' | 'receipt' | 'over'
const WAITING: Phase[] = ['ring', 'pay', 'cash', 'card', 'receipt']

const rand = (n: number) => Math.floor(Math.random() * n)
const priceOf = (id: string) => PRODUCTS.find((p) => p.id === id)!.price

/** A customer for this point of the game: bigger orders and less time as you go. */
function nextCustomer(served: number, notes: number[]): Customer {
  const kinds = Math.min(4, 1 + rand(2) + Math.floor(served / 3))
  const order: Order = {}
  while (Object.keys(order).length < kinds) order[PRODUCTS[rand(PRODUCTS.length)].id] = 1 + rand(served < 4 ? 2 : 3)
  const total = Object.entries(order).reduce((s, [id, q]) => s + priceOf(id) * q, 0)
  // What they hand over: one of the shop's notes that covers it (now and then the exact money).
  const big = [...notes].sort((a, b) => a - b)
  const covers = big.filter((n) => n >= total)
  const top = big[big.length - 1] ?? 5000
  const given = rand(4) === 0 ? total : covers.length ? covers[rand(Math.min(2, covers.length))] : Math.ceil(total / top) * top
  const items = Object.values(order).reduce((s, q) => s + q, 0)
  return {
    name: NAMES[rand(NAMES.length)], face: FACES[rand(FACES.length)], order, total, given,
    card: rand(3) === 0, time: Math.max(12, 26 - served) + items * 2,
  }
}

/** The third login / lock easter egg: Shop Rush. A customer asks for things; ring
 * them up, then take card or cash and give exact change, before the timer runs out.
 * Wrong items, wrong change or too slow costs a heart; three and the shop shuts. */
export default function ShopGame({ onClose }: { onClose: () => void }) {
  const [phase, setPhase] = useState<Phase>('ready')
  const [lives, setLives] = useState(LIVES)
  const [score, setScore] = useState(0)
  const [served, setServed] = useState(0)
  const [best, setBest] = useState(() => readBest(BEST))
  const info = useQuery({ queryKey: ['auth', 'login-info'], queryFn: () => api<LoginInfo>('/auth/login-info') }).data
  // The shop's own notes and coins (Denominations), or Sri Lanka's when none are set.
  const notes = info?.notes?.length ? info.notes.map(Number) : LKR_NOTES
  const coins = info?.coins?.length ? info.coins.map(Number) : LKR_COINS
  const [c, setC] = useState<Customer>(() => nextCustomer(0, LKR_NOTES))
  const [cart, setCart] = useState<Order>({})
  const [left, setLeft] = useState(0)
  const [flash, setFlash] = useState<{ ok: boolean; text: string } | null>(null)
  const busy = useRef(false)
  const customer = useRef<HTMLElement>(null)
  const [handing, setHanding] = useState(false)

  const notesKey = notes.join()
  const newCustomer = useCallback((n: number) => {
    const next = nextCustomer(n, notesKey.split(',').map(Number))
    setC(next); setCart({}); setLeft(next.time); setPhase('ring'); busy.current = false
  }, [notesKey])
  const start = () => { setLives(LIVES); setScore(0); setServed(0); newCustomer(0) }

  // One customer done (served or failed): show why for a moment, then the next one.
  const finish = useCallback((ok: boolean, text: string) => {
    if (busy.current) return
    busy.current = true
    setFlash({ ok, text })
    let livesLeft = lives
    if (ok) { setScore((s) => s + 10 + Math.ceil(left)); setServed((n) => n + 1) }
    else { livesLeft = lives - 1; setLives(livesLeft) }
    setTimeout(() => {
      setFlash(null)
      if (livesLeft <= 0) {
        setPhase('over')
        setScore((s) => { if (s > best) { setBest(s); saveBest(BEST, s) } return s })
      } else newCustomer(served + (ok ? 1 : 0))
    }, 1100)
  }, [lives, left, served, best, newCustomer])

  // The customer's patience.
  useEffect(() => {
    if (!WAITING.includes(phase)) return
    const id = setInterval(() => setLeft((t) => (busy.current ? t : Math.max(0, t - 0.1))), 100)
    return () => clearInterval(id)
  }, [phase])
  useEffect(() => { if (left <= 0 && WAITING.includes(phase)) finish(false, `${c.name} got tired of waiting`) }, [left, phase, c.name, finish])

  useEffect(() => {
    const key = (e: KeyboardEvent) => { if (e.key === 'Escape') { e.preventDefault(); onClose() } e.stopPropagation() }
    window.addEventListener('keydown', key, true)
    ;(document.activeElement as HTMLElement | null)?.blur()
    return () => window.removeEventListener('keydown', key, true)
  }, [onClose])

  const add = (id: string, d: number) => setCart((o) => {
    const q = Math.max(0, (o[id] ?? 0) + d)
    const next = { ...o, [id]: q }
    if (!q) delete next[id]
    return next
  })
  const cartTotal = Object.entries(cart).reduce((s, [id, q]) => s + priceOf(id) * q, 0)
  const charge = () => {
    const same = Object.keys(c.order).length === Object.keys(cart).length && Object.entries(c.order).every(([id, q]) => cart[id] === q)
    if (same) setPhase('pay')
    else finish(false, 'Wrong items rung up')
  }
  const take = (card: boolean) => {
    if (card !== c.card) return finish(false, c.card ? 'They wanted to pay by card' : 'They were paying cash')
    setPhase(card ? 'card' : 'cash')
  }
  // Paid right: print the receipt — the sale ends when it's in the customer's hand.
  const give = (v: number) => {
    const due = c.given - c.total
    if (v === due) setPhase('receipt')
    else finish(false, `Change should be Rs ${due}`)
  }
  const charged = (v: number) => (v === c.total ? setPhase('receipt') : finish(false, `Charged Rs ${v} — it was Rs ${c.total}`))
  const delivered = () => finish(true, c.card ? 'Card sale done ✓' : 'Right change ✓')
  const pct = Math.max(0, Math.min(100, (left / c.time) * 100))

  const panel = 'rounded-[var(--radius)] bg-white/[0.07] p-4'
  const chip = 'rounded-[var(--radius-sm)] bg-white/10 font-bold active:scale-95 hover:bg-white/15'
  return (
    <div role="dialog" aria-label="Shop game" className="fixed inset-0 z-[100] flex flex-col gap-4 overflow-auto bg-hero p-4 text-left text-hero-foreground select-none md:p-6">
      <header className="flex items-center gap-4">
        <b className="font-display text-2xl">🏪 Shop Rush</b>
        <span className="font-display text-xl tabular-nums">{score}</span>
        <span className="text-sm text-hero-muted">Best {best} · served {served}</span>
        <span className="ml-auto flex gap-1" aria-label={`${lives} hearts left`}>
          {Array.from({ length: LIVES }, (_, i) => <Heart key={i} size={22} className={i < lives ? 'fill-destructive text-destructive' : 'text-hero-muted'} />)}
        </span>
        <button type="button" aria-label="Close the game" onClick={onClose} className="grid size-11 place-items-center rounded-full bg-white/10 hover:bg-white/20"><X size={20} /></button>
      </header>

      {phase === 'ready' || phase === 'over' ? (
        <div className="m-auto flex max-w-md flex-col items-center gap-4 text-center">
          <div className="text-6xl">{phase === 'over' ? '🔒' : '🛒'}</div>
          <h2 className="font-display text-3xl font-bold">{phase === 'over' ? `Shop closed · ${score}` : 'Shop Rush'}</h2>
          <p className="text-hero-muted">
            {phase === 'over'
              ? score >= best && score > 0 ? 'New best!' : `Best ${best}. Three unhappy customers and the shop shuts.`
              : 'Customers ask for things: ring them up, then take the cash and give change from the drawer, or swipe the card and key in the amount — work the change out yourself — and hand over the receipt — before they lose patience. Three mistakes and the shop shuts.'}
          </p>
          <button type="button" onClick={start} className="h-14 rounded-[var(--radius-sm)] bg-brand px-10 text-lg font-bold text-brand-foreground">
            {phase === 'over' ? 'Open again' : 'Open the shop'}
          </button>
        </div>
      ) : (
        <div className="grid min-h-0 flex-1 gap-4 md:grid-cols-[300px_1fr_320px]">
          <section ref={customer} className={cn(panel, 'flex flex-col gap-3 transition', handing && 'ring-2 ring-brand')}>
            <div className="flex items-center gap-3">
              <span className="text-5xl">{c.face}</span>
              <div><b className="text-lg">{c.name}</b><p className="text-sm text-hero-muted">wants…</p></div>
            </div>
            <ul className="flex flex-col gap-1.5 rounded-[var(--radius-sm)] bg-white/10 p-3 text-[17px] font-semibold">
              {Object.entries(c.order).map(([id, q]) => {
                const p = PRODUCTS.find((x) => x.id === id)!
                return <li key={id}>{q} × {p.emoji} {p.name}</li>
              })}
            </ul>
            {phase !== 'ring' && (
              <p className="rounded-[var(--radius-sm)] border-[1.5px] border-brand bg-white/10 p-3 font-semibold">
                {c.card ? '💳 “Card, please.”' : `💵 “Cash.” Hands you Rs ${c.given}`}
              </p>
            )}
            <div className="mt-auto">
              <div className="mb-1 flex justify-between text-sm text-hero-muted"><span>Patience</span><span className="tabular-nums">{Math.ceil(left)}s</span></div>
              <div className="h-3 overflow-hidden rounded-full bg-white/10">
                <div className={cn('h-full rounded-full transition-[width] duration-100', pct < 30 ? 'bg-destructive' : 'bg-brand')} style={{ width: `${pct}%` }} />
              </div>
            </div>
          </section>

          <section className="grid content-start gap-3 [grid-template-columns:repeat(auto-fill,minmax(170px,1fr))]">
            {PRODUCTS.map((p) => (
              <button key={p.id} type="button" disabled={phase !== 'ring'} onClick={() => add(p.id, 1)}
                className={cn(chip, 'flex flex-col items-center gap-1.5 py-7 text-lg disabled:opacity-40')}>
                <span className="text-6xl">{p.emoji}</span>
                <span>{p.name}</span>
                <span className="text-sm font-semibold text-hero-muted tabular-nums">Rs {p.price}</span>
              </button>
            ))}
          </section>

          <section className={cn(panel, 'flex flex-col gap-3')}>
            <b>Cart</b>
            <div className="flex flex-col gap-1.5">
              {Object.entries(cart).map(([id, q]) => {
                const p = PRODUCTS.find((x) => x.id === id)!
                return (
                  <div key={id} className="flex items-center gap-2">
                    <span className="flex-1">{p.emoji} {p.name}</span>
                    {phase === 'ring' && <button type="button" aria-label={`Fewer ${p.name}`} onClick={() => add(id, -1)} className={cn(chip, 'grid size-8 place-items-center')}><Minus size={14} /></button>}
                    <b className="w-6 text-center tabular-nums">{q}</b>
                    {phase === 'ring' && <button type="button" aria-label={`More ${p.name}`} onClick={() => add(id, 1)} className={cn(chip, 'grid size-8 place-items-center')}><Plus size={14} /></button>}
                    <span className="w-16 text-right tabular-nums">{priceOf(id) * q}</span>
                  </div>
                )
              })}
              {!Object.keys(cart).length && <p className="text-sm text-hero-muted">Tap the products they ask for.</p>}
            </div>
            <div className="flex justify-between border-t border-white/15 pt-2 font-display text-xl font-bold"><span>Total</span><span className="tabular-nums">Rs {cartTotal}</span></div>
            {phase === 'ring' && (
              <button type="button" onClick={charge} disabled={!cartTotal} className="h-12 rounded-[var(--radius-sm)] bg-brand font-bold text-brand-foreground disabled:opacity-40">Charge</button>
            )}
            {phase === 'pay' && (
              <div className="grid grid-cols-2 gap-2">
                <button type="button" onClick={() => take(false)} className={cn(chip, 'h-12')}>💵 Cash</button>
                <button type="button" onClick={() => take(true)} className={cn(chip, 'h-12')}>💳 Card</button>
              </div>
            )}
            {phase === 'cash' && <CashDrawer key={c.name + c.total} given={c.given} notes={notes} coins={coins} onGive={give} />}
            {phase === 'card' && <CardPay key={c.name + c.total} onDone={charged} />}
            {phase === 'receipt' && (
              <ReceiptOut key={c.name + c.total} shop={info?.receipt ?? { name: info?.shopName || 'Karots Suite', address: '', phone: '', footer: '' }}
                no={served + 1} total={c.total} card={c.card} given={c.given} target={customer} onDragging={setHanding} onDelivered={delivered}
                lines={Object.entries(c.order).map(([id, q]) => ({ name: PRODUCTS.find((p) => p.id === id)!.name, qty: q, price: priceOf(id) }))} />
            )}
          </section>
        </div>
      )}

      {flash && (
        <div className="pointer-events-none fixed inset-0 grid place-items-center">
          <div className={cn('rounded-[var(--radius)] px-8 py-5 font-display text-2xl font-bold shadow-[var(--shadow)]', flash.ok ? 'bg-success text-hero-foreground' : 'bg-destructive text-hero-foreground')}>
            {flash.text}
          </div>
        </div>
      )}
    </div>
  )
}
