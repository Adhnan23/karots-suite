import { Link, useRouterState } from '@tanstack/react-router'
import { ChevronDown } from 'lucide-react'
import { useLayoutEffect, useRef, useState } from 'react'
import type { TillConfig } from '@/api/types.gen'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'
import { tillTabList } from './tillTabList'

const TAB = 'whitespace-nowrap rounded-full px-3 py-1.5 text-muted-foreground hover:bg-muted'
const ON = 'bg-selected text-selected-foreground hover:bg-selected'
const GAP = 4 // gap-1

/** The till's section tabs as a dynamic row: as many as fit the bar stay out
 * (in order, Sell always), the rest go under More. A hidden copy of every tab
 * is measured, and the split is redone whenever the room changes. */
export function TillTabs({ config }: { config: TillConfig }) {
  const path = useRouterState({ select: (s) => s.location.pathname })
  const isOn = (to: string) => (to === '/till' ? path === '/till' || path === '/till/' : path.startsWith(to))
  const tabs = tillTabList(config)
  const box = useRef<HTMLDivElement>(null)
  const meas = useRef<HTMLDivElement>(null)
  const [fit, setFit] = useState(tabs.length)

  useLayoutEffect(() => {
    const el = box.current
    const m = meas.current
    if (!el || !m) return
    const calc = () => {
      const kids = [...m.children] as HTMLElement[]
      const more = kids.pop()!.offsetWidth + GAP // the widest "More" the button can show
      const widths = kids.map((k) => k.offsetWidth + GAP)
      const room = el.clientWidth
      if (widths.reduce((a, b) => a + b, 0) <= room) return setFit(widths.length)
      let used = more
      let n = 0
      for (const w of widths) {
        if (used + w > room) break
        used += w
        n++
      }
      setFit(Math.max(1, n))
    }
    calc()
    const ro = new ResizeObserver(calc)
    ro.observe(el)
    return () => ro.disconnect()
  }, [tabs.length])

  const shown = tabs.slice(0, fit)
  const more = tabs.slice(fit)
  const activeMore = more.find((t) => isOn(t.to))
  const longest = more.length ? more.reduce((a, t) => (t.label.length > a.length ? t.label : a), 'More') : 'More'
  return (
    <div ref={box} className="relative min-w-0 flex-1 overflow-hidden">
      <nav aria-label="Till sections" className="flex items-center gap-1 whitespace-nowrap text-[13px] font-semibold">
        {shown.map((t) => (
          // Plain string paths: tabs are data, and each target route exists.
          <Link key={t.to} to={t.to as '/till'} className={cn(TAB, isOn(t.to) && ON)}>{t.label}</Link>
        ))}
        {more.length > 0 && (
          <DropdownMenu>
            <DropdownMenuTrigger className={cn(TAB, 'flex items-center gap-1', activeMore && ON)}>
              {activeMore?.label ?? 'More'} <ChevronDown size={14} />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              {more.map((t) => (
                <DropdownMenuItem key={t.to} asChild>
                  <Link to={t.to as '/till'}>{t.label}</Link>
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </nav>
      {/* Measuring copy: every tab, then the widest More button. Never seen. */}
      <div ref={meas} aria-hidden className="invisible absolute left-0 top-0 flex gap-1 whitespace-nowrap text-[13px] font-semibold">
        {tabs.map((t) => <span key={t.to} className={TAB}>{t.label}</span>)}
        <span className={cn(TAB, 'flex items-center gap-1')}>{longest} <ChevronDown size={14} /></span>
      </div>
    </div>
  )
}
