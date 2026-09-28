import { useQuery, useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { ChevronDown, User, UserPlus } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { TillCustomer } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import { configQuery, customersQuery, errMsg } from '../api'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'

const initials = (n: string) => n.split(/\s+/).slice(0, 2).map((w) => w[0]?.toUpperCase() ?? '').join('')

export function balanceText(sym: string, c: TillCustomer) {
  const o = num(c.outstanding)
  return o > 0 ? `owes ${money(sym, o)}` : o < 0 ? `advance ${money(sym, -o)}` : 'nothing owed'
}

export function CustomerPicker() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const ui = useTillUi()
  const qc = useQueryClient()
  const customers = useQuery(customersQuery).data ?? []
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const [limit, setLimit] = useState<string | null>(null)
  const box = useRef<HTMLDivElement>(null)
  const cur = customers.find((c) => c.id === state.customerId) ?? null
  const list = useMemo(() => {
    const s = q.trim().toLowerCase()
    return (s ? customers.filter((c) => c.name.toLowerCase().includes(s) || c.phone.includes(s)) : customers).slice(0, 50)
  }, [customers, q])

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => box.current && !box.current.contains(e.target as Node) && setOpen(false)
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  const pick = (id: number | null) => {
    dispatch({ type: 'customer', id })
    setOpen(false)
    setQ('')
  }
  async function saveLimit() {
    if (!cur || limit === null || limit.trim() === '' || num(limit) < 0) return void toast.error('Enter a valid credit limit')
    try {
      await api(`/till/customers/${cur.id}/credit-limit`, { method: 'PATCH', json: { creditLimit: limit.trim() } })
      await qc.invalidateQueries({ queryKey: customersQuery.queryKey })
      setLimit(null)
      toast.success('Credit limit updated')
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  return (
    <div ref={box} className="relative">
      <div className="flex gap-2">
        <button type="button" aria-haspopup="listbox" aria-expanded={open} onClick={() => setOpen(!open)}
          className="flex min-w-0 flex-1 items-center gap-2.5 rounded-[var(--radius-sm)] border border-border bg-muted px-3 py-2 text-left">
          <span className="grid h-8 w-8 shrink-0 place-items-center rounded-full bg-brand-soft text-[13px] font-extrabold">
            {cur ? initials(cur.name) : <User size={16} />}
          </span>
          <span className="min-w-0 flex-1">
            <span className="block truncate text-[14px] font-bold">{cur?.name ?? 'Walk-in customer'}</span>
            <span className="line-clamp-2 text-[12px] text-muted-foreground">
              {cur ? `${cur.phone} · ${balanceText(cfg.symbol, cur)} · limit ${money(cfg.symbol, cur.creditLimit)}` : 'Cash sale, no account'}
            </span>
          </span>
          <ChevronDown size={16} className="text-faint" />
        </button>
        <button type="button" aria-label="Add customer" onClick={() => ui.open({ kind: 'addCustomer' })}
          className="grid w-11 place-items-center rounded-[var(--radius-sm)] border border-border bg-muted"><UserPlus size={17} /></button>
      </div>
      {open && (
        <div className="absolute inset-x-0 top-[calc(100%+6px)] z-30 rounded-[var(--radius-sm)] border border-border bg-popover p-1.5 shadow-[var(--shadow-float)]">
          <input autoFocus aria-label="Search customers" placeholder="Name or phone" autoComplete="off" value={q} onChange={(e) => setQ(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') setOpen(false)
              if (e.key === 'Enter' && list[0]) pick(list[0].id)
            }} className={cn(FIELD, 'mb-1 h-9 w-full')} />
          <div role="listbox" aria-label="Customers" className="max-h-72 overflow-auto">
            <button type="button" role="option" aria-selected={!cur} onClick={() => pick(null)} className="w-full rounded-lg px-2.5 py-2 text-left text-[13.5px] hover:bg-muted">Walk-in</button>
            {list.map((c) => (
              <button key={c.id} type="button" role="option" aria-selected={c.id === cur?.id} onClick={() => pick(c.id)}
                className="flex w-full items-center justify-between gap-2 rounded-lg px-2.5 py-2 text-left text-[13.5px] hover:bg-muted">
                <span>{c.name} <small className="text-muted-foreground">{c.phone}</small></span>
                <small className="text-muted-foreground">{balanceText(cfg.symbol, c)}</small>
              </button>
            ))}
          </div>
        </div>
      )}
      {cur && cfg.canManageCredit && (limit === null ? (
        <button type="button" onClick={() => setLimit(num(cur.creditLimit) ? String(num(cur.creditLimit)) : '')}
          className="mt-1 text-[12px] font-semibold text-muted-foreground underline-offset-2 hover:underline">Adjust credit limit</button>
      ) : (
        <div className="mt-1.5 flex gap-1.5">
          <input autoFocus aria-label="New credit limit" inputMode="decimal" autoComplete="off" value={limit}
            onChange={(e) => setLimit(e.target.value.replace(/[^\d.]/g, ''))} onKeyDown={(e) => e.key === 'Enter' && void saveLimit()}
            className={cn(FIELD, 'h-8 min-w-0 flex-1 text-[13px]')} />
          <Button size="sm" onClick={() => void saveLimit()}>Save</Button>
          <Button size="sm" variant="ghost" onClick={() => setLimit(null)}>Cancel</Button>
        </div>
      ))}
    </div>
  )
}
