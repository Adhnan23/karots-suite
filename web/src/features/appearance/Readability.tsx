import { CircleCheck, TriangleAlert } from 'lucide-react'
import { useEffect, useState } from 'react'
import { failing, resolver } from './contrast'

/** Lists the colour pairs that are hard to read in the mode on screen (WCAG contrast),
 *  measured after the draft palette has painted. */
export function Readability({ css, mode }: { css: string; mode: 'light' | 'dark' }) {
  const [bad, setBad] = useState<ReturnType<typeof failing>>([])
  useEffect(() => {
    const id = requestAnimationFrame(() => setBad(failing(resolver())))
    return () => cancelAnimationFrame(id)
  }, [css, mode])
  if (bad.length === 0)
    return (
      <p className="flex items-center gap-2 rounded-[var(--radius)] border border-border bg-card px-4 py-3 text-sm font-semibold shadow-[var(--shadow)]">
        <CircleCheck className="size-4 text-success" /> Everything reads well in {mode} mode
      </p>
    )
  return (
    <div role="status" aria-label="Readability" className="flex flex-col gap-1.5 rounded-[var(--radius)] border border-warning/50 bg-[color-mix(in_srgb,var(--warning)_12%,var(--card))] px-4 py-3 text-sm">
      <p className="flex items-center gap-2 font-bold"><TriangleAlert className="size-4 text-warning" /> Hard to read in {mode} mode</p>
      {bad.map((b) => (
        <p key={`${b.text}${b.on}`} className="flex flex-wrap gap-x-2">
          <span className="font-semibold">{b.what}</span>
          <span className="text-muted-foreground">{b.ratio} : 1, needs {b.min}</span>
          <span className="font-mono text-xs text-faint">{b.text} on {b.on}</span>
        </p>
      ))}
    </div>
  )
}
