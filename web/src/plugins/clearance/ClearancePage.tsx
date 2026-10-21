import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { SlidersHorizontal } from 'lucide-react'
import { useState } from 'react'
import { PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { configQuery } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { clearanceQuery } from './api'
import { DeadList } from './DeadList'
import { MarkdownList } from './MarkdownList'
import { RulesSheet } from './RulesSheet'

export const CHIP = 'rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground whitespace-nowrap aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

/** Apps → Clearance: mark down slow and expiring stock, retire dead stock.
 * The tab is the sub-path: /admin/x/clearance (to mark down) and …/dead. */
export function ClearancePage({ path }: { path: string }) {
  const tab = path === 'dead' ? 'dead' : 'markdown'
  const go = useNavigate()
  const sym = useSuspenseQuery(configQuery).data.symbol
  const data = useQuery(clearanceQuery).data
  const [rules, setRules] = useState(false)
  const live = data?.rows.filter((r) => r.status === 'live').length ?? 0
  const todo = data?.rows.filter((r) => r.status === '').length ?? 0

  return (
    <>
      <PageHead title="Clearance"
        sub={data ? `${todo} to mark down · ${live} live · ${data.dead.length} dead` : 'Move slow and expiring stock before it becomes a loss.'}
        actions={<Button variant="outline" className="h-10 gap-1.5 px-4 font-bold" disabled={!data} onClick={() => setRules(true)}><SlidersHorizontal size={16} /> Rules</Button>} />
      <div className="mb-3 flex gap-1.5 overflow-x-auto">
        <button type="button" aria-pressed={tab === 'markdown'} className={CHIP} onClick={() => void go({ to: '/admin/x/$key/$', params: { key: 'clearance', _splat: '' } })}>To mark down</button>
        <button type="button" aria-pressed={tab === 'dead'} className={cn(CHIP)} onClick={() => void go({ to: '/admin/x/$key/$', params: { key: 'clearance', _splat: 'dead' } })}>
          Dead stock{data && data.dead.length > 0 ? ` · ${data.dead.length}` : ''}
        </button>
      </div>
      {tab === 'markdown'
        ? <MarkdownList sym={sym} data={data} />
        : <DeadList data={data} />}
      {data && <RulesSheet open={rules} onOpenChange={setRules} settings={data.settings} />}
    </>
  )
}
