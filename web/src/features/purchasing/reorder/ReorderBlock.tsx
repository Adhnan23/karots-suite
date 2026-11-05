import { Layers } from 'lucide-react'

/** The heading over rows that belong together: their group, and how it stands as a whole. */
export function BlockHead({ name, line }: { name: string; line: string }) {
  return (
    <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 bg-brand-soft px-3 py-2">
      <b className="flex min-w-0 items-center gap-1.5 break-words"><Layers size={15} className="shrink-0" /> {name}</b>
      {line && <small className="text-[13px] font-semibold tabular-nums">{line}</small>}
    </div>
  )
}
