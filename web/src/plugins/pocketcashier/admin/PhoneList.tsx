import { Smartphone } from 'lucide-react'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { cn } from '@/lib/utils'
import { ago } from '../bits'
import type { PhoneRow } from './types'

// The server's unreachableAfter (guard.go): a phone silent this long may be selling offline.
const REACHABLE_FOR = 60_000

/** Every paired phone, the waiting ones first: who is on it, whether it answers,
 * how fresh its copy is, and what an admin can do (a blocked phone is only removed;
 * pairing again makes a new row). */
export function PhoneList({ rows, now, onApprove, onBlock, onRelease, onRemove }: {
  rows: PhoneRow[]
  now: number
  onApprove: (p: PhoneRow) => void
  onBlock: (p: PhoneRow) => Promise<unknown> | void
  onRelease: (p: PhoneRow) => Promise<unknown> | void
  onRemove: (p: PhoneRow) => Promise<unknown> | void
}) {
  return (
    <ul aria-label="Phones" className="flex flex-col gap-2.5">
      {rows.map((p) => {
        const reachable = !!p.lastSeen && now - Date.parse(p.lastSeen) < REACHABLE_FOR
        return (
          <li key={p.id} className={cn(SURFACE, 'flex min-w-0 flex-col gap-2 p-4')}>
            <header className="flex items-center gap-2.5">
              <Smartphone size={18} className="shrink-0 text-muted-foreground" />
              <h2 className="min-w-0 truncate font-display text-[16px] font-bold">{p.name}</h2>
              <span className="ml-auto shrink-0">
                {p.status === 'pending' && <Tag tone="warn">Waiting</Tag>}
                {p.status === 'blocked' && <Tag tone="danger">Blocked</Tag>}
                {p.status === 'active' && <Tag tone={reachable ? 'brand' : 'muted'}>{reachable ? 'Live' : 'Away'}</Tag>}
              </span>
            </header>
            {p.status === 'pending' ? (
              <p className="text-[13.5px] text-muted-foreground">Shows the code <b className="font-mono text-[15px] tracking-widest text-foreground">{p.pairCode}</b></p>
            ) : p.status === 'active' && (
              <p className="text-[13.5px] text-muted-foreground">
                {p.userName ? <b className="text-foreground">{`${p.userName} · ${reachable ? 'on now' : 'not reachable'}`}</b> : 'Nobody signed in'}
                <span className="block text-[12px]">Copy taken {ago(p.copyAt || null, now)} · last heard {ago(p.lastSeen || null, now)}</span>
              </p>
            )}
            <div className="flex flex-wrap justify-end gap-2">
              {p.status === 'pending' && <Button type="button" className="h-10 px-4 font-bold" onClick={() => onApprove(p)}>Approve</Button>}
              {p.status === 'active' && p.userName && (
                <ConfirmButton plain label="Release" title={`Release ${p.userName}?`} confirm="Release"
                  body={`${p.userName} can then sign in on another device. Do it when ${p.name} is lost or dead — sales still on it are sent when it comes back.`}
                  onConfirm={() => onRelease(p)} />
              )}
              {p.status === 'active' && (
                <ConfirmButton label="Block" title={`Block ${p.name}?`} confirm="Block"
                  body="It stops working at once. To use it again, remove it and pair it again." onConfirm={() => onBlock(p)} />
              )}
              <ConfirmButton label="Remove" title={`Remove ${p.name}?`} confirm="Remove" onConfirm={() => onRemove(p)}
                body={p.status === 'active'
                  ? `It stops working at once. Its batches leave What came in (its sales stay in Sales), and sales still waiting on it never arrive — block it instead if it may come back. To use it again, pair it again.`
                  : 'It disappears from this list. Pairing it again starts over.'} />
            </div>
          </li>
        )
      })}
    </ul>
  )
}
