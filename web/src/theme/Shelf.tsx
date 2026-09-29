import { Link } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import type { NavArea } from './nav'
import { apps } from './nav'

type Props =
  | { kind: 'area'; area: NavArea; onClose: () => void }
  | { kind: 'apps'; onClose: () => void }

/** The panel a dock click raises above it (theme.md §6). Closes on Escape or a
 * click outside. Apps is the plugin slot — empty until M7. */
export function Shelf(props: Props) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') props.onClose()
    }
    const onClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) props.onClose()
    }
    document.addEventListener('keydown', onKey)
    document.addEventListener('mousedown', onClick)
    return () => {
      document.removeEventListener('keydown', onKey)
      document.removeEventListener('mousedown', onClick)
    }
  }, [props])

  const label = props.kind === 'apps' ? 'Apps' : props.area.label

  return (
    <div
      ref={ref}
      role="dialog"
      aria-label={label}
      className="fixed bottom-[104px] left-1/2 z-20 w-[min(880px,calc(100vw-32px))] -translate-x-1/2 rounded-[22px] border border-border bg-popover p-5 shadow-[var(--shadow-float)]"
    >
      {props.kind === 'apps' ? (
        apps.length === 0 ? (
          <p className="p-6 text-center text-sm text-muted-foreground">No apps installed on this shop yet.</p>
        ) : (
          <div className="grid grid-cols-4 gap-2.5">
            {apps.map((a) => (
              <button key={a.key} type="button" className="flex flex-col gap-2.5 rounded-2xl border border-border bg-card p-3.5 text-left">
                <span className="grid h-11 w-11 place-items-center rounded-[13px] text-white" style={{ background: a.color }}>
                  <a.icon size={20} />
                </span>
                <b className="text-sm">{a.label}</b>
                <span className="text-xs text-muted-foreground">{a.description}</span>
              </button>
            ))}
          </div>
        )
      ) : props.area.pages.length === 0 ? (
        <p className="p-6 text-center text-sm text-muted-foreground">{props.area.label} arrives in a later milestone.</p>
      ) : (
        <div className="grid grid-cols-3 gap-1 gap-x-2.5">
          {props.area.pages.map((p) => (
            <Link key={p.to} to={p.to} onClick={props.onClose} className="flex items-start gap-2.5 rounded-xl p-2.5 text-left hover:bg-muted">
              <span>
                <b className="block text-sm">
                  {p.label}
                  {p.badge ? <span className="ml-1.5 rounded-full bg-danger px-1.5 text-[11px] font-extrabold text-white">{p.badge}</span> : null}
                </b>
                <span className="text-xs text-muted-foreground">{p.description}</span>
              </span>
            </Link>
          ))}
        </div>
      )}
    </div>
  )
}
