import type { JobInfo } from './jobs'

type FieldOption = { key: string; label: string; type: string; scoped: boolean }

/** Which of the shop's fields the chatbot may fill. Nothing starts ticked: it would make
 * up something only the shop can know, like a rack number. */
export function FieldTicks({ job, value, onChange }: { job: JobInfo; value: unknown; onChange: (v: unknown) => void }) {
  if (job.options == null) return null // no Product Plus
  const fields = job.options as FieldOption[]
  const also = job.key !== 'fields' // Identify fills them on the way
  const on = (value as { fields?: string[] } | undefined)?.fields ?? []
  const set = (key: string, tick: boolean) => onChange({ fields: tick ? [...on, key] : on.filter((k) => k !== key) })
  if (!fields.length) return also ? null : <p className="text-[14px] text-muted-foreground">Add fields in Product Plus first.</p>
  return (
    <fieldset className="flex flex-col gap-1.5">
      <legend className="pb-1 text-[13px] font-semibold">{also ? 'Also fill these fields' : 'Fill these fields'}</legend>
      <div className="grid gap-1.5 sm:grid-cols-2">
        {fields.map((f) => (
          <label key={f.key} className="flex items-center gap-2 text-[14px]">
            <input type="checkbox" checked={on.includes(f.key)} onChange={(e) => set(f.key, e.target.checked)} className="size-[18px] accent-[var(--brand)]" />
            <span className="min-w-0">{f.label}{f.scoped && <small className="ml-1.5 text-faint">chosen categories</small>}</span>
          </label>
        ))}
      </div>
    </fieldset>
  )
}
