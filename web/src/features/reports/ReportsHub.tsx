import { Link } from '@tanstack/react-router'
import { ChevronRight, Puzzle } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import type { Me } from '@/api/types.gen'
import { PageHead } from '@/components/admin/PageHead'
import { slots } from '@/plugins/slots'
import { GROUPS, LIVE, REPORTS } from './hub'

function Card({ to, title, desc, icon: Icon }: { to: string; title: string; desc: string; icon: LucideIcon }) {
  return (
    <Link to={to} className="group flex items-start gap-3 rounded-[var(--radius)] border border-border bg-card p-4 shadow-[var(--shadow)] transition-colors hover:border-selected">
      <span className="grid h-10 w-10 flex-none place-items-center rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--brand)_14%,transparent)] text-brand"><Icon size={20} /></span>
      <span className="min-w-0 flex-1">
        <b className="block font-display text-[15px]">{title}</b>
        <span className="text-[13px] text-muted-foreground">{desc}</span>
      </span>
      <ChevronRight size={18} className="mt-2.5 flex-none text-faint transition-transform group-hover:translate-x-0.5" />
    </Link>
  )
}

function Group({ label, children }: { label: string; children: ReactNode }) {
  return (
    <section aria-label={label} className="flex flex-col gap-2.5">
      <h2 className="text-[12px] font-bold uppercase tracking-[0.08em] text-muted-foreground">{label}</h2>
      <div className="grid gap-2.5 sm:grid-cols-2 xl:grid-cols-3">{children}</div>
    </section>
  )
}

/** Reports hub (karots-pos /admin/reports): the reports by group, plugin
 * reports under More, and the live pages people look for here. */
export function ReportsHub({ me }: { me: Me }) {
  return (
    <>
      <PageHead title="Reports" sub="Pick one, set the period, then export it or print it." />
      <div className="flex flex-col gap-7">
        {GROUPS.map(([g, label]) => {
          const cards = REPORTS.filter((r) => r.group === g && (r.gate !== 'tax' || me.taxRegistered))
          return cards.length > 0 && <Group key={g} label={label}>{cards.map((r) => <Card key={r.to} {...r} />)}</Group>
        })}
        {slots.reportCards.length > 0 && (
          <Group label="More">{slots.reportCards.map((c) => <Card key={c.href} to={c.href} title={c.title} desc={c.desc} icon={Puzzle} />)}</Group>
        )}
        <Group label="Live views">{LIVE.map((l) => <Card key={l.to} {...l} />)}</Group>
      </div>
    </>
  )
}
