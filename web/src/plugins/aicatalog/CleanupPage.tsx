import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, Check, FolderX, LoaderCircle, Merge, Sparkles } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { Button } from '@/components/ui/button'
import { MergeConfirm } from '@/features/admin/products/MergeInto'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { applyCleanup, cleanup, deleteEmpty, emptyQuery, lastRunQuery, undoCleanup } from './api'
import { PastePanel } from './PastePanel'
import type { CleanupPlan, CleanupPreview, DupPair } from './types'
import { useAsk } from './useAsk'

type Row = { key: string; what: ReactNode; reason: string }
const PLAN_KEYS = ['renames', 'reparents', 'merges', 'moves', 'uncategorized', 'names'] as const

/** Every proposed change as a ticked row, grouped into the page's four lists. */
function rows(pv: CleanupPreview) {
  const cat = (id: number) => (id ? pv.categories[id] ?? `#${id}` : 'top level')
  const prod = (id: number) => pv.products[id] ?? `#${id}`
  const known = new Set(Object.values(pv.categories))
  const p = pv.plan
  const arrow = (a: ReactNode, b: ReactNode) => <><span className="text-muted-foreground">{a}</span> <ArrowRight size={13} className="inline text-faint" /> <b>{b}</b></>
  return {
    categories: [
      ...(p.renames ?? []).map((o, i): Row => ({ key: `renames:${i}`, what: <>Rename {arrow(cat(o.id), o.to)}</>, reason: o.reason })),
      ...(p.reparents ?? []).map((o, i): Row => ({ key: `reparents:${i}`, what: o.parent_id ? <>Put <b>{cat(o.id)}</b> under <b>{cat(o.parent_id)}</b></> : <>Make <b>{cat(o.id)}</b> top level</>, reason: o.reason })),
      ...(p.merges ?? []).map((o, i): Row => ({ key: `merges:${i}`, what: <>Merge {arrow(cat(o.from), cat(o.into))}</>, reason: o.reason })),
      ...(p.moves ?? []).map((o, i): Row => ({ key: `moves:${i}`, what: <>Move <b>{prod(o.product_id)}</b> to <b>{cat(o.category_id)}</b></>, reason: o.reason })),
    ],
    uncategorized: (p.uncategorized ?? []).map((o, i): Row => ({ key: `uncategorized:${i}`, reason: o.reason,
      what: <>{arrow(prod(o.product_id), o.category)}{!known.has(o.category) && <span className="ml-1.5 rounded-pill bg-muted px-2 py-0.5 text-[11px] font-semibold text-muted-foreground">new</span>}</> })),
    names: (p.names ?? []).map((o, i): Row => ({ key: `names:${i}`, what: arrow(prod(o.product_id), o.to), reason: o.reason })),
  }
}

/** The plan with only the ticked ops (duplicates never — they are merged one by one). */
function ticked(plan: CleanupPlan, off: Set<string>): CleanupPlan {
  const out = { duplicates: null } as CleanupPlan
  for (const k of PLAN_KEYS) (out as Record<string, unknown>)[k] = (plan[k] ?? []).filter((_, i) => !off.has(`${k}:${i}`))
  return out
}

/** Apps → AI Assistant → Clean up: one AI pass over the catalogue (150 products a chunk) proposes
 * category fixes, homes for uncategorized items, names that follow the naming style and look-alike
 * pairs. Ticked changes apply in one go and undo as one; pairs open the merge confirm. */
