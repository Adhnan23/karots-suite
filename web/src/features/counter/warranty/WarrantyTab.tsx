import { keepPreviousData, useInfiniteQuery } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { Seg } from '@/features/till/bits'
import { FIELD } from '@/features/till/ui'
import { cn } from '@/lib/utils'
import { countText, warrantyUnitsQuery } from '../queries'
import { CounterPage, MoreRows, PaneTitle } from '../ui'
import { ReceiptClaim } from './ReceiptClaim'
import { UnitClaim } from './UnitClaim'

type Mode = 'serial' | 'receipt'
type Status = 'active' | 'expired' | 'replaced' | 'all'

/** Warranty at the counter: swap a faulty serial, or replace an un-serialled item by its receipt. */
export function WarrantyTab() {
  const [mode, setMode] = useState<Mode>('serial')
  const [serial, setSerial] = useState<string | null>(null)
  const [receipt, setReceipt] = useState<string | null>(null)
  const aside = mode === 'serial'
    ? serial ? <UnitClaim key={serial} serial={serial} onSerial={setSerial} /> : <Hint text="Scan the serial on the item, or pick it from the list." />
    : receipt ? <ReceiptClaim key={receipt} no={receipt} /> : <Hint text="Items without a serial are claimed by their receipt number." />
  return (
    <CounterPage flush aside={aside}>
      <div className="px-4 pt-3.5">
        <Seg label="Find by" value={mode} onChange={setMode} options={[['serial', 'By serial'], ['receipt', 'By receipt']]} />
      </div>
      {mode === 'serial' ? <UnitList picked={serial} onPick={setSerial} /> : <ReceiptFind onFind={setReceipt} />}
    </CounterPage>
  )
}

function Hint({ text }: { text: string }) {
  return <p className="m-auto max-w-[280px] p-8 text-center text-faint">{text}</p>
}

function UnitList({ picked, onPick }: { picked: string | null; onPick: (s: string) => void }) {
  const [q, setQ] = useState('')
  const [status, setStatus] = useState<Status>('active')
  const dq = useDeferredValue(q.trim())
  const list = useInfiniteQuery({ ...warrantyUnitsQuery(status, dq), placeholderData: keepPreviousData })
  const rows = list.data?.pages.flat() ?? []
  return (
    <>
      <PaneTitle title="Warranty" sub={`${countText(rows.length, list.hasNextPage)} unit${rows.length === 1 ? '' : 's'}`}
        right={<Seg label="Status" value={status} onChange={setStatus} options={[['active', 'Covered'], ['expired', 'Expired'], ['replaced', 'Replaced'], ['all', 'All']]} />} />
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        {/* Enter looks the typed serial up directly — a scan is exact, the list may lag. */}
        <input autoFocus aria-label="Serial number" placeholder="Scan a serial, or type a product" autoComplete="off" value={q}
          onChange={(e) => setQ(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && q.trim() && onPick(q.trim())}
          className={cn(FIELD, 'w-full pl-9 font-mono')} />
      </div>
      <ul aria-label="Units" className="flex min-h-0 flex-1 flex-col gap-1 overflow-auto px-2.5 pb-2.5">
        {rows.length === 0 && <li className="m-auto p-8 text-center text-faint">No units here.</li>}
        {rows.map((u) => (
          <li key={u.id}>
            <button type="button" aria-pressed={picked === u.serial} onClick={() => onPick(u.serial)}
              className="flex w-full items-center gap-3 rounded-[var(--radius-sm)] px-3 py-2 text-left hover:bg-muted aria-pressed:bg-selected aria-pressed:text-selected-foreground">
              <span className="min-w-0 flex-1">
                <b className="block truncate text-[14px]">{u.productName}</b>
                <small className="block truncate font-mono opacity-70">{u.serial}{u.customerName && <span className="font-sans"> · {u.customerName}</span>}</small>
              </span>
              <small className={cn('font-semibold', picked !== u.serial && (u.underWarranty ? 'text-success' : 'text-muted-foreground'))}>
                {u.status === 'replaced' ? 'replaced' : u.left}
              </small>
            </button>
          </li>
        ))}
        <MoreRows list={list} />
      </ul>
    </>
  )
}

function ReceiptFind({ onFind }: { onFind: (no: string) => void }) {
  const [no, setNo] = useState('')
  return (
    <>
      <PaneTitle title="Claim by receipt" />
      <div className="relative mx-3 mb-2">
        <Search size={16} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-faint" />
        <input autoFocus aria-label="Receipt number" placeholder="Scan or type the receipt no, then Enter" autoComplete="off" value={no}
          onChange={(e) => setNo(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && no.trim() && onFind(no.trim().toUpperCase())}
          className={cn(FIELD, 'w-full pl-9 font-mono')} />
      </div>
      <p className="px-5 text-[13px] text-muted-foreground">Works on any past sale with warranted items — nothing needed to be recorded at the time.</p>
    </>
  )
}
