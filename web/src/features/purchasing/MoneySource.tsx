import { queryOptions, useQuery } from '@tanstack/react-query'
import { api } from '@/api/client'
import type { MoneySource } from '@/api/types.gen'
import { INPUT } from '@/components/admin/fields'
import { money } from '@/lib/money'
import { cn } from '@/lib/utils'

export type Method = 'cash' | 'card' | 'online'

export const moneySourcesQuery = queryOptions({ queryKey: ['admin', 'money-sources'], queryFn: () => api<MoneySource[]>('/admin/money-sources') })

/** Where a method's money can sit: cash in a safe / pocket / till, card in a
 * card account, online in a bank account (karots-pos LocationPicker). */
export const sourcesFor = (all: MoneySource[], m: Method) =>
  all.filter((s) => (m === 'cash' ? s.kind !== 'card' && s.kind !== 'bank' : s.kind === (m === 'card' ? 'card' : 'bank')))

/** Admin money source/destination. Cash must name one; card and online may be
 * outside the shop (""). */
export function MoneySourcePicker({ method, value, onChange, sym, into, label }: {
  method: Method; value: string; onChange: (v: string) => void; sym: string; into?: boolean; label?: string
}) {
  const opts = sourcesFor(useQuery(moneySourcesQuery).data ?? [], method)
  const groups = [...new Set(opts.map((o) => o.group))]
  return (
    <select aria-label={label ?? (into ? 'Into' : 'From')} value={value} onChange={(e) => onChange(e.target.value)} className={cn(INPUT, 'appearance-auto')}>
      <option value="">{method === 'cash' ? `— ${into ? 'where it goes' : 'where it comes from'} —` : into ? 'Not into a shop account' : 'Not from a shop account'}</option>
      {groups.map((g) => (
        <optgroup key={g} label={g}>
          {opts.filter((o) => o.group === g).map((o) => (
            <option key={o.value} value={o.value}>{o.label}{o.group === 'Lockers' ? ` (${money(sym, o.balance)})` : ''}</option>
          ))}
        </optgroup>
      ))}
    </select>
  )
}
