import type { AskReply } from './types'

/** One AI action, paste first: idle → asking → done, or → awaiting a pasted reply (the prompt to
 * copy; error says why a paste or the API failed, and the pasted text is kept) → reading → done. */
export type AskState<T> =
  | { k: 'idle' }
  | { k: 'asking' }
  | { k: 'awaiting'; prompt: string; reply: string; error?: string }
  | { k: 'reading'; prompt: string; reply: string }
  | { k: 'done'; result: T }

export type AskAction<T> =
  | { t: 'ask' }
  | { t: 'answer'; a: AskReply<T> }
  | { t: 'type'; reply: string }
  | { t: 'read' }
  | { t: 'fail'; error: string }
  | { t: 'reset' }

export function askReducer<T>(s: AskState<T>, a: AskAction<T>): AskState<T> {
  switch (a.t) {
    case 'ask':
      return { k: 'asking' }
    case 'answer':
      if (a.a.result) return { k: 'done', result: a.a.result }
      return { k: 'awaiting', prompt: a.a.prompt ?? '', reply: s.k === 'reading' || s.k === 'awaiting' ? s.reply : '', error: a.a.error }
    case 'type':
      return s.k === 'awaiting' ? { ...s, reply: a.reply, error: undefined } : s
    case 'read':
      return s.k === 'awaiting' && s.reply.trim() ? { k: 'reading', prompt: s.prompt, reply: s.reply } : s
    case 'fail':
      return s.k === 'reading' ? { k: 'awaiting', prompt: s.prompt, reply: s.reply, error: a.error } : s.k === 'asking' ? { k: 'idle' } : s
    case 'reset':
      return { k: 'idle' }
  }
}

/** Does this text hold a JSON object answer? Used before filling the reply box from the
 * clipboard, so whatever else was copied never lands there. */
export function looksLikeAnswer(text: string): boolean {
  const i = text.indexOf('{')
  const j = text.lastIndexOf('}')
  if (i < 0 || j <= i) return false
  try {
    return typeof JSON.parse(text.slice(i, j + 1)) === 'object'
  } catch {
    return false
  }
}

export const chatbots = {
  gemini: { label: 'Gemini', url: 'https://gemini.google.com/app' },
  chatgpt: { label: 'ChatGPT', url: 'https://chatgpt.com/' },
  claude: { label: 'Claude', url: 'https://claude.ai/new' },
} as const
export type Chatbot = keyof typeof chatbots

const BOT_KEY = 'ks-aicat-bot' // per device: which chatbot the Copy button opens
export function chatbot(): Chatbot {
  try {
    const v = localStorage.getItem(BOT_KEY)
    return v && v in chatbots ? (v as Chatbot) : 'gemini'
  } catch {
    return 'gemini'
  }
}
export function setChatbot(b: Chatbot) {
  try {
    localStorage.setItem(BOT_KEY, b)
  } catch { /* private window: the default stays */ }
}

const PRIMED = 'ks-aicat-primed' // this tab's chat already has the full rules
export function primed(): boolean {
  try {
    return sessionStorage.getItem(PRIMED) === '1'
  } catch {
    return false
  }
}
export function setPrimed(on: boolean) {
  try {
    if (on) sessionStorage.setItem(PRIMED, '1')
    else sessionStorage.removeItem(PRIMED)
  } catch { /* no storage: every prompt is a full one */ }
}

/** Opens the chatbot in one reused tab ('ai-chatbot'). A tab already on the chatbot is only
 * focused — navigating it again would start a new chat and lose the rules it was given. */
export function openChatbot() {
  const url = chatbots[chatbot()].url
  const w = window.open('', 'ai-chatbot')
  if (!w) return void window.open(url, '_blank', 'noopener')
  try {
    if (w.location.href === 'about:blank') {
      w.opener = null
      w.location.href = url
    }
  } catch { /* already on the chatbot (another origin): just bring it forward */ }
  w.focus()
}
