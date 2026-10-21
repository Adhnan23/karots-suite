import { useQuery } from '@tanstack/react-query'
import { motion } from 'motion/react'
import { tx } from '@/theme/motion'
import { useState } from 'react'
import type { TillProduct } from '@/api/types.gen'
import { configQuery } from '@/features/till/api'
import { CHIP, TILE } from '@/features/till/ui'
import { money, num } from '@/lib/money'
import { cn } from '@/lib/utils'
import type { TillApi } from '@/plugins/slots'
import { tillServicesQuery } from './api'
import { JobPanel } from './JobPanel'
import { EMOJI } from './labels'
import type { TillService } from './types'

/** Lowest price a service starts at, for its tile. */
function from(s: TillService) {
  const all = s.offers.flatMap((o) => o.bands.map((b) => num(b.price)))
  return all.length ? Math.min(...all) : null
}

/** Till → 🖨 Print & Copy: the services as tiles, the picked one's job panel below.
 * Opened with a product (picked from search / a card / a scan) it starts on that service. */
export function TillPanel({ till, product }: { till: TillApi; onDone: () => void; product?: TillProduct }) {
  const sym = useQuery(configQuery).data?.symbol ?? ''
  const list = useQuery(tillServicesQuery).data
  const [picked, setPicked] = useState<number | null>(null)
  if (!list) return null
  if (list.length === 0) return <p className="p-8 text-center text-faint">No print services yet. Add them in Apps → Print &amp; Copy.</p>
  const current = list.find((s) => s.id === picked) ?? list.find((s) => s.productId === product?.id) ?? (list.length === 1 ? list[0] : null)
  return (
    <div className="flex flex-col gap-4">
      {current ? (
        <div role="group" aria-label="Print services" className="flex flex-wrap gap-1.5">
          {list.map((s) => (
            <button key={s.id} type="button" className={CHIP} aria-pressed={current.id === s.id} onClick={() => setPicked(s.id)}>
              <span aria-hidden>{EMOJI[s.category]}</span>{s.name}
            </button>
          ))}
        </div>
      ) : (
      <div className="grid grid-cols-[repeat(auto-fill,minmax(150px,1fr))] gap-2.5">
        {list.map((s) => {
          const f = from(s)
          return (
            <motion.button key={s.id} type="button" onClick={() => setPicked(s.id)} whileHover={{ y: -2 }} whileTap={{ scale: 0.97 }} transition={tx('fast')}
              className={cn(TILE, 'min-h-[84px] aria-pressed:border-selected aria-pressed:ring-1 aria-pressed:ring-selected')}>
              <span className="flex items-center gap-2 text-[14px] font-semibold leading-tight"><span aria-hidden className="text-[20px]">{EMOJI[s.category]}</span>{s.name}</span>
              <span className="text-[12.5px] text-muted-foreground tabular-nums">{s.kind === 'custom' ? 'Price at the till' : f !== null ? `from ${money(sym, f)}` : 'No prices yet'}</span>
            </motion.button>
          )
        })}
      </div>
      )}
      {current && (
        <section key={current.id} aria-label={current.name} className="flex flex-col gap-3">
          <JobPanel s={current} till={till} />
        </section>
      )}
    </div>
  )
}
