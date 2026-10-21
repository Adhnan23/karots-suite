import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ExternalLink } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import type { DebtReceiptRow, MoneyReceiptRow, WarrantyClaimRow } from '@/api/types.gen'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { saleQuery } from '@/features/counter/queries'
import { SlipView } from '@/features/print/SlipView'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { printDebtSlip, printMoneySlip, printSale, printWarrantySlip, slipPath } from '@/features/till/print'

export type Pick = { kind: 'sales'; id: number } | { kind: 'cash'; row: MoneyReceiptRow } | { kind: 'credit'; row: DebtReceiptRow } | { kind: 'warranty'; row: WarrantyClaimRow }

/** The print path of a picked receipt; null for a warranty claim with no new unit (no slip). */
export const pickPath = (p: Pick) => p.kind === 'sales' ? slipPath.sale(p.id) : p.kind === 'cash' ? slipPath.money(p.row.id)
  : p.kind === 'credit' ? slipPath.debt(p.row.id) : p.row.replacementSerial !== '' ? slipPath.warranty(p.row.id) : null

/** Where a cash receipt's record lives, following its ref (karots-pos linked
 * the money receipt to its expense / supplier / customer / sale). */
function Follow({ r }: { r: MoneyReceiptRow }) {
  const cls = 'inline-flex items-center gap-1 text-[13px] font-semibold text-muted-foreground hover:text-foreground'
  if (r.refKind === 'expense') return <Link to="/admin/expenses" search={{ q: r.note.split(' - ').pop() ?? '', preset: '' }} className={cls}>Open the expense <ExternalLink size={13} /></Link>
  if (r.refKind.startsWith('supplier') && r.party) return <Link to="/admin/suppliers" search={{ q: r.party }} className={cls}>Open {r.party} <ExternalLink size={13} /></Link>
  if (r.refKind === 'customer_payment' && r.party) return <Link to="/admin/customers" search={{ q: r.party }} className={cls}>Open {r.party} <ExternalLink size={13} /></Link>
  if (r.refKind === 'sale') return <a href={`/print/receipt/${r.refId}`} target="_blank" rel="noreferrer" className={cls}>Open the sale <ExternalLink size={13} /></a>
  return null
}

/** One receipt as it printed (the counter's paper), with Print again. */
export function ReceiptSheet({ pick, onClose }: { pick: Pick; onClose: () => void }) {
  const cfg = useSuspenseQuery(configQuery).data
  const [size, setSize] = useState<'80' | '58'>(cfg.receiptWidth === '58' ? '58' : '80')
  const sale = useQuery({ ...saleQuery(pick.kind === 'sales' ? pick.id : 0), enabled: pick.kind === 'sales' }).data
  const path = pickPath(pick)
  const paper = path ? <SlipView path={path} size={size} /> : <p className="text-center text-faint">A receipt claim prints no slip.</p>
  const canReprint = pick.kind !== 'warranty' || pick.row.replacementSerial !== ''
  const reprint = () => (pick.kind === 'sales' ? printSale(pick.id, false, size) : pick.kind === 'cash' ? printMoneySlip(pick.row.id, false, size)
    : pick.kind === 'credit' ? printDebtSlip(pick.row.id, false, size) : printWarrantySlip(pick.row.id, size))
    .then(() => toast.success('Sent to the printer'), (e) => toast.error(errMsg(e)))
  const title = pick.kind === 'sales' ? (sale?.receiptNo ?? 'Sale') : pick.kind === 'warranty' ? `Warranty · ${pick.row.productName}` : pick.row.receiptNo
  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={title} submit={`Print again · ${size} mm`} busy={!canReprint} onSubmit={() => void reprint()}
      extra={pick.kind === 'sales' && (
        <Button variant="ghost" asChild className="h-10 px-3"><a href={`/print/receipt/${pick.id}`} target="_blank" rel="noreferrer"><ExternalLink size={15} /> A4 / PDF</a></Button>
      )}>
      <div className="flex items-center justify-between gap-2">
        {pick.kind === 'cash' ? <Follow r={pick.row} /> : <span />}
        <Seg label="Paper width" value={size} onChange={setSize} options={[['80', '80 mm'], ['58', '58 mm']]} />
      </div>
      <div className="rounded-[var(--radius-sm)] bg-muted/60 p-4">{paper}</div>
    </FormSheet>
  )
}
