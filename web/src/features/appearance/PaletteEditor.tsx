import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useBlocker } from '@tanstack/react-router'
import { ArrowLeft, RotateCcw } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { toast } from 'sonner'
import type { Appearance } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'
import { errMsg } from '@/features/till/api'
import { reloadPaletteCss } from '@/theme/appearance'
import { EditorControls } from './EditorControls'
import { endDraft, previewMode, startDraft, updateDraft } from './draft'
import { buildPalette, builtinDoc, paletteId, parsePalette, type PaletteDoc } from './paletteCss'
import { PreviewStrip } from './PreviewStrip'
import { appearanceQuery, paletteQuery, palettesQuery, savePalette } from './queries'

type Mode = 'light' | 'dark'

function shownMode(): Mode {
  const t = document.documentElement.dataset.theme
  if (t === 'light' || t === 'dark') return t
  return matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

/** Edits one custom palette with the page as its live preview (spec §4). */
export function PaletteEditor({ id, onDone }: { id: number; onDone: () => void }) {
  const { data } = useQuery(paletteQuery(id))
  return data ? <Editor key={id} id={id} initial={parsePalette(data.css)} onDone={onDone} /> : null
}

function Editor({ id, initial, onDone }: { id: number; initial: PaletteDoc; onDone: () => void }) {
  const queryClient = useQueryClient()
  const pid = paletteId(id)
  const [doc, setDoc] = useState(initial)
  const [savedCss, setSavedCss] = useState(() => buildPalette(initial, pid))
  const [mode, setMode] = useState<Mode>(shownMode)
  const [leaving, setLeaving] = useState(false)
  const [busy, setBusy] = useState(false)
  const css = useMemo(() => buildPalette(doc, pid), [doc, pid])
  const dirty = css !== savedCss
  const blocker = useBlocker({ shouldBlockFn: () => dirty, enableBeforeUnload: () => dirty, withResolver: true })

  const appearance = () => queryClient.getQueryData<Appearance>(appearanceQuery.queryKey)
  const restore = useRef(() => { const a = appearance(); if (a) endDraft(a) })
  useEffect(() => {
    startDraft(pid)
    const end = restore.current
    return () => end()
  }, [pid])
  useEffect(() => updateDraft(css), [css])
  useEffect(() => previewMode(mode), [mode])

  const save = async () => {
    setBusy(true)
    try {
      await savePalette(id, doc)
      setSavedCss(css)
      await queryClient.invalidateQueries({ queryKey: palettesQuery.queryKey })
      if (appearance()?.palette === pid) reloadPaletteCss()
      toast.success('Saved')
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  const reset = () => {
    const base = builtinDoc(doc.base)
    setDoc({ ...doc, light: base.light, dark: base.dark })
  }

  return (
    <div className="mx-auto flex max-w-6xl flex-col gap-4">
      <div className="sticky top-0 z-10 -mx-2 flex flex-wrap items-center gap-2 rounded-[var(--radius)] border border-border bg-bar px-2 py-2 shadow-[var(--shadow)] [backdrop-filter:var(--glass)]">
        <button type="button" onClick={() => (dirty ? setLeaving(true) : onDone())} aria-label="Back to palettes"
          className="grid size-10 shrink-0 place-items-center rounded-pill hover:bg-muted"><ArrowLeft className="size-5" /></button>
        <input aria-label="Palette name" value={doc.name} onChange={(e) => setDoc({ ...doc, name: e.target.value })}
          className="h-10 min-w-0 flex-1 rounded-[var(--radius-sm)] bg-transparent px-2 font-display text-lg font-bold outline-none focus:bg-muted sm:max-w-xs" />
        <div role="group" aria-label="Edit mode" className="flex rounded-pill bg-muted p-1 text-sm font-semibold">
          {(['light', 'dark'] as const).map((m) => (
            <button key={m} type="button" aria-pressed={mode === m} onClick={() => setMode(m)}
              className="rounded-pill px-3 py-1 capitalize text-muted-foreground aria-pressed:bg-selected aria-pressed:text-selected-foreground">{m}</button>
          ))}
        </div>
        <div className="ml-auto flex gap-2">
          <button type="button" onClick={reset} className="flex h-10 items-center gap-1.5 rounded-pill border border-border px-3 text-sm font-semibold hover:bg-muted">
            <RotateCcw className="size-4" /> Reset to base
          </button>
          <button type="button" onClick={save} disabled={busy || !dirty}
            className="h-10 rounded-pill bg-brand px-5 text-sm font-bold text-brand-foreground shadow-[var(--glow)] disabled:opacity-50">Save</button>
        </div>
      </div>

      <div className="grid grid-cols-[minmax(0,1fr)] gap-4 lg:grid-cols-[minmax(0,1fr)_380px]">
        <div className="order-2 lg:order-1"><EditorControls doc={doc} mode={mode} onChange={setDoc} /></div>
        <div className="order-1 lg:order-2"><div className="lg:sticky lg:top-20"><PreviewStrip /></div></div>
      </div>

      <Dialog open={leaving || blocker.status === 'blocked'} onOpenChange={(o) => { if (!o) { setLeaving(false); blocker.reset?.() } }}>
        <DialogContent className="gap-3 p-5 sm:max-w-sm">
          <DialogTitle className="font-display text-lg font-bold">Leave without saving?</DialogTitle>
          <DialogDescription>Your changes to {doc.name} will be lost.</DialogDescription>
          <div className="mt-2 flex justify-end gap-2">
            <Button type="button" variant="ghost" className="h-10 px-4" onClick={() => { setLeaving(false); blocker.reset?.() }}>Stay</Button>
            <Button type="button" className="h-10 bg-danger px-4 font-bold text-danger-foreground hover:bg-danger/90"
              onClick={() => { if (leaving) { setLeaving(false); onDone() } else blocker.proceed?.() }}>Leave</Button>
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}
