import { useQuery } from '@tanstack/react-query'
import { Plus, X } from 'lucide-react'
import { useState } from 'react'
import type { ChequeLine } from '@/api/types.gen'
import { chequeBanksQuery, chequeSeen } from '@/features/money/cheques/queries'
import { money, num, r2 } from '@/lib/money'
import { cn } from '@/lib/utils'
import { INPUT } from './fields'

/** One cheque as a form edits it. Received: Bank is the payer's bank. Given:
 *  LockerID is the shop's bank place it's drawn on. */
export type ChequeDraftLine = { bank: string; lockerId: string; number: string; amount: string; date: string; note: string }

/** A new cheque line: its date is left for the person to type (a date filled
 *  in for them gets left as it is). */
export const blankCheque = (amount = '', lockerId = ''): ChequeDraftLine => ({ bank: '', lockerId, number: '', amount, date: '', note: '' })
export const chequesTotal = (ls: ChequeDraftLine[]) => r2(ls.reduce((t, l) => t + num(l.amount), 0))
/** What a line still lacks ('' = ready). */
export const chequeIssue = (l: ChequeDraftLine, dir: 'in' | 'out') =>
  !l.number.trim() ? 'number' : !(num(l.amount) > 0) ? 'amount' : !l.date ? 'date' : dir === 'in' ? (l.bank.trim() ? '' : 'bank') : l.lockerId ? '' : 'bank account'
export const chequesReady = (ls: ChequeDraftLine[], dir: 'in' | 'out') => ls.length > 0 && ls.every((l) => chequeIssue(l, dir) === '')
export const toChequeLines = (ls: ChequeDraftLine[]): ChequeLine[] =>
  ls.map((l) => ({ bank: l.bank.trim(), lockerId: Number(l.lockerId) || 0, number: l.number.trim(), amount: l.amount, date: l.date, note: l.note.trim() }))

const amt = (v: string) => v.replace(/[^\d.]/g, '')

/** The cheques of one payment: add / remove lines, each with its bank (or the
 *  shop's bank place it's drawn on), number, amount and date; shows their total
 *  against the payment and warns — never blocks — on a cheque seen before. */
export function ChequeLines({ dir, lines, onChange, places = [], sym, want }: {
  dir: 'in' | 'out'
  lines: ChequeDraftLine[]
  onChange: (ls: ChequeDraftLine[]) => void
  places?: { id: number; name: string }[] // given: the bank places a cheque can be drawn on
  sym: string
  want?: number // the payment's amount, when it is fixed
}) {
  const banks = useQuery({ ...chequeBanksQuery, enabled: dir === 'in' }).data ?? []
  const [seen, setSeen] = useState<Record<number, boolean>>({})
  const set = (i: number, patch: Partial<ChequeDraftLine>) => onChange(lines.map((l, j) => (j === i ? { ...l, ...patch } : l)))
  const check = (i: number) => {
    const l = lines[i]
    void chequeSeen(dir, l.number, l.bank, Number(l.lockerId) || 0).then((d) => setSeen((s) => ({ ...s, [i]: d })), () => undefined)
  }
  const total = chequesTotal(lines)
  const off = want !== undefined && lines.length > 0 && Math.abs(total - want) > 0.005
  return (
    <div className="flex flex-col gap-2" role="group" aria-label="Cheques">
      {dir === 'in' && <datalist id="cheque-banks">{banks.map((b) => <option key={b} value={b} />)}</datalist>}
      {lines.map((l, i) => (
        <div key={i} className="grid grid-cols-2 gap-2 rounded-[var(--radius-sm)] border border-[color-mix(in_srgb,var(--cheque)_35%,var(--border))] bg-[color-mix(in_srgb,var(--cheque)_6%,transparent)] p-2.5 sm:grid-cols-[1.4fr_1fr_1fr_1fr_auto]">
          {dir === 'in' ? (
            <input aria-label={`Cheque ${i + 1} bank`} placeholder="Bank" list="cheque-banks" autoComplete="off" value={l.bank}
              onChange={(e) => set(i, { bank: e.target.value })} onBlur={() => check(i)} className={cn(INPUT, 'col-span-2 sm:col-span-1')} />
          ) : (
            <select aria-label={`Cheque ${i + 1} drawn on`} value={l.lockerId} onChange={(e) => set(i, { lockerId: e.target.value })} className={cn(INPUT, 'col-span-2 appearance-auto sm:col-span-1')}>
              <option value="">— drawn on —</option>
              {places.map((p) => <option key={p.id} value={p.id}>{p.name}</option>)}
            </select>
          )}
          <input aria-label={`Cheque ${i + 1} number`} placeholder="Cheque no." inputMode="numeric" autoComplete="off" value={l.number}
            onChange={(e) => { set(i, { number: e.target.value }); setSeen((s) => ({ ...s, [i]: false })) }} onBlur={() => check(i)} className={INPUT} />
          <input aria-label={`Cheque ${i + 1} amount`} placeholder="Amount" inputMode="decimal" autoComplete="off" value={l.amount}
            onChange={(e) => set(i, { amount: amt(e.target.value) })} className={cn(INPUT, 'tabular-nums')} />
          <input aria-label={`Cheque ${i + 1} date`} type="date" value={l.date} onChange={(e) => set(i, { date: e.target.value })} className={INPUT} />
          <button type="button" aria-label={`Remove cheque ${i + 1}`} disabled={lines.length === 1} onClick={() => onChange(lines.filter((_, j) => j !== i))}
            className="col-span-2 justify-self-end text-faint hover:text-danger disabled:opacity-30 sm:col-span-1 sm:self-center"><X size={16} /></button>
          {seen[i] && <p role="status" className="col-span-2 text-[12px] font-semibold text-warning sm:col-span-5">This cheque number was used before — check it isn't the same cheque.</p>}
        </div>
      ))}
      <div className="flex flex-wrap items-center justify-between gap-2 text-[13px]">
        <button type="button" onClick={() => onChange([...lines, blankCheque('', dir === 'out' && lines[0] ? lines[0].lockerId : '')])}
          className="inline-flex items-center gap-1 font-semibold text-cheque"><Plus size={14} />Add a cheque</button>
        <span className={cn('tabular-nums', off ? 'font-bold text-danger' : 'text-muted-foreground')}>
          {lines.length > 1 || off ? `Cheques ${money(sym, total)}` : ''}{off ? ` — the payment is ${money(sym, want!)}` : ''}
        </span>
      </div>
    </div>
  )
}
