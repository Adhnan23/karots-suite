import { useQuery } from '@tanstack/react-query'
import type { CSSProperties } from 'react'
import { api } from '@/api/client'
import type { SlipLine, SlipPreview } from '@/api/types.gen'
import { cn } from '@/lib/utils'

/** A slip exactly as it will print: the server decodes the very bytes the printer
 * would get (`?preview=1`), so logo, layout, bold / big text and the credit line
 * are the paper's. One character = 1ch of the mono font, so the roll's width in
 * characters is the paper's width on screen. Paper and ink in every palette. */
export function SlipPaper({ path, className }: { path: string; className?: string }) {
  const q = useQuery({
    queryKey: ['slip-preview', path],
    queryFn: () => api<SlipPreview>(`${path}?preview=1`, { method: 'POST' }),
  })
  const p = q.data
  return (
    <article aria-label="Receipt preview" className={cn('mx-auto bg-paper px-3 py-4 font-mono text-[12px] leading-[1.35] text-ink shadow-[var(--shadow)]', className)}
      style={{ width: `calc(${p?.columns ?? 32}ch + 1.5rem)` }}>
      {!p ? <p className="py-10 text-center font-sans text-[13px] opacity-60">{q.isError ? 'No preview' : 'Loading the receipt…'}</p>
        : p.lines.map((l, i) => <Line key={i} l={l} dots={p.dots} />)}
    </article>
  )
}

function Line({ l, dots }: { l: SlipLine; dots: number }) {
  const align = l.align === 'center' ? 'text-center' : l.align === 'right' ? 'text-right' : 'text-left'
  if (l.image) {
    return <img src={l.image} alt="" className={cn('block [image-rendering:pixelated]', l.align === 'center' ? 'mx-auto' : l.align === 'right' ? 'ml-auto' : '')}
      style={{ width: `${Math.min(100, ((l.imageDots ?? dots) / dots) * 100)}%` }} />
  }
  // Double width + height = twice the font size (half the characters fill the
  // line); double height alone stretches the glyphs; Font B is the small font.
  const style: CSSProperties = l.wide && l.tall ? { fontSize: '2em', lineHeight: 1.15 }
    : l.small ? { fontSize: '0.75em' } : {}
  const text = l.text || ' '
  return (
    <div className={cn('whitespace-pre', align, l.bold && 'font-bold', l.tall && !l.wide && 'py-[0.35em]')} style={style}>
      {l.tall && !l.wide ? <span className="inline-block origin-center scale-y-[1.8]">{text}</span> : text}
    </div>
  )
}
