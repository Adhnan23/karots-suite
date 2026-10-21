import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { X } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { tx } from '@/theme/motion'
import { useEffect } from 'react'
import { money, num, qtyText } from '@/lib/money'
import { configQuery, drawerQuery } from '../api'
import { notesFor } from '../cart/totals'
import { useTill } from '../TillProvider'

/** Auto-print mode: the last sale's change, kept visible after the reset (no modal)
 * until the next add, the ✕, or 20 seconds. */
export function PostSaleChange() {
  const cfg = useSuspenseQuery(configQuery).data
  const { state, dispatch } = useTill()
  const denoms = (useQuery(drawerQuery).data?.denominations ?? []).map((d) => num(d.value))
  const amt = state.postSaleChange
  useEffect(() => {
    if (!amt) return
    const t = setTimeout(() => dispatch({ type: 'clearPostSaleChange' }), 20_000)
    return () => clearTimeout(t)
  }, [amt, dispatch])
  return (
    <AnimatePresence>
      {amt > 0 && (
        <motion.div initial={{ height: 0, opacity: 0 }} animate={{ height: 'auto', opacity: 1 }} exit={{ height: 0, opacity: 0 }} transition={tx()}
          role="status" className="mx-2.5 mb-2 flex items-center gap-3 overflow-hidden rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--success)_16%,transparent)] px-3 py-2">
          <span className="flex-1">
            <span className="text-[13px] font-semibold">Give change</span>{' '}
            <b className="font-display text-xl text-success tabular-nums">{money(cfg.symbol, amt)}</b>
            <span className="block text-[12px] text-muted-foreground tabular-nums">{notesFor(amt, denoms).map((n) => `${n.qty} × ${qtyText(n.value)}`).join(' · ')}</span>
          </span>
          <button type="button" aria-label="Dismiss change" onClick={() => dispatch({ type: 'clearPostSaleChange' })} className="text-faint hover:text-foreground"><X size={16} /></button>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
