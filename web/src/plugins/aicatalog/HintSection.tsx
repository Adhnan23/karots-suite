import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Sparkles } from 'lucide-react'
import { useState } from 'react'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { hintQuery, saveHint, type Hint } from './api'

const BOX = cn(INPUT, 'h-auto min-h-20 py-2 leading-snug')

/** The AI hint on a saved product's page: what the chatbot said, correctable. It's what till
 * search finds and the ⓘ shows, so a wrong detail is worth fixing here. */
export function HintSection({ productId, where }: { productId: number; where: 'form' | 'intake' }) {
  const q = useQuery({ ...hintQuery(productId), enabled: productId > 0 && where === 'form' })
  if (productId <= 0 || where !== 'form' || !q.data) return null
  return <Editor key={`${q.data.specs}|${q.data.explanation}`} id={productId} saved={q.data} />
}

function Editor({ id, saved }: { id: number; saved: Hint }) {
  const qc = useQueryClient()
  const [h, setH] = useState(saved)
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')
  const dirty = h.specs !== saved.specs || h.explanation !== saved.explanation
  async function save() {
    setBusy(true)
    setErr('')
    try {
      await saveHint(id, h)
      await qc.invalidateQueries({ queryKey: ['x', 'aicatalog', 'hint', id] })
    } catch (e) {
      setErr(errMsg(e))
    } finally {
      setBusy(false)
    }
  }
  return (
    <section aria-label="AI hint" className="flex flex-col gap-2 rounded-[var(--radius-sm)] border border-border p-3">
      <div className="flex flex-wrap items-baseline justify-between gap-x-3">
        <h3 className="flex items-center gap-1.5 text-[13px] font-semibold text-muted-foreground"><Sparkles size={15} /> AI hint</h3>
        <small className="text-faint">Till search finds these words. A chatbot wrote them, so fix anything wrong.</small>
      </div>
      <label className="grid gap-1 text-[12.5px] font-semibold">Details: what it fits, size or strength, type
        <textarea aria-label="AI hint details" name="ai-specs" maxLength={2000} autoComplete="off" value={h.specs} onChange={(e) => setH({ ...h, specs: e.target.value })} className={BOX} />
      </label>
      <label className="grid gap-1 text-[12.5px] font-semibold">What it is
        <textarea aria-label="What it is" name="ai-explanation" maxLength={2000} autoComplete="off" value={h.explanation} onChange={(e) => setH({ ...h, explanation: e.target.value })} className={BOX} />
      </label>
      {err && <small role="alert" className="text-danger">{err}</small>}
      {dirty && <Button type="button" className="h-9 self-end font-bold" disabled={busy} onClick={() => void save()}>Save hint</Button>}
    </section>
  )
}
