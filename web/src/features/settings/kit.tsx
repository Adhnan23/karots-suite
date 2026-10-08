import { queryOptions, useQueryClient } from '@tanstack/react-query'
import { useRouter } from '@tanstack/react-router'
import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { SettingsPage, ShopSettings } from '@/api/types.gen'
import { SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'

export const settingsQuery = queryOptions({ queryKey: ['admin', 'settings'], queryFn: () => api<SettingsPage>('/admin/settings') })

/** After any change: the page, /me (stock-take, tax, lock timer, shop name) and
 * the till's cached config (symbol, receipt header) all read settings. */
export function useSettingsSaved() {
  const qc = useQueryClient()
  const router = useRouter()
  return (page: SettingsPage) => {
    // A save's answer leaves out the printers (slow to find): keep the list.
    qc.setQueryData(settingsQuery.queryKey, (old?: SettingsPage) => ({ ...page, printers: page.printers ?? old?.printers ?? [] }))
    void qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'admin' || q.queryKey[1] !== 'settings' })
    // The menu and pages read /me from the route context, set once on load:
    // fetch it fresh and re-run the route so stock-take / tax pages show at once.
    void qc.refetchQueries({ queryKey: ['me'], type: 'all' }).then(() => router.invalidate())
  }
}

type Draft = { v: ShopSettings; set: <K extends keyof ShopSettings>(k: K, val: ShopSettings[K]) => void }

/** One settings card saved on its own (a phone can't hold the whole form): it
 * edits a copy and Save sends the saved settings with only this card's fields. */
export function Section({ title, sub, saved, keys, children }: {
  title: string
  sub: string
  saved: ShopSettings
  keys: (keyof ShopSettings)[]
  children: (d: Draft) => ReactNode
}) {
  const [edits, setEdits] = useState<Partial<ShopSettings>>({})
  const [busy, setBusy] = useState(false)
  const done = useSettingsSaved()
  const v = { ...saved, ...edits }
  const dirty = keys.some((k) => k in edits && edits[k] !== saved[k])
  const set: Draft['set'] = (k, val) => setEdits((e) => ({ ...e, [k]: val }))
  const save = async () => {
    setBusy(true)
    try {
      const body = { ...saved }
      for (const k of keys) (body as Record<string, unknown>)[k] = v[k]
      done(await api<SettingsPage>('/admin/settings', { method: 'PUT', json: body }))
      setEdits({})
      toast.success(`${title} saved`)
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section aria-label={title} className={cn(SURFACE, 'min-w-0 p-4 md:p-5')}>
      <div className="mb-3">
        <h2 className="font-display text-lg font-bold">{title}</h2>
        <p className="text-[13px] text-muted-foreground">{sub}</p>
      </div>
      <div className="grid gap-3">{children({ v, set })}</div>
      {dirty && (
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="ghost" className="h-10 px-4" disabled={busy} onClick={() => setEdits({})}>Undo</Button>
          <Button className="h-10 px-5 font-bold" disabled={busy} onClick={() => void save()}>{busy ? 'Saving…' : 'Save'}</Button>
        </div>
      )}
    </section>
  )
}

/** Two to four fixed choices as a pill row (receipt width, sale type, pin…). */
export function Segmented<T extends string | number>({ label, value, options, onChange }: {
  label: string
  value: T
  options: { value: T; label: string }[]
  onChange: (v: T) => void
}) {
  return (
    <div className="flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground">
      {label}
      <div role="radiogroup" aria-label={label} className="flex flex-wrap gap-1.5">
        {options.map((o) => (
          <button key={String(o.value)} type="button" role="radio" aria-checked={value === o.value} onClick={() => onChange(o.value)}
            className="h-10 rounded-full border border-border bg-muted px-4 text-sm font-semibold text-foreground aria-checked:border-selected aria-checked:bg-selected aria-checked:text-selected-foreground">
            {o.label}
          </button>
        ))}
      </div>
    </div>
  )
}
