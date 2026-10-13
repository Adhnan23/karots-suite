import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, LoaderCircle, MessageSquareText, RotateCcw, Sparkles, X } from 'lucide-react'
import { type ComponentProps, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { categoriesQuery } from '@/features/admin/categories/queries'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import type { ProductAssist } from '../slots'
import { findCategory, identify } from './api'
import { existingPaths, firstNew, primed, setPrimed } from './ask'
import { PastePanel } from './PastePanel'
import type { Candidate, IdentifyResult } from './types'
import { useAsk } from './useAsk'

type Props = ComponentProps<ProductAssist['Assist']>
type Source = 'ai_confident' | 'user_picked' | 'user_explained'

/** ✨ Identify under the product name: asks the AI (or hands over the prompt to paste into a
 * chatbot) what was typed, shows one sure answer or up to 4 options, and a pick fills the name and
 * category. The person still reviews and saves the form; the pick is learnt on save. */
export function Identify({ name, barcode, fill }: Props) {
  const qc = useQueryClient()
  const { state, dispatch, ask, read } = useAsk<IdentifyResult>()
  const [hint, setHint] = useState('')
  const [explaining, setExplaining] = useState(false)
  const [picked, setPicked] = useState<Candidate | null>(null)
  const [busy, setBusy] = useState(false)
  const [asked, setAsked] = useState({ query: '', short: false, hint: '' })
  const query = name.trim() || barcode.trim()

  function start(short = primed(), withHint = hint) {
    setExplaining(false)
    setPicked(null)
    setAsked({ query, short, hint: withHint })
    ask(() => identify({ query, barcode, hint: withHint, short })).catch((e) => toast.error(errMsg(e)))
  }
  const onReply = (t: string) => void read(t, (reply) => identify({ query: asked.query, barcode, hint: asked.hint, short: asked.short, reply }))

  async function pick(c: Candidate, source: Source) {
    setBusy(true)
    try {
      const categoryId = c.category ? (await findCategory(c.category)).id : undefined
      if (categoryId) await qc.invalidateQueries({ queryKey: ['admin', 'categories'] })
      fill({ name: c.name, categoryId, extra: { ...c, source, query: asked.query } })
      setPicked(c)
      dispatch({ t: 'reset' })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  if (picked && state.k === 'idle') {
    return (
      <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[12px] text-muted-foreground">
        <Sparkles size={13} className="text-brand" /> Filled from the AI: <b className="text-foreground">{picked.name}</b>
        {picked.category && <span className="rounded-full bg-muted px-2 py-0.5">{picked.category}</span>}
        <button type="button" className="font-semibold text-brand underline-offset-2 hover:underline" onClick={() => setPicked(null)}>Identify again</button>
      </p>
    )
  }

  if (state.k === 'idle' || state.k === 'asking') {
    return (
      <div className="flex items-center gap-2">
        <Button type="button" variant="outline" disabled={!query || state.k === 'asking'} onClick={() => start()}
          className="h-8 gap-1.5 rounded-full border-[color-mix(in_srgb,var(--brand)_40%,var(--border))] px-3 text-[13px] font-semibold">
          {state.k === 'asking' ? <LoaderCircle size={14} className="animate-spin" /> : <Sparkles size={14} className="text-brand" />}
          {state.k === 'asking' ? 'Asking the AI…' : 'Identify'}
        </Button>
        {state.k === 'idle' && <small className="text-[12px] text-faint">{query ? 'What is this? The AI suggests a name and category.' : 'Type a name or code, then ask the AI.'}</small>}
      </div>
    )
  }

  return (
    <section aria-label="AI identify" className="flex flex-col gap-3 rounded-[var(--radius)] border border-[color-mix(in_srgb,var(--brand)_35%,var(--border))] bg-[color-mix(in_srgb,var(--brand)_5%,var(--card))] p-3">
      <header className="flex items-center gap-2">
        <Sparkles size={15} className="text-brand" />
        <b className="flex-1 text-[13px]">Identify “{asked.query}”</b>
        <button type="button" aria-label="Close identify" onClick={() => dispatch({ t: 'reset' })} className="grid h-7 w-7 place-items-center rounded-full text-muted-foreground hover:bg-muted"><X size={15} /></button>
      </header>

      {(state.k === 'awaiting' || state.k === 'reading') && (
        <PastePanel compact prompt={state.prompt} reply={state.reply} error={state.k === 'awaiting' ? state.error : undefined} reading={state.k === 'reading'}
          onType={(reply) => dispatch({ t: 'type', reply })} onReply={onReply} onCopied={() => !asked.short && setPrimed(true)}
          extra={asked.short ? (
            <button type="button" onClick={() => { setPrimed(false); start(false) }} className="text-[13px] font-semibold text-muted-foreground underline-offset-2 hover:underline"
              title="A fresh chat doesn't know the rules yet — copy the full prompt">Full prompt (new chat)</button>
          ) : (
            <button type="button" onClick={() => { setPrimed(true); start(true) }} className="text-[13px] font-semibold text-muted-foreground underline-offset-2 hover:underline"
              title="Your chat already knows the rules (and any you added) — send just this item">Short prompt (chat already set up)</button>
          )} />
      )}

      {state.k === 'done' && (
        <Answers r={state.result} busy={busy} onPick={(c, confident) => pick(c, asked.hint ? 'user_explained' : confident ? 'ai_confident' : 'user_picked')} />
      )}

      {state.k === 'done' && (explaining ? (
        <div className="flex flex-col gap-2">
          <textarea autoFocus value={hint} onChange={(e) => setHint(e.target.value)} name="ai-hint" autoComplete="off" aria-label="Explain the item"
            placeholder="What is it? e.g. a ball bearing for a motorbike wheel, 12 mm inside"
            className="h-16 w-full resize-y rounded-[var(--radius-sm)] border border-border bg-background p-2.5 text-[13px] outline-none focus:border-brand" />
          <div className="flex gap-2">
            <Button type="button" variant="ghost" className="h-9 px-3" onClick={() => setExplaining(false)}>Cancel</Button>
            <Button type="button" disabled={!hint.trim()} className="h-9 gap-1.5 px-3" onClick={() => start(primed(), hint)}><RotateCcw size={14} /> Ask again</Button>
          </div>
        </div>
      ) : (
        <button type="button" onClick={() => setExplaining(true)} className="flex items-center gap-1.5 self-start text-[13px] font-semibold text-muted-foreground underline-offset-2 hover:underline">
          <MessageSquareText size={14} /> None of these — let me explain
        </button>
      ))}
    </section>
  )
}

function Answers({ r, busy, onPick }: { r: IdentifyResult; busy: boolean; onPick: (c: Candidate, confident: boolean) => void }) {
  const list = r.confident ? [r.best] : (r.options ?? [])
  return (
    <div className="grid gap-2 sm:grid-cols-2">
      <p className="text-[12px] font-semibold uppercase tracking-wide text-faint sm:col-span-2">{r.confident ? 'The AI is sure' : `Not sure — ${list.length} option${list.length === 1 ? '' : 's'}`}</p>
      {list.map((c, i) => (
        <article key={`${c.name}|${i}`} className={cn('flex flex-col gap-1.5 rounded-[var(--radius-sm)] border bg-card p-3', r.confident ? 'border-brand sm:col-span-2' : 'border-border')}>
          <div className="flex flex-wrap items-start gap-2">
            <b className="min-w-0 flex-1 text-[14px] leading-snug">{c.name}</b>
            <Button type="button" size="sm" disabled={busy} variant={r.confident ? 'default' : 'outline'} className="h-8 gap-1 px-3"
              onClick={() => onPick(c, r.confident)}><Check size={14} /> Use this</Button>
          </div>
          {c.category && <CategoryPath path={c.category} />}
          {c.explanation && <p className="text-[13px] text-foreground">{c.explanation}</p>}
          {c.specs && <p className="text-[12px] tabular-nums text-faint">{c.specs}</p>}
        </article>
      ))}
    </div>
  )
}

/** The suggested category, with the levels a pick would create marked "new". */
function CategoryPath({ path }: { path: string }) {
  const nodes = useQuery(categoriesQuery).data
  const parts = path.split('>').map((p) => p.trim()).filter(Boolean)
  const from = nodes ? firstNew(path, existingPaths(nodes)) : -1
  return (
    <span className="flex flex-wrap items-center gap-1.5 self-start">
      <span className="rounded-full bg-muted px-2 py-0.5 text-[12px] font-semibold text-muted-foreground">
        {parts.map((p, i) => (
          <span key={i}>{i > 0 && ' > '}{from >= 0 && i >= from ? <b className="text-foreground">{p}</b> : p}</span>
        ))}
      </span>
      {from >= 0 && (
        <span className="rounded-full border border-[color-mix(in_srgb,var(--brand)_45%,var(--border))] px-2 py-0.5 text-[11px] font-bold text-brand">
          new {from === 0 ? 'category' : 'sub-category'}
        </span>
      )}
    </span>
  )
}
