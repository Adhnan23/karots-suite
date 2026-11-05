import { ChevronLeft } from 'lucide-react'
import type { ReactNode } from 'react'
import type { TillCopy } from '@/api/types.gen'
import type { Who } from '../../session'
import type { Action } from '../../store/outbox'

/** What every counter page on the phone gets. */
export type CounterProps = { who: Who; copy: TillCopy; out: Action[]; live: boolean; onSent: () => void; onBack: () => void }

export function CounterShell({ title, sub, onBack, children }: { title: string; sub?: string; onBack: () => void; children: ReactNode }) {
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex items-center gap-2 px-3 pb-2">
        <button type="button" aria-label="Back" onClick={onBack} className="grid h-11 w-11 place-items-center rounded-full bg-muted"><ChevronLeft size={20} /></button>
        <div className="min-w-0">
          <h2 className="truncate font-display text-lg font-bold">{title}</h2>
          {sub && <p className="truncate text-[12.5px] text-muted-foreground">{sub}</p>}
        </div>
      </div>
      <div className="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto px-3 pb-3">{children}</div>
    </div>
  )
}
