import { useNavigate } from '@tanstack/react-router'
import { PageHead } from '@/components/admin/PageHead'
import { CleanupPage } from './CleanupPage'
import { JobsPage } from './JobPage'
import { SettingsPage } from './SettingsPage'

const CHIP = 'rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground whitespace-nowrap aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'
const TABS = [{ path: '', label: 'Clean up' }, { path: 'jobs', label: 'Jobs' }, { path: 'settings', label: 'Settings' }]

/** Apps → AI Assistant. The tab is the sub-path: /admin/x/aicatalog (clean up), …/jobs, …/jobs/<key>, …/settings.
 * ✨ Identify lives inside the product form and intake, not here. */
export function AssistantPage({ path }: { path: string }) {
  const go = useNavigate()
  const tab = path === 'settings' ? 'settings' : path === 'jobs' || path.startsWith('jobs/') ? 'jobs' : ''
  return (
    <>
      <PageHead title="AI Assistant" sub="✨ Identify is in the product form and stock intake. Here: tidy the catalogue, and how the AI is reached." />
      <div className="mb-4 flex flex-wrap gap-1.5">
        {TABS.map((t) => (
          <button key={t.path} type="button" aria-pressed={tab === t.path} className={CHIP}
            onClick={() => void go({ to: '/admin/x/$key/$', params: { key: 'aicatalog', _splat: t.path } })}>{t.label}</button>
        ))}
      </div>
      {tab === 'settings' ? <SettingsPage /> : tab === 'jobs' ? <JobsPage jobKey={path.slice('jobs/'.length)} /> : <CleanupPage />}
    </>
  )
}
