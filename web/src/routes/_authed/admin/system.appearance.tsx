import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, notFound } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { applyAppearance } from '@/theme/appearance'
import { PALETTES } from '@/theme/palettes'
import type { Appearance } from '@/api/types.gen'

export const Route = createFileRoute('/_authed/admin/system/appearance')({
  beforeLoad: ({ context }) => {
    if (!context.me.flags.isSystem) throw notFound()
  },
  component: SystemAppearance,
})

const RADII = ['sharp', 'rounded', 'round'] as const

function appearanceQuery() {
  return { queryKey: ['system', 'appearance'], queryFn: () => api<Appearance>('/system/appearance') }
}

function Segmented<T extends string>({
  options,
  value,
  onChange,
  label,
}: {
  options: readonly T[]
  value: T
  onChange: (v: T) => void
  label: string
}) {
  return (
    <div role="group" aria-label={label} className="flex rounded-full border border-border bg-muted p-1">
      {options.map((opt) => (
        <button
          key={opt}
          type="button"
          aria-pressed={value === opt}
          onClick={() => onChange(opt)}
          className="flex-1 rounded-full px-3 py-1.5 text-sm font-semibold capitalize text-muted-foreground aria-pressed:bg-card aria-pressed:text-foreground aria-pressed:shadow-[var(--shadow)]"
        >
          {opt}
        </button>
      ))}
    </div>
  )
}

function SystemAppearance() {
  const { data } = useQuery(appearanceQuery())
  const queryClient = useQueryClient()
  const [form, setForm] = useState<Appearance | null>(null)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (data && !form) setForm(data)
  }, [data, form])

  // Unsaved live-preview changes must not outlive the page: re-apply the saved look on leave.
  const saved = useRef(data)
  saved.current = queryClient.getQueryData<Appearance>(['system', 'appearance']) ?? data
  useEffect(() => () => { if (saved.current) applyAppearance(saved.current) }, [])

  const update = (patch: Partial<Appearance>) => {
    setForm((prev) => {
      if (!prev) return prev
      const next = { ...prev, ...patch }
      // Half-typed hex ("#0f") would feed an invalid colour into the tokens; preview only valid ones.
      if (next.brandColor === '' || /^#[0-9a-fA-F]{6}$/.test(next.brandColor)) applyAppearance(next)
      return next
    })
  }

  const handleSave = async () => {
    if (!form) return
    setSaving(true)
    try {
      const saved = await api<Appearance>('/system/appearance', { method: 'PUT', json: form })
      setForm(saved)
      applyAppearance(saved)
      queryClient.setQueryData(['system', 'appearance'], saved)
      queryClient.removeQueries({ queryKey: ['me'] })
      toast.success('Saved')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Something went wrong')
    } finally {
      setSaving(false)
    }
  }

  if (!form) return null

  return (
    <div className="mx-auto flex max-w-2xl flex-col gap-6">
      <div>
        <h1 className="font-display text-2xl font-bold tracking-tight">Appearance</h1>
        <p className="text-muted-foreground">Locked for the whole shop. Changes preview live as you make them.</p>
      </div>

      <section className="flex flex-col gap-2">
        <span className="text-xs font-bold text-muted-foreground">Palette</span>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {PALETTES.map((p) => (
            <button
              key={p.id}
              type="button"
              aria-pressed={form.palette === p.id}
              onClick={() => update({ palette: p.id })}
              className="flex flex-col gap-2 rounded-[var(--radius)] border border-border p-3 text-left aria-pressed:ring-2 aria-pressed:ring-brand"
            >
              <span className="text-sm font-semibold">{p.name}</span>
              <div className="flex gap-2">
                <span className="flex h-10 flex-1 items-start justify-end rounded-lg p-1.5" style={{ background: p.swatch.light }}>
                  <span className="h-2.5 w-2.5 rounded-full" style={{ background: p.swatch.brand }} />
                </span>
                <span className="flex h-10 flex-1 items-start justify-end rounded-lg p-1.5" style={{ background: p.swatch.dark }}>
                  <span className="h-2.5 w-2.5 rounded-full" style={{ background: p.swatch.brand }} />
                </span>
              </div>
            </button>
          ))}
        </div>
      </section>

      <section className="flex flex-col gap-1.5">
        <label htmlFor="brand-colour" className="text-xs font-bold text-muted-foreground">
          Brand colour
        </label>
        <div className="flex items-center gap-2">
          <input
            type="color"
            aria-hidden="true"
            tabIndex={-1}
            value={/^#[0-9a-fA-F]{6}$/.test(form.brandColor) ? form.brandColor : '#000000'}
            onChange={(e) => update({ brandColor: e.target.value })}
            className="h-9 w-9 shrink-0 cursor-pointer rounded-lg border border-border bg-transparent p-0.5"
          />
          <input
            id="brand-colour"
            type="text"
            placeholder="#rrggbb (theme default)"
            value={form.brandColor}
            onChange={(e) => update({ brandColor: e.target.value })}
            className="h-9 flex-1 rounded-lg border border-border bg-card px-3 text-sm outline-none focus:border-brand"
          />
          <button
            type="button"
            onClick={() => update({ brandColor: '' })}
            className="h-9 shrink-0 rounded-lg border border-border bg-muted px-3 text-sm font-semibold"
          >
            Use theme default
          </button>
        </div>
      </section>

      <section className="flex flex-col gap-1.5 sm:max-w-sm">
        <span className="text-xs font-bold text-muted-foreground">Radius</span>
        <Segmented label="Radius" options={RADII} value={form.radius as (typeof RADII)[number]} onChange={(v) => update({ radius: v })} />
      </section>

      <button
        type="button"
        onClick={handleSave}
        disabled={saving}
        className="h-12 self-start rounded-[var(--radius-sm)] bg-brand px-6 font-semibold text-brand-foreground shadow-[var(--glow)] disabled:opacity-60"
      >
        Save
      </button>
    </div>
  )
}
