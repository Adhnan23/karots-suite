import { toast } from 'sonner'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { DataList } from '@/components/admin/DataList'
import { EmptyState } from '@/components/admin/PageHead'
import { disable } from './api'
import type { ClearancePageData } from './types'

/** Out of stock and idle for months: retire them (Products → Show disabled brings one back). */
export function DeadList({ data }: { data?: ClearancePageData }) {
  const rows = data?.dead ?? []
  const months = data?.settings.deadMonths ?? 6
  const off = async (ids: number[]) => {
    const r = await disable(ids)
    toast.success(r.disabled === 1 ? 'Product disabled' : `${r.disabled} products disabled`)
  }
  return (
    <DataList label="Dead stock" rows={rows} rowKey={(r) => r.productId}
      empty={<EmptyState title={!data ? 'Loading…' : 'No dead stock'}>
        {data && `Products show here when they've had no stock and nothing sold or received for ${months} months.`}
      </EmptyState>}
      footer={rows.length > 1 && (
        <div className="flex justify-end border-t border-border px-4 py-3">
          <ConfirmButton label={`Disable all shown (${rows.length})`} title={`Disable ${rows.length} products?`}
            body={`They leave the till and the lists. You can bring each back from Products → Show disabled.`}
            confirm={`Disable ${rows.length}`} onConfirm={() => off(rows.map((r) => r.productId))} />
        </div>
      )}
      columns={[
        { key: 'name', header: 'Product', cell: (r) => <span><b>{r.name}</b> <small className="text-faint">{r.unit}</small></span> },
        { key: 'idle', header: 'Idle', num: true, cell: (r) => <span className="whitespace-nowrap">{r.monthsIdle <= 1 ? '1 month' : `${r.monthsIdle} months`}</span> },
        { key: 'last', header: 'Last activity', num: true, cell: (r) => <span className="whitespace-nowrap text-muted-foreground">{r.lastActivity}</span> },
        { key: 'act', header: '', interactive: true, className: 'w-[1%]', cell: (r) => (
          <ConfirmButton label="Disable" className="h-9 px-3" title={`Disable ${r.name}?`}
            body="It leaves the till and the lists. Products → Show disabled brings it back." confirm="Disable" onConfirm={() => off([r.productId])} />
        ) },
      ]} />
  )
}
