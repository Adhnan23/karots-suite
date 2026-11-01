import { LockOpen } from 'lucide-react'
import { useState } from 'react'
import type { TillCopy } from '@/api/types.gen'
import { cn } from '@/lib/utils'
import { CARD } from '../bits'
import type { Who } from '../session'
import { sendNow } from '../sync'
import { DrawerSheet } from './DrawerSheet'

/** Sell before the drawer is open: the PC till's "Open the till" — count the
 * float first (live, or from the copy and sent when the PC is back). */
export function OpenFirst({ who, copy, onSent }: { who: Who; copy: TillCopy; onSent: () => void }) {
  const [counting, setCounting] = useState(false)
  async function send(body: Record<string, unknown>): Promise<string> {
    const r = await sendNow({ kind: 'open', userId: who.id, body })
    if (r && r.status !== 'applied') return r.note || 'The PC could not open the drawer'
    onSent()
    return ''
  }
  return (
    <div className="flex flex-1 flex-col justify-center gap-3 px-6">
      <div className={cn(CARD, 'flex flex-col items-center gap-3 rounded-[var(--radius)] px-5 py-6 text-center')}>
        <LockOpen size={28} className="text-muted-foreground" />
        <p className="text-[15px] font-bold">Open the drawer to start selling</p>
        <p className="text-[13px] text-muted-foreground">Count the cash in the drawer, as at the till. Sales go into {who.name}’s drawer.</p>
        <button type="button" onClick={() => setCounting(true)} className="rounded-[var(--radius)] bg-brand px-5 py-3 text-base font-extrabold text-brand-foreground">Count the float</button>
      </div>
      {counting && (
        <DrawerSheet kind="open" copy={copy} expected="0" lastCount={copy.people.find((p) => p.id === who.id)?.lastCount}
          onSend={send} onClose={() => setCounting(false)} />
      )}
    </div>
  )
}
