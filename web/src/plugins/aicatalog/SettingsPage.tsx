import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ClipboardPaste, Cloud, LoaderCircle, PanelRight, Plug, Save } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Field, INPUT } from '@/components/admin/fields'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { loadModels, saveSettings, settingsQuery, testAI } from './api'
import { type Chatbot, chatbot, chatbots, setChatbot } from './ask'
import type { Settings } from './types'

const PRESETS = [
  { key: 'gemini', label: 'Google Gemini — free tier, can search the web', base: 'https://generativelanguage.googleapis.com/v1beta/openai', model: 'gemini-3.6-flash' },
  { key: 'groq', label: 'Groq — fast free tier', base: 'https://api.groq.com/openai/v1', model: 'openai/gpt-oss-120b' },
  { key: 'openrouter', label: 'OpenRouter — free models', base: 'https://openrouter.ai/api/v1', model: 'google/gemini-2.0-flash-exp:free' },
  { key: 'openai', label: 'OpenAI — paid', base: 'https://api.openai.com/v1', model: 'gpt-4o-mini' },
  { key: 'ollama', label: 'Ollama — on this computer, no key', base: 'http://localhost:11434/v1', model: '' },
  { key: 'lmstudio', label: 'LM Studio — on this computer, no key', base: 'http://localhost:1234/v1', model: '' },
  { key: 'llamacpp', label: 'llama.cpp server — on this computer, no key', base: 'http://localhost:8080/v1', model: '' },
]
const AREA = cn(INPUT, 'h-auto min-h-20 py-2 leading-snug')

/** Apps → AI Assistant → Settings: paste mode (the default) or an API, the naming style and
 * standing instructions. The key is typed once and never comes back to the browser. */
export function SettingsPage() {
  const s = useQuery(settingsQuery).data
  return s ? <Form init={s} /> : null
}

