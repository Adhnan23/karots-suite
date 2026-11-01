import { Banknote, BookUser, CreditCard, ScrollText, Wallet } from 'lucide-react'
import type { TillCopy } from '@/api/types.gen'
import { newChequeDraft } from '@/features/till/cart/store'
import { quickCash } from '@/features/till/cart/totals'
import { ChequeFields } from '@/features/till/pay/ChequeFields'
import { FIELD } from '@/features/till/ui'
import { money, num, r2 } from '@/lib/money'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { GLASS } from '../bits'
import type { Tender } from '../sale'

/** Cash / Card / Cheque / Credit (+ plugin tenders: live only, unless they work
 * from the copy), then what the chosen one needs: cash handed over and the change. */
export function PayTenders({ copy, total, live, t, set }: { copy: TillCopy; total: number; live: boolean; t: Tender; set: (t: Tender) => void }) {
  const cards = copy.places.filter((p) => p.kind === 'card' && p.cashierAccess)
  const methods = [
    { value: 'cash', label: 'Cash', Icon: Banknote },
    ...(cards.length ? [{ value: 'card', label: 'Card', Icon: CreditCard }] : []),
    { value: 'cheque', label: 'Cheque', Icon: ScrollText },
    { value: 'credit', label: 'Credit', Icon: BookUser },
    ...copy.tenderMethods.filter((m) => !m.hidden).flatMap((m) => {
      const slot = slots.tillTenders.find((x) => x.value === m.value)
      return live || slot?.offline ? [{ value: m.value, label: m.label, Icon: slot?.Icon ?? Wallet }] : []
    }),
  ]
  const choose = (m: string) => set({ method: m, given: '', meta: {}, accountId: m === 'card' ? String(cards[0]?.id ?? '') : '', cheque: m === 'cheque' ? (t.cheque ?? newChequeDraft()) : undefined })
  const Detail = slots.tillTenders.find((x) => x.value === t.method)?.Detail
  const change = t.method === 'cash' && t.given !== '' ? r2(num(t.given) - total) : 0

  return (
    <>
      <div role="group" aria-label="Payment method" className="grid grid-cols-4 gap-1.5 px-3">
        {methods.map(({ value, label, Icon }) => (
          <button key={value} type="button" aria-pressed={t.method === value} onClick={() => choose(value)}
            className={cn('flex flex-col items-center gap-0.5 rounded-[var(--radius-sm)] border border-border bg-card px-0.5 py-2 text-[11.5px] font-bold aria-pressed:border-transparent aria-pressed:bg-selected aria-pressed:text-selected-foreground', GLASS)}>
            <Icon size={18} />{label}
          </button>
        ))}
      </div>
      <div className="mx-3 mt-2 flex flex-col gap-2">
        {t.method === 'cash' && (
          <div role="group" aria-label="Cash given" className="flex flex-wrap gap-1.5">
            {quickCash(total).slice(0, 4).map((v, i) => {
              const val = i === 0 ? '' : String(v)
              return (
                <button key={v} type="button" aria-pressed={t.given === val} onClick={() => set({ ...t, given: val })}
                  className="rounded-pill bg-muted px-2.5 py-1.5 text-xs font-bold tabular-nums aria-pressed:bg-brand-soft aria-pressed:outline-2 aria-pressed:outline-brand">
                  {i === 0 ? 'Exact' : money('', v).trim()}
                </button>
              )
            })}
            <input aria-label="Other amount" placeholder="Other…" inputMode="decimal" autoComplete="off" value={t.given}
              onChange={(e) => set({ ...t, given: e.target.value.replace(/[^\d.]/g, '') })} className={cn(FIELD, 'h-[30px] w-24 rounded-pill text-xs')} />
          </div>
        )}
        {t.method === 'card' && cards.length > 1 && (
          <select aria-label="Account" value={t.accountId} onChange={(e) => set({ ...t, accountId: e.target.value })} className={cn(FIELD, 'appearance-auto')}>
            {cards.map((c) => <option key={c.id} value={c.id}>{c.name}</option>)}
          </select>
        )}
        {t.method === 'cheque' && t.cheque && <ChequeFields value={t.cheque} onChange={(cheque) => set({ ...t, cheque })} />}
        {Detail && <Detail meta={t.meta} setMeta={(meta) => set({ ...t, meta })} />}
        {change > 0.005 && (
          <div role="status" className="flex justify-between rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--success)_14%,transparent)] px-3 py-2 font-extrabold text-success">
            <span>Give change</span><span className="tabular-nums">{money(copy.symbol, change)}</span>
          </div>
        )}
      </div>
    </>
  )
}
