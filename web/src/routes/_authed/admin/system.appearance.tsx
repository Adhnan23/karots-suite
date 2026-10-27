import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, notFound } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import { PaletteEditor } from '@/features/appearance/PaletteEditor'
import { PalettePicker } from '@/features/appearance/PalettePicker'
import { Fold } from '@/features/appearance/Fold'
import { LabelStyles } from '@/features/appearance/LabelStyles'
import { appearanceQuery } from '@/features/appearance/queries'
import { ReceiptStyles } from '@/features/appearance/ReceiptStyles'
import { labelKinds } from '@/features/print/labels/registry'
import { STYLES } from '@/features/print/styles'
import { configQuery } from '@/features/till/api'
import { PALETTES } from '@/theme/palettes'
import { applyAppearance } from '@/theme/appearance'
import type { Appearance } from '@/api/types.gen'

export const Route = createFileRoute('/_authed/admin/system/appearance')({
  beforeLoad: ({ context }) => {
    if (!context.me.flags.isSystem) throw notFound()
  },
  component: SystemAppearance,
})

/** "Price Tag · Dose label: Clear": each kind's chosen style (Classic kinds left out unless all are). */
function labelHint(styles: Record<string, string>) {
  const picks = labelKinds().map((k) => ({ k, s: k.styles.find((x) => x.key === styles[k.kind]) })).filter((x) => x.s && x.s.key !== 'classic')
  if (!picks.length) return 'Classic'
  return picks.map(({ k, s }, i) => (i === 0 && k.kind === 'price' ? s!.label : `${k.label}: ${s!.label}`)).join(' · ')
}

function SystemAppearance() {
  const { data } = useQuery(appearanceQuery)
  const queryClient = useQueryClient()
  const [form, setForm] = useState<Appearance | null>(null)
  const [saving, setSaving] = useState(false)
  const [editing, setEditing] = useState<number | null>(null)

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
      return { ...prev, ...patch }
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
      // The next print reads the saved style from the till config.
      await queryClient.invalidateQueries({ queryKey: configQuery.queryKey })
      toast.success('Saved')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Something went wrong')
    } finally {
      setSaving(false)
    }
  }

  if (!form) return null
  if (editing !== null) return <PaletteEditor id={editing} onDone={() => setEditing(null)} />

  return (
    <div className="mx-auto flex max-w-5xl flex-col gap-6">
      <div>
        <h1 className="font-display text-2xl font-bold tracking-tight">Appearance</h1>
        <p className="text-muted-foreground">Locked for the whole shop.</p>
      </div>

      <Fold title="Palette" hint={PALETTES.find((p) => p.id === form.palette)?.name ?? 'Custom'} open remember="appearance.palette">
        <PalettePicker appearance={form} onSaved={setForm} onEdit={setEditing} />
      </Fold>

      <Fold title="Receipt style" hint={STYLES.find((s) => s.key === form.receiptStyle)?.label ?? 'Classic'} remember="appearance.receipt">
        <ReceiptStyles value={form.receiptStyle} onChange={(receiptStyle) => update({ receiptStyle })} />
      </Fold>

      <Fold title="Label styles" hint={labelHint(form.labelStyles ?? {})} remember="appearance.labels">
        <LabelStyles value={form.labelStyles ?? {}} onChange={(labelStyles) => update({ labelStyles })} />
      </Fold>

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
