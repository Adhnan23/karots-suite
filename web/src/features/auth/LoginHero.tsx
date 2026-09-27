import { useEffect, useState } from 'react'

function useClock() {
  const [now, setNow] = useState(() => new Date())
  useEffect(() => {
    const id = setInterval(() => setNow(new Date()), 15_000)
    return () => clearInterval(id)
  }, [])
  return now
}

/** Left panel of login/lock (theme.md §5): brand, a live clock and the date.
 * Collapses to a compact header strip on a phone instead of a full column. */
export function LoginHero({ shopName }: { shopName: string }) {
  const now = useClock()
  const h24 = now.getHours()
  const minutes = String(now.getMinutes()).padStart(2, '0')
  const ampm = h24 < 12 ? 'AM' : 'PM'
  const hours = h24 % 12 || 12
  const date = now.toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' })

  return (
    <div className="relative flex flex-col gap-2 overflow-hidden bg-hero bg-[image:var(--hero-art)] p-5 text-hero-foreground md:h-full md:justify-end md:gap-4 md:p-11">
      <div className="flex items-center gap-3 font-display text-base font-bold md:text-lg">
        <span className="grid h-[30px] w-[30px] flex-none place-items-center rounded-[9px] bg-brand font-display text-sm font-extrabold text-brand-foreground shadow-[var(--glow)]">
          K
        </span>
        {shopName}
      </div>
      <div className="font-display text-[64px] leading-none font-extrabold tracking-tight md:text-[132px]">
        {hours}:{minutes}
        <small className="ml-2.5 text-lg font-semibold text-hero-muted md:text-[34px]">{ampm}</small>
      </div>
      <div className="font-display text-base font-semibold md:text-2xl">{date}</div>
      <div className="mt-2 hidden flex-wrap gap-2.5 md:flex">
        <span className="flex items-center gap-2 rounded-full border border-white/15 bg-white/10 px-3.5 py-2 text-sm">
          <span className="h-2 w-2 rounded-full bg-success shadow-[0_0_0_4px_color-mix(in_srgb,var(--success)_20%,transparent)]" />
          Server online
        </span>
      </div>
    </div>
  )
}
