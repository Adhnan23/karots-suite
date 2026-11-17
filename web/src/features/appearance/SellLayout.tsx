import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { api, ApiError } from '@/api/client'
import type { SellLayout as Layout, SellLayoutPage } from '@/api/types.gen'
import { configQuery } from '@/features/till/api'
import { ARRANGEMENTS, nearestKey, sizeKey, todayLayout } from '@/features/till/layout/arrangements'
import { endTrial, readTrial, startTrial } from '@/features/till/layout/trial'
import { cn } from '@/lib/utils'
import { innerOf, startShares } from './layoutDrag'
import { AREA_NAME, LayoutPreview } from './LayoutPreview'
import { Seg } from './ReceiptStyles'

export const sellLayoutQuery = { queryKey: ['system', 'sell-layout'], queryFn: () => api<SellLayoutPage>('/system/sell-layout') }

const CARDS = [{ label: 'Large', key: 'L' }, { label: 'Medium', key: 'M' }, { label: 'Small', key: 'S' }, { label: 'List', key: 'T' }] as const
const TEXTS = [{ label: 'Smaller', key: '90' }, { label: 'Normal', key: '100' }, { label: 'Larger', key: '110' }] as const

/** Starting points: one tap sets a whole layout for the size, then drag to fine-tune. */
const PRESETS = [
  { label: 'Scanner counter', note: 'search only, a wide cart, larger text', make: (w: number) => ({ arr: 'CY', card: 'T', text: 110, split: 0, fr: startShares('CY', w) }) },
  { label: 'Touch counter', note: 'big product cards, as it always was', make: (w: number) => ({ ...todayLayout(w) }) },
  { label: 'Square screen', note: 'products over pay, cart full height', make: (w: number) => ({ arr: 'PY_C', card: 'M', text: 100, split: 0.6, fr: startShares('PY_C', w) }) },
] as const

type Size = { key: string; w: number; h: number; users: string[] }
const parse = (key: string) => key.split('x').map(Number) as [number, number]

/** A small picture of an arrangement: its columns, a stacked one split in two. */
function Picture({ cols, strip }: { cols: (keyof typeof AREA_NAME)[][]; strip?: boolean }) {
  const tone = { cat: 'bg-brand-soft', cart: 'bg-foreground', pay: 'bg-brand' }
  return (
    <span className="flex h-12 w-20 flex-col gap-0.5 rounded-sm border border-border bg-card p-0.5">
      {strip && <span className={cn('h-1.5 shrink-0 rounded-[calc(var(--radius-sm)/4)]', tone.cat)} />}
      <span className="flex min-h-0 flex-1 gap-0.5">
        {cols.map((c, i) => (
          <span key={i} className="flex flex-1 flex-col gap-0.5">
            {c.map((a) => <span key={a} className={cn('flex-1 rounded-[calc(var(--radius-sm)/4)]', tone[a])} />)}
          </span>
        ))}
      </span>
    </span>
  )
}

/** Appearance → Sell page layout (system user only): per screen size the shop's tills
 * have, the arrangement, widths, product card size and text size. Saved on its own. */
