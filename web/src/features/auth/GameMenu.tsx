import { X } from 'lucide-react'
import { lazy, Suspense, useCallback, useEffect, useState } from 'react'
import { readBest } from './games'

const GAMES = [
  { name: 'Coin catch', emoji: '🪙', desc: 'Catch the falling coins in the drawer. ♥ gives a life back.', best: 'ks-coin-best', Game: lazy(() => import('./CoinGame')) },
  { name: 'Blocks', emoji: '🧱', desc: 'Fit the falling blocks and clear lines.', best: 'ks-blocks-best', Game: lazy(() => import('./BlockGame')) },
  { name: 'Shop Rush', emoji: '🏪', desc: 'Ring up customers, give the right change or swipe the card.', best: 'ks-shop-best', Game: lazy(() => import('./ShopGame')) },
]

/** The login / lock easter egg's menu: pick a game (tap, or 1 2 3); closing a game
 * comes back here, Esc here closes it all. */
export function GameMenu({ onClose }: { onClose: () => void }) {
  const [open, setOpen] = useState<number | null>(null)
  const back = useCallback(() => setOpen(null), [])

  useEffect(() => {
    if (open !== null) return
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { e.preventDefault(); onClose() }
      const n = Number(e.key) - 1
      if (n >= 0 && n < GAMES.length) { e.preventDefault(); setOpen(n) }
      e.stopPropagation()
    }
    window.addEventListener('keydown', key, true)
    ;(document.activeElement as HTMLElement | null)?.blur()
    return () => window.removeEventListener('keydown', key, true)
  }, [open, onClose])

  if (open !== null) {
    const { Game } = GAMES[open]
    return <Suspense><Game onClose={back} /></Suspense>
  }
  return (
    <div role="dialog" aria-label="Games" className="fixed inset-0 z-[100] grid place-items-center overflow-auto bg-hero p-6 text-left text-hero-foreground select-none">
      <button type="button" aria-label="Close the games" onClick={onClose}
        className="absolute top-5 right-5 grid size-11 place-items-center rounded-full bg-white/10 hover:bg-white/20">
        <X size={20} />
      </button>
      <div className="flex w-full max-w-4xl flex-col gap-6">
        <div>
          <h2 className="font-display text-4xl font-bold">🎮 Break time</h2>
          <p className="text-hero-muted">Pick a game — tap it or press 1, 2 or 3. Esc to close.</p>
        </div>
        <div className="grid gap-4 md:grid-cols-3">
          {GAMES.map((g, i) => (
            <button key={g.name} type="button" onClick={() => setOpen(i)}
              className="flex flex-col gap-3 rounded-[var(--radius)] border border-white/10 bg-white/[0.07] p-6 text-left transition hover:-translate-y-0.5 hover:border-brand hover:bg-white/10 active:scale-[0.98]">
              <span className="flex items-start justify-between">
                <span className="text-6xl">{g.emoji}</span>
                <span className="grid size-8 place-items-center rounded-full bg-white/10 text-sm font-bold">{i + 1}</span>
              </span>
              <b className="font-display text-2xl">{g.name}</b>
              <span className="text-sm text-hero-muted">{g.desc}</span>
              <span className="mt-auto text-sm font-semibold">Best <b className="font-display text-lg text-brand">{readBest(g.best)}</b></span>
            </button>
          ))}
        </div>
      </div>
    </div>
  )
}