export function CleanupPage() {
  const qc = useQueryClient()
  const { state, dispatch, ask, read } = useAsk<CleanupPreview>()
  const [offset, setOffset] = useState(0)
  const [runId, setRunId] = useState(0)
  const [off, setOff] = useState<Set<string>>(new Set())
  const [merged, setMerged] = useState<Set<number>>(new Set())
  const [merging, setMerging] = useState<DupPair | null>(null)
  const [applied, setApplied] = useState('')
  const [busy, setBusy] = useState(false)
  const refresh = () => void qc.invalidateQueries({ queryKey: ['x', 'aicatalog'] })

  function start(at: number) {
    setOffset(at)
    setOff(new Set())
    setMerged(new Set())
    setApplied('')
    ask(() => cleanup(at)).catch((e) => toast.error(errMsg(e)))
  }
  const onReply = (t: string) => void read(t, (reply) => cleanup(offset, reply))

  async function apply(pv: CleanupPreview) {
    setBusy(true)
    try {
      const r = await applyCleanup(ticked(pv.plan, off), runId)
      setRunId(r.runId)
      setApplied(r.summary)
      toast.success(r.summary)
      refresh()
      void qc.invalidateQueries({ queryKey: ['admin'] })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  const pv = state.k === 'done' ? state.result : null
  const r = pv && rows(pv)
  const count = r ? r.categories.length + r.uncategorized.length + r.names.length : 0
  const on = r ? count - off.size : 0
  const toggle = (k: string) => setOff((s) => { const n = new Set(s); if (n.has(k)) n.delete(k); else n.add(k); return n })

  return (
    <div className="flex flex-col gap-4">
      <LastRun onUndone={() => { setRunId(0); refresh() }} />

      {(state.k === 'idle' || state.k === 'asking') && (
        <section className="flex flex-col items-start gap-3 rounded-[var(--radius)] border border-border bg-card p-5">
          <span className="grid h-11 w-11 place-items-center rounded-[var(--radius-sm)] bg-brand text-brand-foreground"><Sparkles size={20} /></span>
          <b className="font-display text-lg">Tidy the catalogue</b>
          <p className="max-w-xl text-[14px] text-muted-foreground">The AI looks over your categories and products and suggests: categories to merge, rename or move · a category for every uncategorized item · names that follow your naming style · products listed twice. You tick what to keep; nothing changes until you apply.</p>
          <Button className="h-11 gap-1.5 px-5 font-bold" disabled={state.k === 'asking'} onClick={() => start(0)}>
            {state.k === 'asking' ? <><LoaderCircle size={16} className="animate-spin" /> Asking the AI…</> : <><Sparkles size={16} /> Start a clean-up</>}
          </Button>
        </section>
      )}

      {(state.k === 'awaiting' || state.k === 'reading') && (
        <section className="flex flex-col gap-3 rounded-[var(--radius)] border border-border bg-card p-4">
          <b className="text-[15px]">Ask the chatbot{offset ? ` — products ${offset + 1} onwards` : ''}</b>
          <PastePanel prompt={state.prompt} reply={state.reply} error={state.k === 'awaiting' ? state.error : undefined} reading={state.k === 'reading'}
            onType={(reply) => dispatch({ t: 'type', reply })} onReply={onReply} />
          <Button variant="ghost" className="h-9 self-start px-3" onClick={() => dispatch({ t: 'reset' })}>Cancel</Button>
        </section>
      )}

      {pv && r && (
        <>
          <p className="text-[13px] font-semibold text-muted-foreground">
            {pv.chunks > 1 ? `Part ${pv.chunk} of ${pv.chunks} · ` : ''}{count} change{count === 1 ? '' : 's'} suggested{pv.duplicates.length ? ` · ${pv.duplicates.length} possible duplicate${pv.duplicates.length === 1 ? '' : 's'}` : ''}
          </p>
          <List title="Categories" rows={r.categories} off={off} toggle={toggle} locked={!!applied} empty="Your categories look fine." />
          <List title="Uncategorized" rows={r.uncategorized} off={off} toggle={toggle} locked={!!applied} empty="Nothing is uncategorized." />
          <List title="Names" rows={r.names} off={off} toggle={toggle} locked={!!applied} empty="No names to change — set a naming style in Settings to tidy names." />
          <div className="flex flex-wrap items-center gap-2 rounded-[var(--radius)] border border-[color-mix(in_srgb,var(--brand)_35%,var(--border))] bg-[color-mix(in_srgb,var(--brand)_6%,var(--card))] p-3">
            {applied ? (
              <>
                <span className="flex min-w-0 flex-1 items-center gap-1.5 text-[14px] font-semibold"><Check size={16} className="flex-none text-brand" /> {applied}</span>
                <Button variant={pv.next >= 0 ? 'ghost' : 'outline'} className="h-10 px-4" onClick={() => { dispatch({ t: 'reset' }); setRunId(0) }}>{pv.next >= 0 ? 'Stop here' : 'Done'}</Button>
                {pv.next >= 0 && <Button className="h-10 px-4 font-bold" onClick={() => start(pv.next)}>Next part ({pv.chunk + 1} of {pv.chunks})</Button>}
              </>
            ) : (
              <>
                <Button variant="ghost" className="h-10 px-3" onClick={() => dispatch({ t: 'reset' })}>Cancel</Button>
                <span className="flex-1" />
                {pv.next >= 0 && <Button variant="outline" className="h-10 px-3" onClick={() => start(pv.next)}>Skip this part</Button>}
                <Button className="h-10 px-5 font-bold" disabled={busy || on === 0} onClick={() => apply(pv)}>
                  {busy && <LoaderCircle size={15} className="animate-spin" />} Apply {on} ticked
                </Button>
              </>
            )}
          </div>
          <section aria-label="Duplicates" className="rounded-[var(--radius)] border border-border bg-card">
            <h3 className="border-b border-border px-4 py-3 text-[13px] font-bold uppercase tracking-wide text-faint">Duplicates</h3>
            {pv.duplicates.length === 0 ? <p className="px-4 py-3 text-[13px] text-muted-foreground">No look-alike products found.</p> : pv.duplicates.map((d) => (
              <div key={`${d.keep.id}-${d.drop.id}`} className="flex flex-wrap items-center gap-x-3 gap-y-2 border-b border-border px-4 py-3 last:border-b-0">
                <span className="flex min-w-0 flex-1 basis-64 flex-col gap-0.5 text-[14px]">
                  <span><b>{d.keep.name}</b> <span className="text-[12px] text-faint">{d.keep.sales} sale{d.keep.sales === 1 ? '' : 's'} · keep</span></span>
                  <span className="text-muted-foreground">{d.drop.name} <span className="text-[12px] text-faint">{d.drop.sales} sale{d.drop.sales === 1 ? '' : 's'}</span></span>
                  {d.reason && <small className="text-[12px] text-faint">{d.reason}</small>}
                </span>
                {merged.has(d.drop.id) ? <span className="flex items-center gap-1 text-[13px] font-semibold text-brand"><Check size={14} /> Merged</span>
                  : <Button variant="outline" className="h-9 gap-1.5 px-3" onClick={() => setMerging(d)}><Merge size={14} /> Merge…</Button>}
              </div>
            ))}
          </section>
        </>
      )}

      {merging && (
        <MergeConfirm from={{ id: merging.drop.id, name: merging.drop.name }} into={{ id: merging.keep.id, name: merging.keep.name }}
          onClose={() => setMerging(null)} onDone={() => setMerged((s) => new Set(s).add(merging.drop.id))} />
      )}

      <EmptyCategories />
    </div>
  )
}

function List({ title, rows, off, toggle, locked, empty }: { title: string; rows: Row[]; off: Set<string>; toggle: (k: string) => void; locked: boolean; empty: string }) {
  return (
    <section aria-label={title} className="rounded-[var(--radius)] border border-border bg-card">
      <h3 className="border-b border-border px-4 py-3 text-[13px] font-bold uppercase tracking-wide text-faint">{title}{rows.length ? ` · ${rows.length}` : ''}</h3>
      {rows.length === 0 ? <p className="px-4 py-3 text-[13px] text-muted-foreground">{empty}</p> : rows.map((r) => (
        <label key={r.key} className={cn('flex cursor-pointer items-start gap-3 border-b border-border px-4 py-3 last:border-b-0', off.has(r.key) && 'opacity-55')}>
          <input type="checkbox" checked={!off.has(r.key)} disabled={locked} onChange={() => toggle(r.key)} className="mt-1 h-4 w-4 flex-none accent-[var(--brand)]" />
          <span className="flex min-w-0 flex-col gap-0.5 text-[14px]">
            <span className="break-words">{r.what}</span>
            {r.reason && <small className="text-[12px] text-faint">{r.reason}</small>}
          </span>
        </label>
      ))}
    </section>
  )
}

function LastRun({ onUndone }: { onUndone: () => void }) {
  const run = useQuery(lastRunQuery).data?.run
  if (!run) return null
  return (
    <div className="flex flex-wrap items-center gap-3 rounded-[var(--radius)] border border-border bg-muted px-4 py-3">
      <span className="min-w-0 flex-1 text-[13px]"><b>Last clean-up:</b> <span className="text-muted-foreground">{run.summary}</span></span>
      <ConfirmButton plain label="Undo last clean-up" title="Undo the last clean-up?" confirm="Undo" className="h-9 gap-1.5 border border-border bg-card px-3"
        body="Puts categories, products and names back the way they were before it. Product merges are not undone."
        onConfirm={async () => {
          try {
            toast.success(`Undone: ${(await undoCleanup()).summary}`)
            onUndone()
          } catch (e) {
            toast.error(errMsg(e))
          }
        }} />
    </div>
  )
}

function EmptyCategories() {
  const qc = useQueryClient()
  const list = useQuery(emptyQuery).data?.categories ?? []
  if (!list.length) return null
  return (
    <section aria-label="Empty categories" className="flex flex-col gap-3 rounded-[var(--radius)] border border-border bg-card p-4">
      <b className="flex items-center gap-2 text-[14px]"><FolderX size={16} className="text-muted-foreground" /> {list.length} empty categor{list.length === 1 ? 'y' : 'ies'}</b>
      <div className="flex flex-wrap gap-1.5">{list.map((c) => <span key={c.id} className="rounded-pill bg-muted px-2.5 py-1 text-[12px] font-semibold text-muted-foreground">{c.name}</span>)}</div>
      <ConfirmButton label="Delete empty categories" title={`Delete ${list.length} empty categor${list.length === 1 ? 'y' : 'ies'}?`} confirm="Delete" className="h-9 self-start px-4"
        body="They hold no products and no sub-categories."
        onConfirm={async () => {
          try {
            const n = (await deleteEmpty(list.map((c) => c.id))).deleted
            toast.success(`${n} deleted`)
            void qc.invalidateQueries({ queryKey: ['x', 'aicatalog'] })
            void qc.invalidateQueries({ queryKey: ['admin', 'categories'] })
          } catch (e) {
            toast.error(errMsg(e))
          }
        }} />
    </section>
  )
}
