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
  const image = cfg.printAs === 'image'
  // A set width (the roll at screen scale), never more than the space given: a box sized to
  // its content (the sale-complete overlay) then makes room for the slip — the slip alone only
  // says "up to 100%", which such a box resolves to almost nothing.
  return (
    <div className="mx-auto max-w-full" style={{ width: rollPx(image, size || cfg.receiptWidth) }}>
      {image ? <SlipPicture path={path} size={size} className={className} /> : <SlipPaper path={path} size={size} className={className} />}
    </div>
  )
}

/** The roll on screen: Image = its dots at 0.7×, Classic = its characters at 12 px (0.6 em each), plus the paper's padding. */
const rollPx = (image: boolean, mm: string) => {
  const narrow = mm === '58'
  return (image ? (narrow ? 384 : 576) * 0.7 : (narrow ? 32 : 48) * 7.2) + 24
}

function SlipPicture({ path, size, className }: { path: string; size?: string; className?: string }) {
  const q = useQuery(slipPictureQuery(path, size))
  if (!q.data) {
    return <p className={cn('mx-auto w-[403px] max-w-full bg-paper py-10 text-center text-[13px] text-ink opacity-60', className)}>{q.isError ? 'No preview' : 'Loading the receipt…'}</p>
  }
  // 0.7× on screen (a 576-dot roll ≈ 403 px), smoothly scaled: at half size the 1-dot strokes
  // of a 1-bit picture go soft on a normal screen. The printer gets every dot.
  // The paper frame around it is screen-only breathing room — the print is the picture alone.
  return (
    <div className={cn('mx-auto w-fit max-w-full bg-paper px-3 py-4 shadow-[var(--shadow)]', className)}>
      <img src={q.data.url} alt="Receipt preview" className="block h-auto max-w-full" style={{ width: q.data.dots * 0.7 }} />
    </div>
  )
}
