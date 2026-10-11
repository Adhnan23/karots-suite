import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { FormSheet } from '@/components/admin/FormSheet'
import { errMsg } from '@/features/till/api'
import { saveRules } from './api'
import type { ClearanceSettings } from './types'

/** The five thresholds, each saying what it does. */
export function RulesSheet({ open, onOpenChange, settings }: { open: boolean; onOpenChange: (o: boolean) => void; settings: ClearanceSettings }) {
  const [f, setF] = useState(settings)
  const [busy, setBusy] = useState(false)
  const n = (k: 'staleDays' | 'expiryDays' | 'deadMonths') => ({ value: String(f[k]), onChange: (e: React.ChangeEvent<HTMLInputElement>) => setF({ ...f, [k]: Number(e.target.value.replace(/\D/g, '')) || 0 }) })
  const d = (k: 'defaultPercent' | 'minMarginPercent') => ({ value: f[k], onChange: (e: React.ChangeEvent<HTMLInputElement>) => setF({ ...f, [k]: e.target.value.replace(/[^\d.]/g, '') }) })
  return (
    <FormSheet open={open} onOpenChange={(o) => { if (o) setF(settings); onOpenChange(o) }} title="Clearance rules" submit="Save rules" busy={busy}
      onSubmit={async () => {
        setBusy(true)
        try { await saveRules(f); toast.success('Rules saved'); onOpenChange(false) } catch (e) { toast.error(errMsg(e)) } finally { setBusy(false) }
      }}>
      <Field label="Slow after (days without a sale)" hint="A product in stock that hasn't sold for this long is suggested for a markdown. Snooze hides one for this long too.">
        <input inputMode="numeric" className={INPUT} {...n('staleDays')} />
      </Field>
      <Field label="Expiring within (days)" hint="A batch expiring this soon is suggested too, floored at that batch's own cost.">
        <input inputMode="numeric" className={INPUT} {...n('expiryDays')} />
      </Field>
      <Field label="Suggested markdown (%)" hint="What's suggested first — never more than the margin below allows.">
        <input inputMode="decimal" className={INPUT} {...d('defaultPercent')} />
      </Field>
      <Field label="Keep at least (% over cost)" hint="The suggestion never takes the price under cost plus this.">
        <input inputMode="decimal" className={INPUT} {...d('minMarginPercent')} />
      </Field>
      <Field label="Dead after (months idle)" hint="Out of stock with nothing sold or received for this long — listed under Dead stock to retire.">
        <input inputMode="numeric" className={INPUT} {...n('deadMonths')} />
      </Field>
    </FormSheet>
  )
}
