import { useQuery } from '@tanstack/react-query'
import { type ComponentProps, useEffect, useState } from 'react'
import { Field, INPUT } from '@/components/admin/fields'
import { cn } from '@/lib/utils'
import type { ProductFormSection } from '../slots'
import { groupsQuery, productTierQuery } from './api'
import { GroupPicker } from './GroupPicker'
import { hasTiers } from './labels'

type Props = ComponentProps<ProductFormSection['Section']>
const SELECT = cn(INPUT, 'appearance-auto')

/** "Alternatives" on the product form and intake New item: which group (and tier)
 * the product sits in. A duplicate starts in its source's group. Sends a choice
 * only when there is one to make (a change, or a copy of a grouped product). */
export function ProductSection({ productId, sourceId, onChange }: Props) {
  const groups = useQuery(groupsQuery(true)).data
  const cur = useQuery(productTierQuery(productId || sourceId)).data
  const [pick, setPick] = useState<number | null>(null) // tier id; null = untouched
  const tierId = pick ?? cur?.tierId ?? 0
  const send = pick !== null || (productId === 0 && !!cur?.tierId)

  useEffect(() => {
    onChange(send ? { tierId } : undefined)
    // oxlint-disable-next-line react-hooks/exhaustive-deps -- onChange is a fresh arrow each render
  }, [send, tierId])

  if (!groups) return null
  const group = groups.find((g) => g.tiers.some((t) => t.id === tierId))
  return (
    <fieldset className="grid gap-3 rounded-[var(--radius-sm)] border border-border p-3 md:grid-cols-2">
      <legend className="px-1 text-[12px] font-bold uppercase tracking-wide text-faint">Alternatives</legend>
      <Field label="Group" hint="Products that do the same job: the till offers them when this runs out.">
        <GroupPicker groups={groups} value={group?.id ?? 0} onPick={setPick} />
      </Field>
      {group && hasTiers(group) && (
        <Field label="Tier">
          <select aria-label="Alternatives tier" value={tierId} onChange={(e) => setPick(Number(e.target.value))} className={SELECT}>
            {group.tiers.map((t) => <option key={t.id} value={t.id}>{t.name || '(no name)'}{t.active ? '' : ' (off)'}</option>)}
          </select>
        </Field>
      )}
    </fieldset>
  )
}
