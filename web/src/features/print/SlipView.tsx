import { useQuery } from '@tanstack/react-query'
import { configQuery } from '@/features/till/api'
import { SlipPaper } from '@/features/till/SlipPaper'
import { cn } from '@/lib/utils'
import { slipPictureQuery } from './picture'

/** A slip exactly as it prints, in either mode: Image mode shows the very
 * picture the printer gets; Classic shows the decoded printer bytes. The only
 * receipt viewer — every screen that shows a slip uses it. */
export function SlipView({ path, size, className }: { path: string; size?: string; className?: string }) {
  const cfg = useQuery(configQuery).data
  if (!cfg) return null
  return cfg.printAs === 'image' ? <SlipPicture path={path} size={size} className={className} /> : <SlipPaper path={path} size={size} className={className} />
}

function SlipPicture({ path, size, className }: { path: string; size?: string; className?: string }) {
  const q = useQuery(slipPictureQuery(path, size))
  if (!q.data) {
    return <p className={cn('mx-auto w-[288px] bg-paper py-10 text-center text-[13px] text-ink opacity-60', className)}>{q.isError ? 'No preview' : 'Loading the receipt…'}</p>
  }
  // Half size on screen (a 576-dot roll ≈ 288 px), smoothly scaled; the printer gets every dot.
  // The paper frame around it is screen-only breathing room — the print is the picture alone.
  return (
    <div className={cn('mx-auto w-fit bg-paper px-3 py-4 shadow-[var(--shadow)]', className)}>
      <img src={q.data.url} alt="Receipt preview" className="block" style={{ width: q.data.dots / 2 }} />
    </div>
  )
}
