import { useQueries, useQuery, useQueryClient } from '@tanstack/react-query'
import { Copy, Download, Pencil, Upload } from 'lucide-react'
import { useRef } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { Appearance } from '@/api/types.gen'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { errMsg } from '@/features/till/api'
import { applyAppearance } from '@/theme/appearance'
import { PALETTES, THEME } from '@/theme/palettes'
import { buildPalette, builtinDoc, paletteId, parsePalette, type PaletteDoc } from './paletteCss'
import { createPalette, paletteQuery, palettesQuery } from './queries'

const LEGACY_RADIUS: Record<string, [string, string]> = { sharp: ['6px', '4px'], round: ['22px', '14px'] }

type Entry = { id: string; customId?: number; name: string; doc?: PaletteDoc; swatch: { light: string; dark: string; brand: string } }

/** Every palette of this theme: Use, Duplicate, and for custom ones Edit / Export / Delete. */
export function PalettePicker({ appearance, onSaved, onEdit }: {
  appearance: Appearance
  onSaved: (a: Appearance) => void
  onEdit: (customId: number) => void
}) {
  const queryClient = useQueryClient()
  const fileRef = useRef<HTMLInputElement>(null)
  const { data: list = [] } = useQuery(palettesQuery)
  const docs = useQueries({ queries: list.map((p) => ({ ...paletteQuery(p.id), select: (x: { css: string }) => parsePalette(x.css) })) })

  const entries: Entry[] = [
    ...PALETTES.map((p) => ({ id: p.id as string, name: p.name, swatch: p.swatch })),
    ...list.map((p, i) => {
      const doc = docs[i]?.data
      return {
        id: paletteId(p.id), customId: p.id, name: p.name, doc,
        swatch: { light: doc?.light['--background'] ?? 'transparent', dark: doc?.dark['--background'] ?? 'transparent', brand: doc?.light['--brand'] ?? 'transparent' },
      }
    }),
  ]

  const refresh = () => queryClient.invalidateQueries({ queryKey: palettesQuery.queryKey })

  const use = async (palette: string) => {
    try {
      const saved = await api<Appearance>('/system/appearance', { method: 'PUT', json: { ...appearance, palette } })
      applyAppearance(saved)
      queryClient.setQueryData(['system', 'appearance'], saved)
      queryClient.removeQueries({ queryKey: ['me'] })
      onSaved(saved)
    } catch (e) {
      toast.error(errMsg(e))
    }
  }

  const duplicate = async (e: Entry) => {
    const doc = e.doc ? structuredClone(e.doc) : builtinDoc(e.id)
    // A built-in still wearing the old brand / radius override starts from what's on screen.
    if (!e.customId && appearance.palette === e.id) {
      const r = LEGACY_RADIUS[appearance.radius]
      for (const mode of [doc.light, doc.dark]) {
        if (appearance.brandColor) mode['--brand'] = appearance.brandColor
        if (r) [mode['--radius'], mode['--radius-sm']] = r
      }
    }
    try {
      const made = await createPalette({ ...doc, name: `${e.name} copy` })
      await refresh()
      return made
    } catch (err) {
      toast.error(errMsg(err))
    }
  }

  const exportCss = (e: Entry) => {
    if (!e.doc || !e.customId) return
    const url = URL.createObjectURL(new Blob([buildPalette(e.doc, paletteId(e.customId))], { type: 'text/css' }))
    const a = Object.assign(document.createElement('a'), { href: url, download: `${e.name}.css` })
    a.click()
    URL.revokeObjectURL(url)
  }

  const importCss = async (file: File) => {
    const doc = parsePalette(await file.text())
    if (doc.theme !== THEME) return toast.error('This palette is for another theme')
    try {
      await createPalette({ ...doc, name: doc.name || file.name.replace(/\.css$/i, '') })
      await refresh()
    } catch (err) {
      toast.error(errMsg(err))
    }
  }

  return (
    <section className="flex flex-col gap-3">
      <div className="flex items-end justify-between gap-3">
        <div>
          <h2 className="font-display text-lg font-bold">Palette</h2>
          <p className="text-sm text-muted-foreground">Colours, material, shape and motion for the whole shop. Duplicate one to make your own.</p>
        </div>
        <button type="button" onClick={() => fileRef.current?.click()}
          className="flex h-10 shrink-0 items-center gap-1.5 rounded-pill border border-border bg-card px-3.5 text-sm font-semibold hover:bg-muted">
          <Upload className="size-4" /> Import
        </button>
        <input ref={fileRef} type="file" accept=".css,text/css" aria-label="Import a palette" className="hidden"
          onChange={(ev) => { const f = ev.target.files?.[0]; ev.target.value = ''; if (f) void importCss(f) }} />
      </div>
      <div className="-mx-1 flex items-start gap-3 overflow-x-auto px-1 pb-1 sm:mx-0 sm:grid sm:grid-cols-2 sm:overflow-visible sm:px-0 lg:grid-cols-3">
        {entries.map((e) => {
          const active = appearance.palette === e.id
          return (
            <div key={e.id} role="group" aria-label={e.name} data-active={active || undefined}
              className="flex w-[264px] shrink-0 flex-col gap-3 rounded-[var(--radius)] border-2 sm:w-auto sm:min-w-0 border-transparent bg-card p-3 shadow-[var(--shadow)] data-active:border-brand">
              <div className="flex items-center gap-2">
                <span className="min-w-0 flex-1 truncate font-semibold">{e.name}</span>
                <span className="rounded-pill bg-muted px-2 py-0.5 text-[11px] font-bold text-muted-foreground">{e.customId ? 'Custom' : 'Built-in'}</span>
              </div>
              <div className="flex gap-2">
                {[e.swatch.light, e.swatch.dark].map((bg, i) => (
                  <span key={i} className="flex h-12 flex-1 items-end justify-end rounded-[var(--radius-sm)] border border-border p-1.5" style={{ background: bg }}>
                    <span className="h-3 w-8 rounded-pill" style={{ background: e.swatch.brand }} />
                  </span>
                ))}
              </div>
              <div className="flex flex-wrap items-center gap-1.5 text-sm">
                {active
                  ? <span className="flex h-9 items-center rounded-pill bg-brand-soft px-3 font-bold">In use</span>
                  : <button type="button" onClick={() => use(e.id)} className="h-9 rounded-pill bg-selected px-3.5 font-bold text-selected-foreground">Use</button>}
                <Act label="Duplicate" icon={<Copy className="size-3.5" />} onClick={async () => { const p = await duplicate(e); if (p) onEdit(p.id) }} />
                {e.customId && <>
                  <Act label="Edit" icon={<Pencil className="size-3.5" />} onClick={() => onEdit(e.customId!)} />
                  <Act label="Export" icon={<Download className="size-3.5" />} onClick={() => exportCss(e)} />
                  {!active && (
                    <ConfirmButton label="Delete" title={`Delete ${e.name}?`} body="The palette is gone for good. Export it first if you may want it back."
                      confirm="Delete palette" className="h-9 rounded-pill px-3 text-sm font-semibold"
                      onConfirm={async () => { await api(`/system/palettes/${e.customId}`, { method: 'DELETE' }); await refresh() }} />
                  )}
                </>}
              </div>
            </div>
          )
        })}
      </div>
    </section>
  )
}

function Act({ label, icon, onClick }: { label: string; icon: React.ReactNode; onClick: () => void }) {
  return (
    <button type="button" onClick={onClick} className="flex h-9 items-center gap-1 rounded-pill border border-border px-3 font-semibold hover:bg-muted">
      {icon}{label}
    </button>
  )
}
