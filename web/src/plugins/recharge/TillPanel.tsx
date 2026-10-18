import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import type { TillProduct } from '@/api/types.gen'
import { configQuery } from '@/features/till/api'
import { CHIP } from '@/features/till/ui'
import type { TillApi } from '../slots'
import { tillDataQuery } from './api'
import { FloatTab } from './FloatTab'
import { MoneyTab } from './MoneyTab'
import { ReloadTab } from './ReloadTab'

type Tab = 'reload' | 'billpay' | 'deposit' | 'cashout' | 'float'
const TABS: [Tab, string][] = [['reload', '📶 Reload'], ['billpay', '🧾 Pay a bill'], ['deposit', '📤 Send to wallet'], ['cashout', '💵 Cash out'], ['float', '💰 Float']]

/** Till → 📲 Reload & Bills. Everything lands on the cart and is paid (or paid out) on
 * the normal Pay. Opened from a carrier / bill product picked in search, it starts on
 * the matching tab. */
export function TillPanel({ till, product }: { till: TillApi; onDone: () => void; product?: TillProduct }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const data = useQuery(tillDataQuery).data
  const start: Tab = !data || !product ? 'reload'
    : product.id === data.products.billpay ? 'billpay' : product.id === data.products.deposit ? 'deposit'
    : product.id === data.products.cashout ? 'cashout' : 'reload'
  const [tab, setTab] = useState<Tab | null>(null)
  if (!data) return null
  const t = tab ?? start
  return (
    <div className="flex flex-col gap-4">
      <div role="tablist" aria-label="Reload & Bills" className="flex flex-wrap gap-1.5">
        {TABS.map(([v, label]) => <button key={v} role="tab" type="button" aria-selected={t === v} aria-pressed={t === v} className={CHIP} onClick={() => setTab(v)}>{label}</button>)}
      </div>
      <section role="tabpanel" aria-label={TABS.find(([v]) => v === t)![1]}>
        {t === 'reload' && <ReloadTab till={till} data={data} sym={sym} />}
        {(t === 'billpay' || t === 'deposit' || t === 'cashout') && <MoneyTab key={t} mode={t} till={till} data={data} sym={sym} />}
        {t === 'float' && <FloatTab data={data} sym={sym} />}
      </section>
    </div>
  )
}
