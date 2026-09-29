import { queryOptions, useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { ExpenseRequest, ExpensesPage, ReceiptRef } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, drawerQuery, errMsg } from '@/features/till/api'
import { FIELD } from '@/features/till/ui'
import { afterMove } from '../slip'
import { CounterPage, PaneTitle, SourcePicker } from '../ui'

export const expensesQuery = queryOptions({ queryKey: ['till', 'expenses'], queryFn: () => api<ExpensesPage>('/till/expenses') })

const LABEL = 'flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground'
const time = (iso: string) => new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })

/** Pay a small shop cost out of the drawer (or a locker) and log it as an expense. */
export function ExpensesTab() {
  const cfg = useSuspenseQuery(configQuery).data
  const page = useSuspenseQuery(expensesQuery).data
  const lockers = (useQuery(drawerQuery).data?.lockers ?? []).filter((l) => l.kind !== 'card' && l.kind !== 'bank')
  const qc = useQueryClient()
  const [category, setCategory] = useState('')
  const [amount, setAmount] = useState('')
  const [note, setNote] = useState('')
  const [source, setSource] = useState('')
  const [service, setService] = useState('0')
  const [busy, setBusy] = useState(false)
  const sym = cfg.symbol
  const total = page.today.reduce((s, e) => s + num(e.amount), 0)

  async function submit(e: { preventDefault(): void }) {
    e.preventDefault()
    if (!category.trim()) return void toast.error('Pick or type a category')
    if (!(num(amount) > 0)) return void toast.error('Enter the amount paid')
    setBusy(true)
    try {
      const body: ExpenseRequest = {
        category: category.trim(), amount, description: note, serviceProductId: num(service),
        lockerId: num(source), deferDrawerKick: !cfg.askToPrint,
      }
      const res = await api<ReceiptRef>('/till/expenses', { json: body })
      afterMove(cfg, `Expense recorded — ${money(sym, amount)}`, res.receiptId, source === '')
      setAmount('')
      setNote('')
      setService('0')
      void qc.invalidateQueries({ queryKey: ['till'] })
    } catch (err) {
      toast.error(errMsg(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <CounterPage
      aside={
        <>
          <PaneTitle title="My expenses today" right={<b className="font-display tabular-nums">{money(sym, total)}</b>} />
          <ul className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-2.5 pb-2.5">
            {page.today.length === 0 && <li className="m-auto p-8 text-center text-faint">Nothing paid out today.</li>}
            {page.today.map((x) => (
              <li key={x.id} className="flex items-center gap-3 rounded-[var(--radius-sm)] bg-muted px-3 py-2">
                <span className="min-w-0 flex-1">
                  <b className="block truncate text-[14px]">{x.category}</b>
                  <small className="block truncate text-muted-foreground">
                    {time(x.createdAt)}{x.description && ` · ${x.description}`}{x.serviceName && ` · for ${x.serviceName}`}
                  </small>
                </span>
                <b className="tabular-nums">{money(sym, x.amount)}</b>
              </li>
            ))}
          </ul>
        </>
      }
    >
      <form onSubmit={submit} autoComplete="off" className="flex flex-col gap-5">
        <h2 className="font-display text-xl font-bold">Pay an expense</h2>
        <div className={LABEL}>
          What for?
          <div className="flex flex-wrap gap-1.5">
            {page.categories.slice(0, 10).map((c) => (
              <button key={c} type="button" aria-pressed={category === c} onClick={() => setCategory(c)}
                className="rounded-full border border-border px-3 py-1.5 text-[13px] font-semibold text-foreground aria-pressed:border-brand aria-pressed:bg-brand aria-pressed:text-brand-foreground">
                {c}
              </button>
            ))}
          </div>
          <input aria-label="Category" list="expense-categories" placeholder="…or type a category" value={category}
            onChange={(e) => setCategory(e.target.value)} className={FIELD} />
          <datalist id="expense-categories">{page.categories.map((c) => <option key={c} value={c} />)}</datalist>
        </div>
        <label className={LABEL}>
          Amount
          <input aria-label="Amount" inputMode="decimal" placeholder={money(sym, 0)} value={amount}
            onChange={(e) => setAmount(e.target.value.replace(/[^\d.]/g, ''))}
            className={cn(FIELD, 'h-14 font-display text-2xl font-bold tabular-nums')} />
        </label>
        <label className={LABEL}>
          Note <span className="sr-only">(optional)</span>
          <input aria-label="Note" placeholder="Optional — who, what" value={note} onChange={(e) => setNote(e.target.value)} className={FIELD} />
        </label>
        <div className={LABEL}>
          Paid from
          <SourcePicker value={source} onChange={setSource} lockers={lockers} sym={sym} />
        </div>
        {page.services.length > 0 && (
          <label className={LABEL}>
            For a service
            <select aria-label="For a service" value={service} onChange={(e) => setService(e.target.value)} className={cn(FIELD, 'appearance-auto')}>
              <option value="0">Not for a service</option>
              {page.services.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
            <small className="font-normal">Tags the cost to that service, e.g. a repair for the photocopier.</small>
          </label>
        )}
        <Button type="submit" size="lg" disabled={busy} className="h-12 text-[15px]">
          Record expense{num(amount) > 0 && ` · ${money(sym, amount)}`}
        </Button>
      </form>
    </CounterPage>
  )
}
