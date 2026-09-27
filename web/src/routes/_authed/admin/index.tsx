import { createFileRoute, Link } from '@tanstack/react-router'
import { Store } from 'lucide-react'

export const Route = createFileRoute('/_authed/admin/')({
  component: AdminHome,
})

function greeting(h: number) {
  return h < 12 ? 'Good morning' : h < 17 ? 'Good afternoon' : 'Good evening'
}

/** M0 home: the page head in the approved style. M5 fills it with the dashboard
 * (KPIs vs yesterday, trend chart, attention list). */
function AdminHome() {
  const { me } = Route.useRouteContext()
  const now = new Date()
  const first = me.user.name.split(' ')[0]
  return (
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="font-display text-[28px] leading-tight font-bold tracking-tight">
          {greeting(now.getHours())}, {first}
        </h1>
        <p className="text-muted-foreground">
          {now.toLocaleDateString('en-GB', { weekday: 'long', day: 'numeric', month: 'long' })}, {me.shopName}
        </p>
      </div>
      <div className="relative overflow-hidden rounded-[var(--radius)] bg-hero bg-[image:var(--hero-art)] p-6 text-hero-foreground md:p-8">
        <p className="font-display text-xl font-bold md:text-2xl">Today’s figures show here once the till takes its first sale.</p>
        <p className="mt-1 max-w-[60ch] text-hero-muted">Sales, profit and cash in the drawer, compared with yesterday.</p>
        <Link
          to="/till"
          className="mt-5 inline-flex h-11 items-center gap-2 rounded-[var(--radius-sm)] bg-brand px-4 font-bold text-brand-foreground shadow-[var(--glow)]"
        >
          <Store size={18} /> Open the till
        </Link>
      </div>
    </div>
  )
}
