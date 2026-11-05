import { ArrowDownToLine, ArrowUpFromLine, KeyRound, Lock, LockOpen, UserRound } from 'lucide-react'
import { useState } from 'react'
import type { HeldSale, TillCopy } from '@/api/types.gen'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'
import type { TillState } from '@/features/till/cart/types'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'
import { CARD } from '../bits'
import { closeNote, drawerOpen, expectedCash } from '../drawer'
import type { Who } from '../session'
import type { Action } from '../store/outbox'
import { refresh, sendNow } from '../sync'
import { DrawerSheet, type DrawerKind } from './DrawerSheet'
import { COUNTER_PAGES, type CounterPage } from './counter/pages'
import { PinSheet } from './PinSheet'
import { TicketsBeforeClose } from './Tickets'

const DONE: Record<DrawerKind, string> = { open: 'Drawer opened', withdraw: 'Cash taken out', deposit: 'Cash put in', close: 'Drawer closed' }

/** The drawer from the phone — open, take out, put in, close — online or from
 * the copy (sent when the PC is back), and switch person. */
export function Menu({ who, copy, out, live, onSent, onSwitch, onPage, tickets = [], cart, onTicket, onTicketsChanged }: {
  who: Who; copy: TillCopy; out: Action[]; live: boolean; onSent: () => void; onSwitch: () => void; onPage: (p: CounterPage) => void
  tickets?: HeldSale[]; cart?: TillState; onTicket?: (t: HeldSale) => void; onTicketsChanged?: () => void
}) {
  const [sheet, setSheet] = useState<DrawerKind | null>(null)
  const [notice, setNotice] = useState('')
  const [pin, setPin] = useState(false)
  const open = drawerOpen(copy, who.id, out)
  const expected = expectedCash(copy, who.id, out)
  const waitingOnMe = tickets.filter((t) => t.forMe)

  async function send(kind: DrawerKind, body: Record<string, unknown>): Promise<string> {
    const r = await sendNow({ kind, userId: who.id, body })
    if (r && r.status !== 'applied') return r.note || 'The PC could not do this'
    const done = r ? DONE[kind] : `${DONE[kind]} on this phone — it sends when the PC is back`
    setNotice(kind === 'close' ? `${done}. ${closeNote(String(body.closingCash), expected, copy.symbol)}` : done)
    onSent()
    return ''
  }

  const item = (kind: DrawerKind, label: string, Icon: typeof Lock) => (
    <button type="button" onClick={() => { setNotice(''); setSheet(kind) }} className={cn(CARD, 'flex items-center gap-3 px-4 py-3.5 text-left text-[15px] font-bold')}>
      <Icon size={19} className="text-muted-foreground" />{label}
    </button>
  )

  return (
    <div className="flex flex-1 flex-col gap-2 overflow-y-auto px-3 pb-3">
      <div className="rounded-[var(--radius)] bg-hero bg-[image:var(--hero-art)] px-4 py-3 text-hero-foreground shadow-md">
        <div className="text-[11.5px] text-hero-muted">{open ? (live ? 'Drawer open · should hold' : 'Drawer open · should hold about') : 'Drawer closed'}</div>
        {open && <div className="font-display text-2xl font-extrabold tabular-nums">{money(copy.symbol, expected)}</div>}
      </div>
      {notice && <p role="status" className="px-1 text-sm font-semibold text-success">{notice}</p>}
      {open ? (
        <>
          {item('withdraw', 'Take cash out', ArrowUpFromLine)}
          {item('deposit', 'Put cash in', ArrowDownToLine)}
          {item('close', 'Close the drawer', Lock)}
        </>
      ) : item('open', 'Open the drawer', LockOpen)}
      {COUNTER_PAGES.map(({ key, label, Icon }) => (
        <button key={key} type="button" onClick={() => onPage(key)} className={cn(CARD, 'flex items-center gap-3 px-4 py-3.5 text-left text-[15px] font-bold')}>
          <Icon size={19} className="text-muted-foreground" />{label}
        </button>
      ))}
      <button type="button" onClick={() => { setNotice(''); setPin(true) }} className={cn(CARD, 'mt-2 flex items-center gap-3 px-4 py-3.5 text-left text-[15px] font-bold')}>
        <KeyRound size={19} className="text-muted-foreground" />Change my PIN
      </button>
      <button type="button" onClick={onSwitch} className={cn(CARD, 'flex items-center gap-3 px-4 py-3.5 text-left text-[15px] font-bold')}>
        <UserRound size={19} className="text-muted-foreground" />Switch person
      </button>
      {pin && <PinSheet live={live} onClose={() => setPin(false)} onDone={() => { setNotice('PIN changed'); void refresh() }} />}
      {sheet === 'close' && waitingOnMe.length > 0 && cart && onTicket && onTicketsChanged ? (
        <Sheet open onOpenChange={(o) => !o && setSheet(null)}>
          <SheetContent side="bottom" className="max-h-[85dvh] gap-3 overflow-y-auto p-4">
            <SheetHeader className="sr-only p-0"><SheetTitle>Tickets on your till</SheetTitle><SheetDescription>Deal with these first</SheetDescription></SheetHeader>
            <TicketsBeforeClose tickets={waitingOnMe} sym={copy.symbol} cart={cart} onOpen={onTicket} onChanged={onTicketsChanged} onClose={() => setSheet(null)} />
          </SheetContent>
        </Sheet>
      ) : sheet && <DrawerSheet kind={sheet} copy={copy} expected={expected} lastCount={copy.people.find((p) => p.id === who.id)?.lastCount} onSend={(b) => send(sheet, b)} onClose={() => setSheet(null)} />}
    </div>
  )
}
