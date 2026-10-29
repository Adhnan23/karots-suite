import { METHOD_LABEL, type ChequeStatus } from '@/lib/payMethods'
import { cn } from '@/lib/utils'

/** How a payment was made, as a small pill. Cheques wear their own colour so
 *  they stand out from cash / card / online; a pending one says so, a bounced
 *  or cancelled one turns warning / muted. Every money list uses this, so the
 *  look lives in one place. */
export function MethodBadge({ method, label, status, className }: { method: string; label?: string; status?: ChequeStatus | string; className?: string }) {
  const text = label ?? METHOD_LABEL[method] ?? method
  const cheque = method === 'cheque'
  const returned = cheque && (status === 'bounced' || status === 'cancelled')
  return (
    <span
      data-method={method}
      className={cn(
        'inline-flex items-center gap-1 whitespace-nowrap rounded-pill px-2 py-0.5 text-[11.5px] font-bold',
        !cheque && 'bg-muted text-muted-foreground',
        cheque && !returned && 'bg-[color-mix(in_srgb,var(--cheque)_14%,transparent)] text-cheque',
        returned && status === 'bounced' && 'bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] text-warning',
        returned && status === 'cancelled' && 'bg-muted text-muted-foreground line-through',
        className,
      )}>
      {text}
      {cheque && status && status !== 'cleared' && <span className="font-semibold opacity-80">· {status}</span>}
    </span>
  )
}
