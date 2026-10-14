import { useNavigate } from '@tanstack/react-router'
import { EmptyState, PageHead } from '@/components/admin/PageHead'

export const CHIP = 'rounded-full border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground whitespace-nowrap aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'
const TABS = [
  { path: '', label: 'Patients' },
  { path: 'refills', label: 'Refills due' },
  { path: 'controlled', label: 'Controlled' },
  { path: 'dispensed', label: 'Dispensed' },
]

/** Apps → Pharmacy: one page, a tab per list; sub-pages (a patient, a drug's register) by path. */
export function PharmacyPage({ path }: { path: string }) {
  const go = useNavigate()
  const tab = path.split('/')[0]
  return (
    <>
      <PageHead title="Pharmacy" sub="Patients, prescriptions, refills and the controlled register." />
      <div className="mb-3 flex flex-wrap items-center gap-1.5">
        {TABS.map((t) => (
          <button key={t.path} type="button" aria-pressed={tab === t.path} className={CHIP}
            onClick={() => void go({ to: '/admin/x/$key/$', params: { key: 'pharmacy', _splat: t.path } })}>
            {t.label}
          </button>
        ))}
      </div>
      <EmptyState title="Nothing here yet" />
    </>
  )
}