export function SellLayout() {
  const { data } = useQuery(sellLayoutQuery)
  const qc = useQueryClient()
  const [draft, setDraft] = useState<Record<string, Layout> | null>(null)
  const [sel, setSel] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const navigate = useNavigate()
  // Back from Try here: carry on with the draft that was being tried.
  const [trial] = useState(readTrial) // read once: effects may run twice
  useEffect(() => {
    if (!data || draft) return
    endTrial()
    setDraft(trial?.draft ?? data.layouts)
    if (trial) setSel(trial.sel)
  }, [data, draft, trial])
  if (!data || !draft) return <p className="text-sm text-muted-foreground">Loading…</p>

  // Sizes the tills reported, then saved ones no till has shown lately, then this screen.
  const sizes: Size[] = data.screens.map((s) => ({ key: sizeKey(s.w, s.h), w: s.w, h: s.h, users: s.users }))
  for (const key of Object.keys(draft)) if (!sizes.some((s) => s.key === key)) sizes.push({ key, w: parse(key)[0], h: parse(key)[1], users: [] })
  const hereKey = nearestKey(sizes.map((s) => s.key), window.innerWidth, window.innerHeight) ?? sizeKey(window.innerWidth, window.innerHeight)
  // A phone (system user away from the counter) isn't a till size.
  if (window.innerWidth >= 900 && !sizes.some((s) => s.key === hereKey)) sizes.push({ key: hereKey, w: window.innerWidth, h: window.innerHeight, users: [] })

  const cur = sizes.find((s) => s.key === sel) ?? sizes.find((s) => s.key === hereKey) ?? sizes[0] ?? { key: '1366x768', w: 1366, h: 768, users: [] }
  const layout = draft[cur.key] ?? todayLayout(cur.w)
  const set = (l: Layout) => setDraft({ ...draft, [cur.key]: l })
  const pick = (id: string) => {
    const fr = startShares(id, cur.w)
    if (fr) set({ ...layout, arr: id, fr, split: id.includes('_') ? (layout.split || 0.5) : 0 })
  }
  const reset = () => { const { [cur.key]: _, ...rest } = draft; setDraft(rest) }
  const arr = ARRANGEMENTS.find((a) => a.id === layout.arr) ?? ARRANGEMENTS[0]
  const inner = innerOf(arr.id, cur.w)

  const save = async () => {
    setSaving(true)
    try {
      const page = await api<SellLayoutPage>('/system/sell-layout', { method: 'PUT', json: { layouts: draft } })
      qc.setQueryData(sellLayoutQuery.queryKey, page)
      setDraft(page.layouts)
      await qc.invalidateQueries({ queryKey: configQuery.queryKey })
      toast.success('Layouts saved')
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Something went wrong')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">Each screen size the shop’s tills show the Sell page at. Tills of the same size share a layout; a size left alone looks as it always has.</p>
      <div role="group" aria-label="Screen size" className="flex flex-wrap gap-2">
        {sizes.map((s) => (
          <button key={s.key} type="button" aria-pressed={s.key === cur.key} onClick={() => setSel(s.key)}
            className="flex min-w-0 flex-col items-start rounded-[var(--radius-sm)] border border-border bg-card px-3 py-2 text-left aria-pressed:border-brand aria-pressed:shadow-[var(--shadow)]">
            <span className="flex items-center gap-1.5 font-semibold tabular-nums">
              {s.w} × {s.h}
              {draft[s.key] && <span aria-label="set" className="h-2 w-2 rounded-full bg-brand" />}
              {s.key === hereKey && <span className="rounded-pill bg-brand-soft px-2 text-xs font-bold">this screen</span>}
            </span>
            <span className="max-w-56 truncate text-xs text-muted-foreground">{s.users.length ? `used by ${s.users.join(', ')}` : 'no till seen yet'}</span>
          </button>
        ))}
      </div>

      <div className="flex flex-col gap-2">
        <span className="text-xs font-bold text-muted-foreground">Start from</span>
        <div role="group" aria-label="Start from" className="flex flex-wrap gap-2">
          {PRESETS.map((p) => {
            const l = p.make(cur.w)
            return (
              <button key={p.label} type="button" disabled={!l.fr} onClick={() => l.fr && set({ ...l, fr: l.fr })}
                title={l.fr ? undefined : 'Too narrow for this screen'}
                className="flex flex-col items-start rounded-[var(--radius-sm)] border border-border bg-card px-3 py-2 text-left disabled:opacity-40">
                <span className="font-semibold">{p.label}</span>
                <span className="text-xs text-muted-foreground">{p.note}</span>
              </button>
            )
          })}
        </div>
      </div>

      <div className="flex flex-col gap-2">
        <span className="text-xs font-bold text-muted-foreground">Arrangement</span>
        <div role="group" aria-label="Arrangement" className="flex flex-wrap gap-2">
          {ARRANGEMENTS.map((a) => {
            const fits = startShares(a.id, cur.w) !== null
            return (
              <button key={a.id} type="button" aria-label={a.label} aria-pressed={a.id === layout.arr} disabled={!fits} onClick={() => pick(a.id)}
                title={fits ? undefined : 'Too narrow for this screen'}
                className="flex w-24 flex-col items-center gap-1 rounded-[var(--radius-sm)] border-2 border-transparent p-1 text-xs font-semibold text-muted-foreground aria-pressed:border-brand aria-pressed:text-foreground disabled:opacity-40">
                <Picture cols={a.cols} strip={a.strip} />
                {a.label}
              </button>
            )
          })}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-2">
        <span className="flex flex-wrap items-center gap-2"><span className="text-xs font-bold text-muted-foreground">Product cards</span>
          <Seg label="Product cards" options={CARDS} value={layout.card as (typeof CARDS)[number]['key']} onChange={(card) => set({ ...layout, card })} /></span>
        <span className="flex flex-wrap items-center gap-2"><span className="text-xs font-bold text-muted-foreground">Text size</span>
          <Seg label="Text size" options={TEXTS} value={String(layout.text || 100) as (typeof TEXTS)[number]['key']} onChange={(t) => set({ ...layout, text: Number(t) })} /></span>
      </div>

      <LayoutPreview w={cur.w} h={cur.h} layout={layout} onChange={set} />
      <p className="text-sm text-muted-foreground">
        {arr.cols.map((c, i) => `${c.map((a) => AREA_NAME[a]).join(' over ')} ${Math.round(layout.fr[i] * inner)} px (${Math.round(layout.fr[i] * 100)}%)`).join(' · ')}
        {' '}· drag the bars to change them
      </p>

      <div className="flex flex-wrap gap-2">
        <button type="button" onClick={save} disabled={saving}
          className="h-11 rounded-[var(--radius-sm)] bg-brand px-5 font-semibold text-brand-foreground shadow-[var(--glow)] disabled:opacity-60">Save layouts</button>
        <button type="button" onClick={() => { startTrial({ layout, draft, sel: cur.key }); void navigate({ to: '/till' }) }}
          title="Open the Sell page with this layout before saving it (the real Sell page: a sale there counts)"
          className="h-11 rounded-[var(--radius-sm)] border border-border bg-card px-4 font-semibold">Try here</button>
        <button type="button" onClick={reset} disabled={!draft[cur.key]}
          className="h-11 rounded-[var(--radius-sm)] border border-border bg-card px-4 font-semibold disabled:opacity-50">Reset this size</button>
      </div>
    </div>
  )
}
