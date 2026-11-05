import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { AlertTriangle, ChevronLeft, LoaderCircle, Minus, Plus, RotateCcw, Sparkles } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { CategoryPicker } from '@/features/admin/categories/CategoryPicker'
import { ProductPicker } from '@/features/admin/products/ProductPicker'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { slots } from '../slots'
import { LastRun } from './CleanupPage'
import { answersNeed, applyBody, batchLabel, byProduct, initialTicks, jobApply, jobNext, jobRedoOver, jobsQuery, rowKey, tickAll, type JobAsk, type JobInfo, type JobPreview, type JobRow } from './jobs'
import { PastePanel } from './PastePanel'
import { jobControls } from './jobControls'
import { useAsk } from './useAsk'

const CARD = 'flex flex-col gap-3 rounded-[var(--radius)] border border-border bg-card p-4'


/** The Jobs tab: the list, or one job. */
export function JobsPage({ jobKey }: { jobKey: string }) {
  const go = useNavigate()
  const jobs = useQuery(jobsQuery).data
  const open = (key: string) => void go({ to: '/admin/x/$key/$', params: { key: 'aicatalog', _splat: key ? `jobs/${key}` : 'jobs' } })
  const job = jobs?.find((j) => j.key === jobKey)
  if (job) return <JobRun key={job.key} job={job} onBack={() => open('')} />
  const combos = slots.apps.some((a) => a.key === 'combos')
  return (
    <div className="grid gap-3 md:grid-cols-2">
      {jobs?.map((j) => (
        <button key={j.key} type="button" onClick={() => open(j.key)} className="flex flex-col items-start gap-1 rounded-[var(--radius)] border border-border bg-card p-4 text-left duration-(--dur) hover:border-selected">
          <b className="flex items-center gap-1.5 font-display text-[16px]"><Sparkles size={16} className="text-brand" /> {j.label}</b>
          <span className="text-[13.5px] text-muted-foreground">{j.desc}</span>
        </button>
      ))}
      {combos && (
        <button type="button" onClick={() => void go({ to: '/admin/x/$key/$', params: { key: 'combos', _splat: '' } })} className="flex flex-col items-start gap-1 rounded-[var(--radius)] border border-border bg-card p-4 text-left duration-(--dur) hover:border-selected">
          <b className="flex items-center gap-1.5 font-display text-[16px]"><Sparkles size={16} className="text-brand" /> Combo suggestions</b>
          <span className="text-[13.5px] text-muted-foreground">Products that go together. Opens from the Combos page, and you set the price.</span>
        </button>
      )}
    </div>
  )
}

