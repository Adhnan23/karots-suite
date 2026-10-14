import { useQuery } from '@tanstack/react-query'
import { PageHead } from '@/components/admin/PageHead'
import { fieldsQuery } from '../productplus/api'
import { refillsQuery, unrecordedQuery } from './api'
import { ControlledTab } from './ControlledTab'
import { CHIP, useGo } from './nav'
import { DispensedTab } from './DispensedTab'
import { PatientPage } from './PatientPage'
import { PatientsTab } from './PatientsTab'
import { RefillsTab } from './RefillsTab'
import { RegisterPage } from './RegisterPage'

/** Apps → Pharmacy: Patients · Refills due · Controlled · Dispensed; a patient and a
 * drug's register are sub-pages (patients/<id>, controlled/<product id>). */
export function PharmacyPage({ path }: { path: string }) {
  const go = useGo()
  const [tab, id] = path.split('/')
  const due = useQuery(refillsQuery).data?.length ?? 0
  const missing = useQuery(unrecordedQuery).data?.length ?? 0
  if (tab === 'patients' && Number(id)) return <PatientPage id={Number(id)} />
  if (tab === 'controlled' && Number(id)) return <RegisterPage productId={Number(id)} />
  const tabs = [
    { path: '', label: 'Patients' },
    { path: 'refills', label: `Refills due${due ? ` · ${due}` : ''}` },
    { path: 'controlled', label: `Controlled${missing ? ` · ${missing} to record` : ''}` },
    { path: 'dispensed', label: 'Dispensed' },
  ]
  return (
    <>
      <PageHead title="Pharmacy" sub="Patients, prescriptions, refills and the controlled register." />
      <div className="mb-3 flex flex-wrap items-center gap-1.5">
        {tabs.map((t) => <button key={t.path} type="button" aria-pressed={(tab ?? '') === t.path} className={CHIP} onClick={() => go(t.path)}>{t.label}</button>)}
      </div>
      <GenericHint />
      {tab === 'refills' ? <RefillsTab /> : tab === 'controlled' ? <ControlledTab /> : tab === 'dispensed' ? <DispensedTab /> : <PatientsTab />}
    </>
  )
}

/** Generic name / strength are the shop's own Product Plus fields; say so until one exists. */
function GenericHint() {
  const fields = useQuery(fieldsQuery(false)).data
  if (!fields || fields.some((f) => /generic/i.test(f.label))) return null
  return (
    <p className="mb-3 rounded-[var(--radius)] border border-dashed border-border px-3 py-2 text-[13px] text-muted-foreground">
      Tip: add a <b>Generic name</b> field in Apps → Product Plus (searchable, shown at the till) so the counter finds every brand of a medicine by its generic name.
    </p>
  )
}
