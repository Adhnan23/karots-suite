import { useQuery } from '@tanstack/react-query'
import { ArrowLeft, Printer } from 'lucide-react'
import { useState } from 'react'
import { INPUT } from '@/components/admin/fields'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { qtyText } from '@/lib/money'
import { cn } from '@/lib/utils'
import { registerQuery } from './api'
import { useGo } from './nav'

const when = (iso: string) => new Date(iso).toLocaleString('en-GB', { day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit' })
const TYPE: Record<string, string> = { purchase: 'Received', sale: 'Sold', return: 'Returned', adjust: 'Count', damage: 'Damaged / expired', conversion: 'Repacked',
  purchase_return: 'Back to supplier', recovery: 'Replaced by supplier', own_use: 'Shop use', warranty_replacement: 'Warranty', staff: 'Staff' }

/** One controlled drug's register: every stock movement with the balance after it;
 * a sale names the patient and doctor. Prints on A4 (Print). */
export function RegisterPage({ productId }: { productId: number }) {
  const go = useGo()
  const [from, setFrom] = useState('')
  const r = useQuery(registerQuery(productId, from)).data
  if (!r) return <EmptyState title="Loading…" />
  return (
    <>
      <button type="button" onClick={() => go('controlled')} className="mb-2 flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground print:hidden"><ArrowLeft size={14} /> Controlled</button>
      <PageHead title={`${r.name} — controlled register`} sub={`${r.schedule || 'Controlled'} · ${qtyText(r.stock)} in stock`}
        actions={<Button variant="outline" className="h-10 gap-1.5 px-4 print:hidden" onClick={() => window.print()}><Printer size={16} /> Print</Button>} />
      <div className="mb-3 flex flex-wrap items-center gap-2 print:hidden">
        <label className="flex items-center gap-2 text-[13px] font-semibold text-muted-foreground">From
          <input aria-label="Register from" type="date" value={from} onChange={(e) => setFrom(e.target.value)} className={`${INPUT} h-10 w-44`} />
        </label>
      </div>
      <div className={cn(SURFACE, 'overflow-x-auto')}>
        <table aria-label="Register" className="w-full min-w-[640px] text-sm">
          <thead>
            <tr className="border-b border-border text-left text-[12px] font-bold uppercase tracking-wide text-faint">
              <th className="px-3 py-2">When</th><th className="px-3 py-2">What</th><th className="px-3 py-2">Patient · doctor</th>
              <th className="px-3 py-2 text-right">In</th><th className="px-3 py-2 text-right">Out</th><th className="px-3 py-2 text-right">Balance</th>
            </tr>
          </thead>
          <tbody>
            <tr className="border-b border-border text-muted-foreground"><td className="px-3 py-2" colSpan={5}>Opening balance{r.from && ` on ${r.from}`}</td><td className="px-3 py-2 text-right tabular-nums">{qtyText(r.opening)}</td></tr>
            {r.rows.map((x) => (
              <tr key={x.id} className="border-b border-border last:border-0">
                <td className="px-3 py-2 whitespace-nowrap">{when(x.at)}</td>
                <td className="px-3 py-2">{TYPE[x.type] ?? x.type}{x.reference && <span className="ml-1.5 font-mono text-[12px] text-faint">{x.reference}</span>}{x.note && <small className="block text-faint">{x.note}</small>}</td>
                <td className="px-3 py-2">{x.type === 'sale' ? (x.patient ? `${x.patient}${x.prescriber ? ` · ${x.prescriber}` : ''}` : <span className="font-semibold text-warning">— no record yet</span>) : ''}</td>
                <td className="px-3 py-2 text-right tabular-nums">{x.in && qtyText(x.in)}</td>
                <td className="px-3 py-2 text-right tabular-nums">{x.out && qtyText(x.out)}</td>
                <td className="px-3 py-2 text-right font-semibold tabular-nums">{qtyText(x.balance)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {r.rows.length === 0 && <p className="px-3 py-4 text-sm text-muted-foreground">No movements{r.from ? ' since then' : ' yet'}.</p>}
      </div>
      {r.old.length > 0 && (
        <details className="mt-4">
          <summary className="cursor-pointer text-sm font-semibold">Old register ({r.old.length} rows, kept by hand before)</summary>
          <div className={cn(SURFACE, 'mt-2 divide-y divide-border text-sm')}>
            {r.old.map((o, i) => (
              <div key={i} className="flex flex-wrap gap-x-3 px-3 py-2">
                <span className="w-40 text-muted-foreground">{when(o.at)}</span>
                <span className="w-16 font-semibold">{o.direction === 'in' ? `+${qtyText(o.qty)}` : `−${qtyText(o.qty)}`}</span>
                <span className="flex-1">{[o.patient, o.prescriber, o.reference, o.notes].filter(Boolean).join(' · ')}</span>
              </div>
            ))}
          </div>
        </details>
      )}
    </>
  )
}
