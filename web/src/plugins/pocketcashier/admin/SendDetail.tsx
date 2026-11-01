import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { PageHead, SURFACE } from '@/components/admin/PageHead'
import { Tag } from '@/features/admin/products/ProductsPage'
import { cn } from '@/lib/utils'
import { sendQuery, when } from './api'

const KIND: Record<string, string> = { sale: 'Sale', open: 'Drawer opened', close: 'Drawer closed', withdraw: 'Cash taken out', deposit: 'Cash put in', hold: 'Sale held', unhold: 'Held sale taken back' }

/** One send: each thing in it, what became of it, and its flags. A flagged sale
 * opens in Sales, where a look-alike is undone with the admin return. */
export function SendDetail({ id }: { id: number }) {
  const s = useQuery(sendQuery(id)).data
  if (!s) return null
  return (
    <>
      <PageHead title={s.phone} sub={`Sent ${when(s.receivedAt)}`}
        actions={<Link to="/admin/x/$key/$" params={{ key: 'pocketcashier', _splat: 'sends' }} className="text-[13px] font-semibold text-muted-foreground hover:text-foreground">← What came in</Link>} />
      <ul aria-label="Actions" className="flex flex-col gap-2">
        {s.actions.map((a) => (
          <li key={a.ref} className={cn(SURFACE, 'flex min-w-0 flex-col gap-1.5 p-3.5')}>
            <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1">
              <b>{KIND[a.kind] ?? a.kind}</b>
              {a.saleId > 0 && (
                <Link to="/admin/sales" search={{ sale: a.saleId }} className="font-semibold text-brand underline-offset-2 hover:underline">{a.receiptNo || `#${a.saleId}`}</Link>
              )}
              <span className="text-[13px] text-muted-foreground">{a.userName} · {when(a.at)}</span>
              <span className="ml-auto">
                {a.status === 'failed' && <Tag tone="danger">Failed</Tag>}
                {a.status === 'skipped' && <Tag tone="muted">Already done</Tag>}
              </span>
            </div>
            {a.note && <p className={cn('text-[13px]', a.status === 'failed' ? 'font-semibold text-danger' : 'text-muted-foreground')}>{a.note}</p>}
            {a.flags.map((f, i) => (
              <p key={i} className="flex min-w-0 items-start gap-2 text-[13px]"><Tag tone="warn">{f.kind.replace('_', ' ')}</Tag><span className="min-w-0 break-words">{f.note}</span></p>
            ))}
          </li>
        ))}
      </ul>
    </>
  )
}
