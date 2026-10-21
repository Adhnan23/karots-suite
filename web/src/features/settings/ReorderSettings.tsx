import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ReorderAutoRequest, ReorderAutoResult, ShopSettings } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Field, INPUT } from '@/components/admin/fields'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { Section } from './kit'

const int = (s: string) => Math.max(0, Math.trunc(Number(s) || 0))

/** Settings → Auto reorder levels: the fallbacks the nightly run uses, and switching older (Fixed) products to Auto. */
export function ReorderSettings({ saved }: { saved: ShopSettings }) {
  const qc = useQueryClient()
  const [cat, setCat] = useState(0)
  async function switchAuto() {
    const body: ReorderAutoRequest = { categoryId: cat }
    const r = await api<ReorderAutoResult>('/admin/products/reorder-auto', { json: body })
    toast.success(r.switched ? `${r.switched} product${r.switched === 1 ? '' : 's'} switched to Auto` : 'Every product there is Auto already')
    void qc.invalidateQueries({ queryKey: ['admin'] })
  }
  return (
    <Section title="Auto reorder levels" sub="Learned every night from sales and deliveries. These numbers are used until a product has its own history." saved={saved} keys={['reorderDefaultDays', 'reorderSafetyDays']}>
      {({ v, set }) => (
        <>
          <div className="grid gap-3 sm:grid-cols-2">
            <Field label="Supplier comes every … days when we don't know">
              <input className={INPUT} inputMode="numeric" aria-label="Default supplier days" value={v.reorderDefaultDays} onChange={(e) => set('reorderDefaultDays', Math.min(90, int(e.target.value)))} />
            </Field>
            <Field label="Safety days" hint="Extra days of stock in case a delivery is late.">
              <input className={INPUT} inputMode="numeric" aria-label="Safety days" value={v.reorderSafetyDays} onChange={(e) => set('reorderSafetyDays', Math.min(60, int(e.target.value)))} />
            </Field>
          </div>
          <div className="flex flex-col gap-2 border-t border-border pt-3">
            <b className="text-sm">Switch to Auto</b>
            <small className="text-[13px] text-muted-foreground">New products are Auto already. Older ones keep the number you typed until you switch them; each starts from that number and adjusts over the next weeks.</small>
            <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
              <CategoryPicker label="Category to switch" noneLabel="All products" value={cat} onChange={setCat} />
              <ConfirmButton plain className="h-10 bg-brand px-4 font-bold text-brand-foreground hover:bg-brand/90" label="Switch Fixed products to Auto" title="Switch to Auto?"
                body={cat ? 'Every Fixed product in this category and its sub-categories will learn its reorder level from now on.' : 'Every Fixed product in the shop will learn its reorder level from now on.'}
                confirm="Switch to Auto" onConfirm={switchAuto} />
            </div>
          </div>
        </>
      )}
    </Section>
  )
}
