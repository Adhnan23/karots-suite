import { ChevronDown, ClipboardCopy, Sparkles } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { applyAiReply, buildPrompt } from './aiPrompt'
import type { PaletteDoc } from './paletteCss'

/** Make it with AI: copy a prompt for any AI, paste its JSON reply back. */
export function AiBox({ doc, onApply }: { doc: PaletteDoc; onApply: (d: PaletteDoc) => void }) {
  const [reply, setReply] = useState('')
  const [manual, setManual] = useState('')
  const [surprise, setSurprise] = useState(false)

  const copy = async () => {
    const prompt = buildPrompt(doc, { surprise })
    try {
      await navigator.clipboard.writeText(prompt)
      toast.success(surprise ? 'Prompt copied. Paste it into any AI as it is.' : 'Prompt copied. Describe your look where it says, then paste it into any AI.')
    } catch {
      setManual(prompt) // no clipboard (plain http on the shop network): show it to copy by hand
    }
  }

  const apply = () => {
    try {
      const r = applyAiReply(doc, reply)
      onApply(r.doc)
      setReply('')
      const skipped = r.ignored.length ? ` Skipped: ${r.ignored.join(', ')}.` : ''
      toast.success(`Applied ${r.applied} values. Check Readability, then Save.${skipped}`)
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Could not read that reply')
    }
  }

  return (
    <details className="group rounded-[var(--radius)] border border-border bg-card shadow-[var(--shadow)] [backdrop-filter:var(--glass)]">
      <summary className="flex cursor-pointer list-none items-center gap-2 px-4 py-3 [&::-webkit-details-marker]:hidden">
        <Sparkles className="size-4 text-brand" />
        <span className="font-display font-bold">Make it with AI</span>
        <span className="text-xs font-semibold text-faint">Describe a look, paste the answer</span>
        <ChevronDown className="ml-auto size-4 text-muted-foreground transition-transform duration-(--dur-fast) group-open:rotate-180" />
      </summary>
      <div className="flex flex-col gap-3 border-t border-border px-4 py-3 text-sm">
        <ol className="list-decimal space-y-1 pl-5 text-muted-foreground">
          <li>Copy the prompt and paste it into any AI (ChatGPT, Claude, Gemini…).</li>
          <li>Replace the line under “THE LOOK I WANT” with what you want, or tick Surprise me and let the AI choose.</li>
          <li>Paste the AI's whole answer below and press Apply. Undo puts it back.</li>
        </ol>
        <div className="flex flex-wrap items-center gap-3">
          <button type="button" onClick={copy} className="flex h-10 items-center gap-1.5 rounded-pill border border-border px-4 font-semibold hover:bg-muted">
            <ClipboardCopy className="size-4" /> Copy prompt
          </button>
          <label className="flex items-center gap-2 font-semibold">
            <input type="checkbox" checked={surprise} onChange={(e) => { setSurprise(e.target.checked); setManual('') }} className="size-4 accent-(--brand)" />
            Surprise me <span className="font-normal text-faint">(the AI chooses the look)</span>
          </label>
        </div>
        {manual && (
          <textarea aria-label="Prompt to copy" readOnly value={manual} onFocus={(e) => e.target.select()}
            className="h-32 w-full rounded-[var(--radius-sm)] border border-input bg-background/60 p-2 font-mono text-xs" />
        )}
        <textarea aria-label="AI answer" value={reply} onChange={(e) => setReply(e.target.value)} spellCheck={false}
          placeholder='Paste the answer here: {"name": "…", "light": {…}, "dark": {…}}'
          className="h-28 w-full rounded-[var(--radius-sm)] border border-input bg-background/60 p-2 font-mono text-xs outline-none focus:border-brand" />
        <button type="button" onClick={apply} disabled={!reply.trim()}
          className="h-10 self-start rounded-pill bg-brand px-5 font-bold text-brand-foreground disabled:opacity-50">Apply</button>
      </div>
    </details>
  )
}
