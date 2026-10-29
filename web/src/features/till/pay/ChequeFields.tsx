import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { chequeBanksQuery, chequeSeen } from '@/features/money/cheques/queries'
import { cn } from '@/lib/utils'
import type { ChequeDraft } from '../cart/types'
import { FIELD } from '../ui'

/** The customer's cheque: bank (suggested), number, date, note. */
export function ChequeFields({ value, onChange }: { value: ChequeDraft; onChange: (c: ChequeDraft) => void }) {
  const banks = useQuery(chequeBanksQuery).data ?? []
  const [seen, setSeen] = useState(false)
  const set = (patch: Partial<ChequeDraft>) => onChange({ ...value, ...patch })
  const check = () => void chequeSeen('in', value.number, value.bank).then(setSeen, () => setSeen(false))
  return (
    <div className="grid grid-cols-2 gap-1.5">
      <input aria-label="Bank" placeholder="Bank" list="till-cheque-banks" autoComplete="off" value={value.bank}
        onChange={(e) => set({ bank: e.target.value })} onBlur={check} className={cn(FIELD, 'col-span-2')} />
      <datalist id="till-cheque-banks">{banks.map((b) => <option key={b} value={b} />)}</datalist>
      <input aria-label="Cheque number" placeholder="Cheque no." inputMode="numeric" autoComplete="off" value={value.number}
        onChange={(e) => { set({ number: e.target.value }); setSeen(false) }} onBlur={check} className={FIELD} />
      <input aria-label="Cheque date" type="date" value={value.date} onChange={(e) => set({ date: e.target.value })} className={FIELD} />
      {seen && <p role="status" className="col-span-2 text-[12px] font-semibold text-warning">This bank and cheque number were taken before — check it isn't the same cheque.</p>}
    </div>
  )
}
