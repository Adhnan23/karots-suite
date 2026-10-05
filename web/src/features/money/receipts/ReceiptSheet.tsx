import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ExternalLink } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import type { DebtReceiptRow, MoneyReceiptRow, WarrantyClaimRow } from '@/api/types.gen'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { saleQuery } from '@/features/counter/queries'
import { DebtPaper, MoneyPaper, SalePaper, WarrantyPaper } from '@/features/counter/receipts/Paper'
import { Seg } from '@/features/till/bits'
import { configQuery, errMsg } from '@/features/till/api'
import { printDebtSlip, printMoneySlip, printSale, printWarrantySlip } from '@/features/till/print'

export type Pick = { kind: 'sales'; id: number } | { kind: 'cash'; row: MoneyReceiptRow } | { kind: 'credit'; row: DebtReceiptRow } | { kind: 'warranty'; row: WarrantyClaimRow }

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
  const narrow = size === '58'
  const sale = useQuery({ ...saleQuery(pick.kind === 'sales' ? pick.id : 0), enabled: pick.kind === 'sales' }).data
  const paper = pick.kind === 'sales'
    ? sale && <SalePaper s={sale} shop={cfg.shop} sym={cfg.symbol} narrow={narrow} />
    : pick.kind === 'cash' ? <MoneyPaper r={pick.row} shop={cfg.shop} sym={cfg.symbol} narrow={narrow} />
      : pick.kind === 'credit' ? <DebtPaper r={pick.row} shop={cfg.shop} sym={cfg.symbol} narrow={narrow} /> : <WarrantyPaper r={pick.row} shop={cfg.shop} narrow={narrow} />
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
      <div className="rounded-[var(--radius-sm)] bg-muted/60 p-4">{paper ?? <p className="text-center text-faint">Loading…</p>}</div>
    </FormSheet>
  )
}
