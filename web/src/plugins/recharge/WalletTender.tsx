import { useQuery } from '@tanstack/react-query'
import { useEffect } from 'react'
import { FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import { tillDataQuery } from './api'

/** The wallet tender's detail: which device the customer's eZ Cash / mCash came to. */
export function WalletTender({ meta, setMeta }: { meta: Record<string, string>; setMeta: (m: Record<string, string>) => void }) {
  const floats = (useQuery(tillDataQuery).data?.floats ?? []).filter((f) => f.money)
  const first = floats[0]?.id
  useEffect(() => {
    if (!meta.device && first) setMeta({ ...meta, device: String(first) })
  }, [meta, first, setMeta])
  return (
    <div className="flex flex-wrap gap-1.5">
      <select aria-label="Wallet money came to" value={meta.device ?? ''} onChange={(e) => setMeta({ ...meta, device: e.target.value })}
        className={cn(FIELD, 'h-9 min-w-0 flex-1 appearance-auto text-[13px]')}>
        {floats.map((f) => <option key={f.id} value={f.id}>{f.carrier} · {f.label}</option>)}
      </select>
      <input aria-label="Wallet reference" placeholder="Reference" autoComplete="off" value={meta.ref ?? ''} onChange={(e) => setMeta({ ...meta, ref: e.target.value })}
        className={cn(FIELD, 'h-9 w-32 text-[13px]')} />
    </div>
  )
}
