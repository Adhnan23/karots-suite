import { ClipboardPaste, Copy, ExternalLink, LoaderCircle, TriangleAlert } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { currentBot, looksLikeAnswer, openChatbot } from './ask'

async function copyText(text: string, fallback: HTMLTextAreaElement | null) {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    fallback?.select()
    document.execCommand('copy') // plain-http LAN: the async clipboard is refused
  }
}

/** The paste path, the main one: Copy (and open the chatbot, if this device chose one), then paste its whole answer — it is read
 * the moment it lands. While waiting, coming back to this tab reads the clipboard (when the browser
 * allows it) and fills the box if it holds an answer. */
export function PastePanel({ prompt, reply, error, reading, onType, onReply, onCopied, extra, compact }: {
  prompt: string
  reply: string
  error?: string
  reading: boolean
  onType: (t: string) => void
  onReply: (t: string) => void
  onCopied?: () => void
  extra?: React.ReactNode
  compact?: boolean
}) {
  const [copied, setCopied] = useState(false)
  const [show, setShow] = useState(false)
  const promptRef = useRef<HTMLTextAreaElement>(null)
  const replyRef = useRef<HTMLTextAreaElement>(null)
  const choice = currentBot()
  const bot = choice.url ? choice.label : 'Chatbot'
  const button = choice.url ? `Copy & open ${choice.label}` : 'Copy prompt'

  // Back from the chatbot: read the clipboard once per return.
  useEffect(() => {
    if (!copied || !window.isSecureContext || !navigator.clipboard?.readText) return
    const back = () => {
      if (document.visibilityState !== 'visible') return
      navigator.clipboard.readText().then((t) => {
        if (t && t.trim() !== prompt.trim() && looksLikeAnswer(t)) onReply(t)
      }).catch(() => { /* not allowed: the person pastes */ })
    }
    window.addEventListener('focus', back)
    document.addEventListener('visibilitychange', back)
    return () => {
      window.removeEventListener('focus', back)
      document.removeEventListener('visibilitychange', back)
    }
  }, [copied, prompt, onReply])

  async function copy() {
    await copyText(prompt, promptRef.current)
    setCopied(true)
    onCopied?.()
    openChatbot()
    replyRef.current?.focus()
  }

  return (
    <div className={cn('flex flex-col gap-2.5', !compact && 'gap-3')}>
      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" onClick={copy} className="h-10 gap-1.5 px-4 font-bold">
          {copied || !choice.url ? <Copy size={15} /> : <ExternalLink size={15} />} {copied ? 'Copy again' : button}
        </Button>
        <button type="button" onClick={() => setShow((v) => !v)} className="text-[13px] font-semibold text-muted-foreground underline-offset-2 hover:underline">
          {show ? 'Hide prompt' : 'Show prompt'}
        </button>
        {extra}
      </div>
      <textarea ref={promptRef} readOnly value={prompt} aria-label="Prompt to copy" tabIndex={show ? 0 : -1}
        className={cn('h-28 w-full resize-y rounded-[var(--radius-sm)] border border-border bg-muted p-2 font-mono text-[12px] text-muted-foreground', !show && 'sr-only')} />
      <label className="flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground">
        <span className="flex items-center gap-1.5"><ClipboardPaste size={14} /> {bot}'s answer</span>
        <span className="relative">
          <textarea ref={replyRef} value={reply} name="ai-reply" autoComplete="off" spellCheck={false} disabled={reading}
            aria-label="Chatbot answer" aria-invalid={!!error}
            placeholder={copied ? 'Paste the whole answer here — it\'s read as soon as it lands' : 'After asking your chatbot, paste its whole answer here'}
            onChange={(e) => onType(e.target.value)}
            onPaste={(e) => { const t = e.clipboardData.getData('text'); if (t.trim()) { e.preventDefault(); onReply(t) } }}
            onKeyDown={(e) => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); onReply(reply) } }}
            className={cn('h-24 w-full resize-y rounded-[var(--radius-sm)] border bg-background p-2.5 text-[13px] font-normal text-foreground outline-none focus:border-brand',
              error ? 'border-danger' : 'border-border', reading && 'opacity-60')} />
          {reading && <LoaderCircle size={18} className="absolute right-2.5 top-2.5 animate-spin text-brand" aria-label="Reading the answer" />}
        </span>
      </label>
      {error && (
        <p role="alert" className="flex gap-2 rounded-[var(--radius-sm)] bg-[color-mix(in_srgb,var(--danger)_12%,transparent)] px-3 py-2 text-[13px] text-foreground">
          <TriangleAlert size={15} className="mt-0.5 flex-none text-danger" /> {error}
        </p>
      )}
      {copied && !window.isSecureContext && (
        <p className="text-[12px] text-faint">This connection can't read the clipboard for you — paste the answer with Ctrl+V.</p>
      )}
    </div>
  )
}
