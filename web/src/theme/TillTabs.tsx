import { Link, useRouterState } from '@tanstack/react-router'
import { ChevronDown } from 'lucide-react'
import type { TillConfig } from '@/api/types.gen'
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'

type Tab = { to: string; label: string; show?: (c: TillConfig) => boolean }

// Primary tabs sit in the bar; the rest live under More (the bar must fit at 1024).
const PRIMARY: Tab[] = [
  { to: '/till', label: 'Sell' },
  { to: '/till/returns', label: 'Returns' },
]
const MORE: Tab[] = [
  { to: '/till/write-off', label: 'Write-off' },
  { to: '/till/convert', label: 'Convert' },
  { to: '/till/labels', label: 'Labels' },
  { to: '/till/expenses', label: 'Expenses', show: (c) => c.canHandleSuppliers },
]

const TAB = 'whitespace-nowrap rounded-full px-3 py-1.5 text-muted-foreground hover:bg-muted'
const ON = 'bg-selected text-selected-foreground hover:bg-selected'

export function TillTabs({ config }: { config: TillConfig }) {
  const path = useRouterState({ select: (s) => s.location.pathname })
  const isOn = (to: string) => (to === '/till' ? path === '/till' || path === '/till/' : path.startsWith(to))
  const more = [
    ...MORE.filter((t) => !t.show || t.show(config)),
    ...slots.tillTabs.map((t) => ({ to: `/till/x/${t.key}`, label: t.label })),
  ]
  const activeMore = more.find((t) => isOn(t.to))
  return (
    <nav aria-label="Till sections" className="flex items-center gap-1 whitespace-nowrap text-[13px] font-semibold">
      {PRIMARY.filter((t) => !t.show || t.show(config)).map((t) => (
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
  )
}
