import { Plus, X } from 'lucide-react'
import { useState } from 'react'
import { INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { addBot, allBots, currentBot, removeBot, safeUrl, setBot } from './ask'

const CHIP = 'rounded-full border border-border bg-card px-4 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

/** What the Copy button does on this device: copy only, open a built-in chatbot, or open one of the
 * shop's own (a Gem, a custom GPT…). Kept in this browser only — never in the shop's data. */
export function CopyChoice() {
  const [bots, setBots] = useState(allBots)
  const [pick, setPick] = useState(() => currentBot().key)
  const [adding, setAdding] = useState(false)
  const [name, setName] = useState('')
  const [url, setUrl] = useState('')
  const urlOk = !url.trim() || !!safeUrl(url)
  const choose = (key: string) => { setPick(key); setBot(key) }

  function add() {
    const b = addBot(name, url)
    if (!b) return
    setBots(allBots())
    choose(b.key)
    setName('')
    setUrl('')
    setAdding(false)
  }
  function remove(key: string) {
    removeBot(key)
    setBots(allBots())
    if (pick === key) choose('copy')
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-1.5">
        {bots.map((b) => b.custom ? (
          <span key={b.key} className="inline-flex">
            <button type="button" aria-pressed={pick === b.key} onClick={() => choose(b.key)} title={b.url} className={cn(CHIP, 'rounded-r-none border-r-0 pr-2.5')}>
              Copy & open {b.label}
            </button>
            <button type="button" aria-label={`Remove ${b.label}`} onClick={() => remove(b.key)}
              className={cn(CHIP, 'rounded-l-none px-2.5', pick === b.key && 'border-selected')}><X size={14} /></button>
          </span>
        ) : (
          <button key={b.key} type="button" aria-pressed={pick === b.key} onClick={() => choose(b.key)} className={CHIP}>
            {b.url ? `Copy & open ${b.label}` : b.label}
          </button>
        ))}
        {!adding && (
          <button type="button" onClick={() => setAdding(true)} className={cn(CHIP, 'inline-flex items-center gap-1 border-dashed')}><Plus size={14} /> Add chatbot</button>
        )}
      </div>
      {adding && (
        <div className="grid gap-2 rounded-[var(--radius-sm)] border border-border p-3 sm:grid-cols-[1fr_2fr_auto] sm:items-start">
          <input aria-label="Chatbot name" placeholder="Name, e.g. My Gemini Gem" value={name} onChange={(e) => setName(e.target.value)} className={INPUT} maxLength={40} autoComplete="off" />
          <div className="flex flex-col gap-1">
            <input aria-label="Chatbot address" placeholder="https://…" value={url} onChange={(e) => setUrl(e.target.value)} aria-invalid={!urlOk}
              onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); add() } }}
              className={cn(INPUT, !urlOk && 'border-danger')} spellCheck={false} autoComplete="off" />
            {!urlOk && <small className="text-[12px] text-danger">Use a web address starting with https://</small>}
          </div>
          <div className="flex gap-1.5">
            <Button type="button" variant="ghost" className="h-[var(--control-h)] px-3" onClick={() => setAdding(false)}>Cancel</Button>
            <Button type="button" className="h-[var(--control-h)] px-4 font-bold" disabled={!name.trim() || !safeUrl(url)} onClick={add}>Add</Button>
          </div>
        </div>
      )}
    </div>
  )
}