function Form({ init }: { init: Settings }) {
  const qc = useQueryClient()
  const [s, setS] = useState<Settings>({ ...init, apiKey: '' })
  const [bot, setBot] = useState<Chatbot>(chatbot)
  const [models, setModels] = useState<string[]>([])
  const [busy, setBusy] = useState<'' | 'save' | 'models' | 'test'>('')
  const set = (p: Partial<Settings>) => setS((x) => ({ ...x, ...p }))

  async function run(kind: 'save' | 'models' | 'test') {
    setBusy(kind)
    try {
      if (kind === 'save') {
        const saved = await saveSettings(s)
        qc.setQueryData(settingsQuery.queryKey, saved)
        setS({ ...saved, apiKey: '' })
        toast.success('AI settings saved')
      } else if (kind === 'models') {
        const list = (await loadModels(s)).models
        setModels(list)
        toast.success(`${list.length} models found`)
      } else {
        await testAI(s)
        toast.success('The AI answered')
      }
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy('')
    }
  }

  const mode = (manual: boolean, icon: React.ReactNode, title: string, body: string) => (
    <button type="button" aria-pressed={s.manualMode === manual} onClick={() => set({ manualMode: manual })}
      className="flex flex-1 basis-56 items-start gap-3 rounded-[var(--radius)] border border-border bg-card p-4 text-left aria-pressed:border-selected aria-pressed:bg-[color-mix(in_srgb,var(--selected)_10%,var(--card))]">
      <span className="grid h-10 w-10 flex-none place-items-center rounded-[12px] bg-muted text-brand">{icon}</span>
      <span className="flex flex-col gap-0.5"><b className="text-[15px]">{title}</b><span className="text-[13px] text-muted-foreground">{body}</span></span>
    </button>
  )

  return (
    <form className="flex max-w-3xl flex-col gap-5" autoComplete="off" onSubmit={(e) => { e.preventDefault(); void run('save') }}>
      <div className="flex flex-wrap gap-3">
        {mode(true, <ClipboardPaste size={20} />, 'Paste mode', 'No key needed. Copy the prompt into any free chatbot and paste its answer back.')}
        {mode(false, <Cloud size={20} />, 'API', 'The shop asks the AI directly with your API key — no copying.')}
      </div>

      {s.manualMode ? (
        <section className="flex flex-col gap-3 rounded-[var(--radius)] border border-border bg-card p-4">
          <b className="text-[14px]">Chatbot the Copy button opens <span className="font-normal text-faint">(this device)</span></b>
          <div className="flex flex-wrap gap-1.5">
            {(Object.keys(chatbots) as Chatbot[]).map((k) => (
              <button key={k} type="button" aria-pressed={bot === k} onClick={() => { setBot(k); setChatbot(k) }}
                className="rounded-full border border-border bg-card px-4 py-2 text-[13px] font-semibold text-muted-foreground aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground">
                {chatbots[k].label}
              </button>
            ))}
          </div>
          <p className="flex gap-2 text-[13px] text-muted-foreground">
            <PanelRight size={16} className="mt-0.5 flex-none text-brand" />
            <span>Tip: keep the chatbot beside this page instead of switching tabs — in Chrome open <b className="text-foreground">Gemini in the side panel</b>; in Firefox use the <b className="text-foreground">AI chatbot in the sidebar</b>. Gemini searches the web for free, which helps with part numbers. Chrome can also read the answer from the clipboard when you come back; in Firefox, press Ctrl+V in the answer box.</span>
          </p>
        </section>
      ) : (
        <section className="grid gap-3 rounded-[var(--radius)] border border-border bg-card p-4 md:grid-cols-2">
          <Field label="Provider" className="md:col-span-2">
            <select aria-label="Provider" value={PRESETS.some((p) => p.key === s.provider) ? s.provider : ''} className={cn(INPUT, 'appearance-auto')}
              onChange={(e) => { const p = PRESETS.find((x) => x.key === e.target.value); if (p) { set({ provider: p.key, baseUrl: p.base, model: p.model }); setModels([]) } }}>
              <option value="" disabled>Other (type the address below)</option>
              {PRESETS.map((p) => <option key={p.key} value={p.key}>{p.label}</option>)}
            </select>
          </Field>
          <Field label="Address (base URL)" className="md:col-span-2" hint="Any OpenAI-compatible address works.">
            <input aria-label="Base URL" value={s.baseUrl} onChange={(e) => set({ baseUrl: e.target.value })} className={INPUT} spellCheck={false} />
          </Field>
          <Field label="API key" hint={init.hasKey ? 'A key is saved — leave blank to keep it.' : 'Local providers need none.'}>
            <input aria-label="API key" type="password" name="ai-key" autoComplete="off" value={s.apiKey ?? ''} onChange={(e) => set({ apiKey: e.target.value })}
              placeholder={init.hasKey ? '•••••••• saved' : 'Paste the key'} className={INPUT} spellCheck={false} />
          </Field>
          <Field label="Model">
            <input aria-label="Model" list="ai-models" value={s.model} onChange={(e) => set({ model: e.target.value })} className={INPUT} spellCheck={false} />
            <datalist id="ai-models">{models.map((m) => <option key={m} value={m} />)}</datalist>
          </Field>
          {s.provider === 'gemini' && (
            <label className="flex items-start gap-2.5 text-[13px] md:col-span-2">
              <input type="checkbox" checked={s.webSearch} onChange={(e) => set({ webSearch: e.target.checked })} className="mt-0.5 h-4 w-4 accent-[var(--brand)]" />
              <span><b>Search the web</b> <span className="text-muted-foreground">— checks part numbers online. Google's free quota for it is smaller; turn off if you run out.</span></span>
            </label>
          )}
          <div className="flex flex-wrap gap-2 md:col-span-2">
            <Button type="button" variant="outline" className="h-10 gap-1.5 px-4" disabled={!!busy} onClick={() => run('models')}>
              {busy === 'models' ? <LoaderCircle size={15} className="animate-spin" /> : <Plug size={15} />} Load models
            </Button>
            <Button type="button" variant="outline" className="h-10 gap-1.5 px-4" disabled={!!busy || !s.model} onClick={() => run('test')}>
              {busy === 'test' && <LoaderCircle size={15} className="animate-spin" />} Test AI
            </Button>
          </div>
        </section>
      )}

      <Field label="Naming style" hint="How product names should look, e.g. “Brand Product Size — Anchor Milk Powder 400g”. Used by Identify and the name clean-up.">
        <textarea aria-label="Naming style" value={s.namingStyle} onChange={(e) => set({ namingStyle: e.target.value })} className={AREA} />
      </Field>
      <Field label="Custom instructions" hint="Added to every prompt, e.g. “Use British spelling” or “We sell motorbike spares”.">
        <textarea aria-label="Custom instructions" value={s.customPrompt} onChange={(e) => set({ customPrompt: e.target.value })} className={AREA} />
      </Field>
      <div>
        <Button type="submit" disabled={!!busy} className="h-11 gap-1.5 px-6 font-bold">
          {busy === 'save' ? <LoaderCircle size={16} className="animate-spin" /> : <Save size={16} />} Save settings
        </Button>
      </div>
    </form>
  )
}
