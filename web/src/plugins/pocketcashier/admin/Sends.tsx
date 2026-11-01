import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Tag } from '@/features/admin/products/ProductsPage'
import { cn } from '@/lib/utils'
import { sendsQuery, when } from './api'

/** Every batch the phones sent, newest first; the ones with failed or flagged work stand out. */
export function Sends() {
  const rows = useQuery(sendsQuery).data
  return (
    <>
      <PageHead title="What came in" sub="Work the phones sent, newest first. Open one to see each sale and anything to look at." />
      {rows && !rows.length && <EmptyState title="Nothing yet">Sales and drawer moves from phones show here.</EmptyState>}
      <ul aria-label="Sends" className="flex flex-col gap-2">
        {rows?.map((s) => (
          <li key={s.id}>
            <Link to="/admin/x/$key/$" params={{ key: 'pocketcashier', _splat: `sends/${s.id}` }}
              className={cn(SURFACE, 'flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 p-3.5 transition-colors hover:border-brand')}>
              <b className="min-w-0 truncate">{s.phone}</b>
              <span className="text-[13px] text-muted-foreground">{when(s.receivedAt)} · {s.actions} {s.actions === 1 ? 'thing' : 'things'}</span>
              <span className="ml-auto flex gap-1.5">
                {s.failed > 0 && <Tag tone="danger">{`${s.failed} failed`}</Tag>}
                {s.flagged > 0 && <Tag tone="warn">{`${s.flagged} to look at`}</Tag>}
              </span>
            </Link>
          </li>
        ))}
      </ul>
    </>
  )
}
