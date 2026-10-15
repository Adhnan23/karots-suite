import { lazy, Suspense, useCallback, useEffect, useRef, useState } from 'react'

const GAMES = [lazy(() => import('./CoinGame')), lazy(() => import('./BlockGame')), lazy(() => import('./ShopGame'))]

function useClock() {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 15_000)
    return () => clearInterval(id)
  }, [])
  return now
}

/** Left panel of login/lock (theme.md §5): brand on top, a live clock and the date
 * in the middle, the server status at the foot. Collapses to a compact header strip
 * on a phone. Tapping the clock 7 times opens a little game at random: coins, blocks or the shop (an easter egg). */
export function LoginHero({ shopName }: { shopName: string }) {
  const now = useClock()
  const h24 = now.getHours()
  const minutes = String(now.getMinutes()).padStart(2, '0')
  const ampm = h24 < 12 ? 'AM' : 'PM'
  const hours = h24 % 12 || 12
  const date = now.toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' })
  const [game, setGame] = useState<number | null>(null)
  const taps = useRef<number[]>([])
  const Game = game === null ? null : GAMES[game]
  const close = useCallback(() => setGame(null), [])
  const tap = () => {
    const t = Date.now()
    taps.current = [...taps.current.filter((x) => t - x < 3000), t]
    if (taps.current.length >= 7) { taps.current = []; setGame(Math.floor(Math.random() * GAMES.length)) }
  }

  return (
    <div className="relative flex flex-col gap-2 overflow-hidden bg-hero bg-[image:var(--hero-art)] p-5 text-hero-foreground md:h-full md:items-center md:justify-center md:gap-4 md:p-11 md:text-center">
      <div className="flex items-center gap-3 font-display text-base font-bold md:absolute md:top-11 md:left-11 md:text-lg">
        <span className="grid h-[30px] w-[30px] flex-none place-items-center rounded-[9px] bg-brand font-display text-sm font-extrabold text-brand-foreground shadow-[var(--glow)]">
          K
        </span>
        {shopName}
      </div>
      <div onClick={tap} className="select-none font-display text-[64px] leading-none font-extrabold tracking-tight md:text-[168px]">
        {hours}:{minutes}
        <small className="ml-2.5 text-lg font-semibold text-hero-muted md:text-[40px]">{ampm}</small>
      </div>
      <div className="font-display text-base font-semibold md:text-3xl">{date}</div>
      <div className="hidden flex-wrap gap-2.5 md:absolute md:bottom-11 md:left-11 md:flex">
        <span className="flex items-center gap-2 rounded-full border border-white/15 bg-white/10 px-3.5 py-2 text-sm">
          <span className="h-2 w-2 rounded-full bg-success shadow-[0_0_0_4px_color-mix(in_srgb,var(--success)_20%,transparent)]" />
          Server online
        </span>
      </div>
      {Game && <Suspense><Game onClose={close} /></Suspense>}
    </div>
  )
}
