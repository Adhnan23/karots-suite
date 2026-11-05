import { useQueryClient } from '@tanstack/react-query'
import { LoaderCircle, Sparkles } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import type { PageDraft } from '../slots'
import { PastePanel } from './PastePanel'
import type { AskReply } from './types'
import { useAsk } from './useAsk'

type Idea = { name: string; productIds: number[]; names: string[]; reason: string }
const ask_ = (reply?: string) => api<AskReply<{ ideas: Idea[] }>>('/x/aicatalog/combo-suggest', { json: { reply } })

/** ✨ Suggest combos on the Combos page. Use opens that page's own form with the products
 * filled in: the person sets the price and saves. */
export function ComboSuggest({ open }: { open: (d: PageDraft) => void }) {
  const qc = useQueryClient()
  const { state, dispatch, ask, read } = useAsk<{ ideas: Idea[] }>()
  const [shown, setShown] = useState(false)
  const start = () => {
    setShown(true)
    dispatch({ t: 'reset' })
    ask(() => ask_()).catch((e) => toast.error(errMsg(e)))
  }
  const close = () => { setShown(false); dispatch({ t: 'reset' }); void qc.invalidateQueries({ queryKey: ['x', 'combos'] }) }
  return (
    <>
      <Button variant="secondary" className="h-10 gap-1.5 px-4" onClick={start}><Sparkles size={16} /> Suggest combos</Button>
      {shown && (
        <FormSheet open onOpenChange={(o) => !o && close()} title="Combo suggestions" busy={false} onSubmit={close} submit="Done">
          <p className="text-[13.5px] text-muted-foreground">Based on what usually sells together. You set the price, and nothing is saved until you do.</p>
          {state.k === 'asking' && <p className="flex items-center gap-2"><LoaderCircle size={16} className="animate-spin" /> Asking the AI…</p>}
          {(state.k === 'awaiting' || state.k === 'reading') && (
            <PastePanel prompt={state.prompt} reply={state.reply} error={state.k === 'awaiting' ? state.error : undefined} reading={state.k === 'reading'}
              onType={(reply) => dispatch({ t: 'type', reply })} onReply={(t) => void read(t, (r) => ask_(r))} />
          )}
          {state.k === 'done' && (state.result.ideas.length === 0
            ? <p>No suggestions from that reply.</p>
            : (
              <ul className="flex flex-col divide-y divide-border">
                {state.result.ideas.map((c) => (
                  <li key={c.name} className="flex flex-wrap items-center gap-2 py-2.5">
                    <span className="min-w-0 flex-1">
                      <b className="block">{c.name}</b>
                      <small className="block break-words">{c.names.join(' + ')}</small>
                      {c.reason && <small className="block text-muted-foreground">{c.reason}</small>}
                    </span>
                    <Button type="button" variant="outline" className="h-9" onClick={() => { open({ name: c.name, productIds: c.productIds }); close() }}>Use</Button>
                  </li>
                ))}
              </ul>
            ))}
        </FormSheet>
      )}
    </>
  )
}