function JobRun({ job, onBack }: { job: JobInfo; onBack: () => void }) {
  const qc = useQueryClient()
  const { state, dispatch, ask, read } = useAsk<JobPreview>()
  const [size, setSize] = useState(job.oneByOne ? 1 : 5)
  const [categoryId, setCategoryId] = useState(0)
  const [mode, setMode] = useState<'' | 'skipped' | 'redo'>('')
  const [left, setLeft] = useState(0)
  const [firstLeft, setFirstLeft] = useState(0)
  const [options, setOptions] = useState<unknown>(undefined)
  const [redo, setRedo] = useState<number[] | undefined>()
  const [ticks, setTicks] = useState<Set<string>>(new Set())
  const [edits, setEdits] = useState<Record<string, Partial<JobRow>>>({})
  const [busy, setBusy] = useState(false)
  const [skips, setSkips] = useState<{ productId: number; label: string; why: string }[]>([])
  const body = (): JobAsk => ({ size, categoryId, filter: mode === 'skipped' ? 'skipped' : '', force: mode === 'redo', ids: redo, options })
  // Every answer says how many are left, so each step can say where it stands.
  const next = async (b: JobAsk) => {
    const r = await jobNext(job.key, b)
    if (r.left !== undefined) {
      setLeft(r.left)
      setFirstLeft((f) => f || r.left || 0)
    }
    return r
  }
  const progress = batchLabel(firstLeft, left, redo?.length ?? size)
  const Controls = jobControls[job.key]

  function ask_() {
    setEdits({})
    setSkips([])
    dispatch({ t: 'reset' })
    ask(() => next(body())).catch((e) => toast.error(errMsg(e)))
  }
  function start() {
    setFirstLeft(0)
    ask_()
  }
  const onReply = (reply: string) => void read(reply, (r) => next({ ...body(), reply: r }))
  const pv = state.k === 'done' ? state.result : null
  const [seen, setSeen] = useState<JobPreview | null>(null)
  if (pv !== seen) {
    setSeen(pv)
    setTicks(pv ? initialTicks(pv.rows) : new Set())
    setEdits({})
  }
  const merged = (r: JobRow, key: string): JobRow => {
    const e = edits[key]
    return e ? { ...r, ...e, extra: { ...r.extra, ...e.extra } } : r
  }

  async function apply() {
    if (!pv) return
    setBusy(true)
    try {
      const rows = pv.rows.map((r, i) => merged(r, rowKey(r, i)))
      const r = await jobApply(job.key, applyBody({ ...pv, rows }, new Set([...ticks].filter((k) => rows.some((x, i) => rowKey(x, i) === k && !x.needs)))))
      toast.success(r.summary)
      setSkips(r.skipped.map((s) => ({ ...s, label: `${pv.names[s.productId] ?? `#${s.productId}`}, ${s.label}` })))
      setRedo(undefined)
      await qc.invalidateQueries({ queryKey: ['x', 'aicatalog'] }) // the last apply, and what's still unanswered
      await qc.invalidateQueries({ queryKey: ['admin'] })
      if (pv.remaining > 0 && !r.skipped.length) ask_()
      else dispatch({ t: 'reset' })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  const idle = state.k === 'idle' || state.k === 'asking'
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <Button variant="ghost" className="h-9 gap-1 px-2" onClick={onBack}><ChevronLeft size={16} /> All jobs</Button>
        <b className="font-display text-lg">{job.label}</b>
      </div>
      {job.warning && <p role="note" className="flex items-start gap-2 rounded-[var(--radius-sm)] border border-border bg-[color-mix(in_srgb,var(--warning)_14%,transparent)] p-3 text-[14px] font-semibold"><AlertTriangle size={18} className="mt-0.5 shrink-0 text-warning" /> {job.warning}</p>}
      {job.noAnswer && job.noAnswer.length > 0 && (
        <section aria-label="No answer yet" className={cn(CARD, 'gap-1 text-[13.5px]')}>
          <b>The chatbot wasn't sure about these. Check them by hand:</b>
          <p className="break-words text-muted-foreground">{job.noAnswer.join(' · ')}</p>
        </section>
      )}
      <LastRun label="Undo it" onUndone={() => { void qc.invalidateQueries({ queryKey: ['x', 'aicatalog'] }); void qc.invalidateQueries({ queryKey: ['admin'] }) }} />
      {skips.length > 0 && (
        <ul className={cn(CARD, 'gap-1 text-[13.5px]')} aria-label="Not applied">
          <b>These no longer fit, so they weren't applied:</b>
          {skips.map((s, i) => <li key={i} className="text-muted-foreground">{s.label}: {s.why}</li>)}
        </ul>
      )}
      {idle && (
        <section className={CARD}>
          <p className="max-w-2xl text-[14px] text-muted-foreground">{job.desc}</p>
          {Controls && <Controls job={job} value={options} onChange={setOptions} />}
          <div className="flex flex-wrap items-end gap-3">
            <div className="grid gap-1 text-[12.5px] font-semibold">How many at a time
              <span className="flex items-center gap-1">
                <Button type="button" variant="outline" size="icon" aria-label="Fewer" className="h-[var(--control-h)] w-[var(--control-h)]" onClick={() => setSize(Math.max(1, size - 1))}><Minus size={15} /></Button>
                <input aria-label="How many at a time" inputMode="numeric" value={size} onChange={(e) => setSize(Math.min(50, Math.max(1, Number(e.target.value.replace(/\D/g, '')) || 1)))} className={cn(INPUT, 'w-16 text-center tabular-nums')} />
                <Button type="button" variant="outline" size="icon" aria-label="More" className="h-[var(--control-h)] w-[var(--control-h)]" onClick={() => setSize(Math.min(50, size + 1))}><Plus size={15} /></Button>
              </span>
            </div>
            <div className="grid min-w-52 gap-1 text-[12.5px] font-semibold">Category
              <CategoryPicker label="Only this category" noneLabel="All categories" value={categoryId} onChange={setCategoryId} />
            </div>
            <div role="radiogroup" aria-label="Which products" className="flex flex-wrap items-center gap-1.5">
              {([['', 'Continue'], ['skipped', 'Skipped or no answer'], ['redo', 'Redo everything']] as const).map(([m, label]) => (
                <button key={m} type="button" role="radio" aria-checked={mode === m} onClick={() => setMode(m)}
                  className="min-h-[var(--control-h)] rounded-pill border border-border bg-card px-3 text-[13px] font-semibold aria-checked:border-selected aria-checked:bg-brand-soft">{label}</button>
              ))}
            </div>
          </div>
          {mode === '' && <p className="text-[13px] text-faint">Carries on with the ones not done yet, even if you left halfway.</p>}
          {mode === 'redo' && (
            <p className="flex flex-wrap items-center gap-x-2 text-[13px] text-muted-foreground">
              Asks again about products already done. You see old and new side by side, and the old stays unless you tick the new. Leave halfway and it continues here too.
              <button type="button" className="font-semibold text-foreground underline underline-offset-2"
                onClick={() => void jobRedoOver(job.key).then(() => toast.success('The redo starts from the first product'), (e) => toast.error(errMsg(e)))}>Start the redo over</button>
            </p>
          )}
          <p className="text-[13px] text-faint">One at a time gets the chatbot's full attention. Raise it once you trust the results.</p>
          {redo && <p className="text-[14px] font-semibold">Redoing one product. You'll see the old and new side by side.</p>}
          <div className="flex flex-wrap items-center gap-2">
            <Button className="h-11 gap-1.5 px-5 font-bold" disabled={state.k === 'asking'} onClick={start}>
              {state.k === 'asking' ? <><LoaderCircle size={16} className="animate-spin" /> Asking the AI…</> : <><Sparkles size={16} /> {redo ? 'Redo it' : 'Start'}</>}
            </Button>
            {redo
              ? <Button variant="ghost" className="h-11" onClick={() => setRedo(undefined)}>Cancel the redo</Button>
              : <ProductPicker title="Redo which product?" onPick={(p) => setRedo([p.id])} trigger={(o) => <Button variant="outline" className="h-11 gap-1.5" onClick={o}><RotateCcw size={15} /> Redo a product</Button>} />}
          </div>
        </section>
      )}
      {(state.k === 'awaiting' || state.k === 'reading') && (
        <section className={CARD}>
          <b className="text-[15px]">{progress || 'Ask the chatbot'}</b>
          {progress && <small className="-mt-2 text-[13px] text-muted-foreground">Copy this to the chatbot, then paste its answer here.</small>}
          <PastePanel prompt={state.prompt} reply={state.reply} error={state.k === 'awaiting' ? state.error : undefined} reading={state.k === 'reading'}
            onType={(reply) => dispatch({ t: 'type', reply })} onReply={onReply} />
        </section>
      )}
      {pv && (pv.asked.length === 0
        ? <section className={CARD}><b>Nothing left here.</b><Button variant="outline" className="h-10 self-start" onClick={() => dispatch({ t: 'reset' })}>Back</Button></section>
        : <Preview progress={progress} job={job} pv={pv} ticks={ticks} setTicks={setTicks} edits={edits} setEdits={setEdits} merged={merged} busy={busy} onApply={() => void apply()}
          onRedo={(id) => { setRedo([id]); dispatch({ t: 'reset' }) }} onCancel={() => dispatch({ t: 'reset' })} />)}
    </div>
  )
}

function Preview({ progress, job, pv, ticks, setTicks, edits, setEdits, merged, busy, onApply, onRedo, onCancel }: {
  progress: string; job: JobInfo; pv: JobPreview; ticks: Set<string>; setTicks: (t: Set<string>) => void
  edits: Record<string, Partial<JobRow>>; setEdits: (e: Record<string, Partial<JobRow>>) => void; merged: (r: JobRow, key: string) => JobRow
  busy: boolean; onApply: () => void; onRedo: (id: number) => void; onCancel: () => void
}) {
  const toggle = (k: string) => { const n = new Set(ticks); if (n.has(k)) n.delete(k); else n.add(k); setTicks(n) }
  const groups = byProduct(pv.rows)
  const none = pv.asked.filter((id) => !groups.some((g) => g.productId === id))
  const names = !job.oneByOne && pv.rows.some((r) => r.kind === 'name')
  const picked = pv.rows.filter((r, i) => ticks.has(rowKey(r, i)) && !merged(r, rowKey(r, i)).needs).length
  return (
    <section className={CARD}>
      <div className="flex flex-wrap items-center justify-between gap-2">
        <b className="text-[15px]">{progress ? `${progress} · ` : ''}{pv.remaining} left after this</b>
        {names && <Button variant="outline" className="h-9" onClick={() => setTicks(tickAll(pv.rows, ticks, 'name', false))}>Tick all names</Button>}
      </div>
      <ul className="flex flex-col divide-y divide-border">
        {groups.map((g) => (
          <li key={g.productId} className="flex flex-col gap-1.5 py-3">
            <div className="flex items-center justify-between gap-2">
              <b className="min-w-0 truncate">{pv.names[g.productId] ?? `#${g.productId}`}</b>
              <Button variant="ghost" className="h-8 shrink-0 gap-1 px-2 text-[12.5px]" onClick={() => onRedo(g.productId)}><RotateCcw size={13} /> Redo alone</Button>
            </div>
            {g.rows.map(({ row, key }) => {
              const r = merged(row, key)
              return (
                <label key={key} className="grid grid-cols-[24px_minmax(0,1fr)] items-start gap-2 text-[13.5px]">
                  <input type="checkbox" aria-label={`${r.label}: ${r.new}`} disabled={!!r.needs} checked={ticks.has(key) && !r.needs} onChange={() => toggle(key)} className="mt-0.5 size-[18px]" />
                  <span className="min-w-0">
                    <b>{r.label}</b>{r.isNew && <span className="ml-1.5 rounded-md bg-brand-soft px-1.5 py-0.5 text-[11.5px] font-bold">new</span>}
                    {r.old && <span className="block break-words whitespace-pre-line text-faint line-through">{r.old}</span>}
                    {r.new && <span className="block break-words whitespace-pre-line">{r.new}</span>}
                    {row.needs && !row.choices && (
                      <input aria-label={row.needs} placeholder={row.needs} name={`job-${key}`} autoComplete="off" value={edits[key]?.new ?? ''} className={cn(INPUT, 'mt-1')}
                        onChange={(e) => setEdits({ ...edits, [key]: { ...edits[key], new: e.target.value, needs: answersNeed(e.target.value) ? '' : row.needs } })} />
                    )}
                    {row.isNew && r.extra?.perDay !== undefined && (
                      <span className="mt-1 grid gap-1 sm:grid-cols-[minmax(0,1fr)_120px]">
                        <input aria-label="What it means" placeholder="What it means, for the label" name={`job-meaning-${key}`} autoComplete="off" value={r.extra.meaning ?? ''} className={INPUT}
                          onChange={(e) => setEdits({ ...edits, [key]: { ...edits[key], extra: { ...edits[key]?.extra, meaning: e.target.value } } })} />
                        <input aria-label="Doses a day" title="Doses a day (0 = when needed)" inputMode="decimal" name={`job-perday-${key}`} autoComplete="off" value={r.extra.perDay} className={cn(INPUT, 'tabular-nums')}
                          onChange={(e) => setEdits({ ...edits, [key]: { ...edits[key], extra: { ...edits[key]?.extra, perDay: e.target.value.replace(/[^\d.]/g, '') } } })} />
                        <small className="text-faint sm:col-span-2">Doses a day: 0 means only when needed.</small>
                      </span>
                    )}
                    {row.needs && row.choices && (
                      <select aria-label={row.needs} value={edits[key]?.extra?.[row.key] ?? ''} className={cn(INPUT, 'mt-1')}
                        onChange={(e) => setEdits({ ...edits, [key]: { extra: { [row.key]: e.target.value }, needs: e.target.value ? '' : row.needs } })}>
                        <option value="">{row.needs}</option>
                        {row.choices.map((c) => <option key={c.value} value={c.value}>{c.label}</option>)}
                      </select>
                    )}
                    {r.reason && <small className="block text-muted-foreground">{r.reason}</small>}
                  </span>
                </label>
              )
            })}
          </li>
        ))}
      </ul>
      {none.length > 0 && <p className="text-[13px] text-muted-foreground">No answer for: {none.map((id) => pv.names[id]).join(', ')}. They stay under "Skipped or no answer".</p>}
      <div className="flex flex-wrap justify-end gap-2">
        <Button variant="ghost" className="h-11" onClick={onCancel}>Cancel</Button>
        <Button className="h-11 px-5 font-bold" disabled={busy} onClick={onApply}>{picked ? `Apply ${picked} and save the rest as skipped` : 'Nothing ticked: skip these'}</Button>
      </div>
    </section>
  )
}
