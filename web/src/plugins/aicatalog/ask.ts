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

/** What the Copy button does on this device: copy only (the default — paste into a chat you already
 * have open, keeping its history and your own rules), or copy and open a chatbot: the three built in
 * or the shop's own (a Gem, a custom GPT…), kept in this browser only. */
export type Bot = { key: string; label: string; url: string; custom?: boolean }
export const BUILT_IN: Bot[] = [
  { key: 'copy', label: 'Copy only', url: '' },
  { key: 'gemini', label: 'Gemini', url: 'https://gemini.google.com/app' },
  { key: 'chatgpt', label: 'ChatGPT', url: 'https://chatgpt.com/' },
  { key: 'claude', label: 'Claude', url: 'https://claude.ai/new' },
]

const BOT_KEY = 'ks-aicat-bot' // per device: which Bot the Copy button uses
const BOTS_KEY = 'ks-aicat-bots' // per device: the shop's own chatbots

const read = (k: string) => {
  try {
    return localStorage.getItem(k)
  } catch {
    return null
  }
}
const write = (k: string, v: string) => {
  try {
    localStorage.setItem(k, v)
  } catch { /* private window: the choice isn't remembered */ }
}

/** A chatbot address must be a plain web link — never javascript: or data: (it would run in the shop app). */
export function safeUrl(raw: string): string | null {
  try {
    const u = new URL(raw.trim())
    return u.protocol === 'https:' || u.protocol === 'http:' ? u.href : null
  } catch {
    return null
  }
}

export function customBots(): Bot[] {
  try {
    const list = JSON.parse(read(BOTS_KEY) ?? '[]') as Bot[]
    return Array.isArray(list) ? list.filter((b) => b && typeof b.label === 'string' && safeUrl(b.url)).map((b) => ({ ...b, custom: true })) : []
  } catch {
    return []
  }
}
export const allBots = () => [...BUILT_IN, ...customBots()]

export function addBot(label: string, url: string): Bot | null {
  const href = safeUrl(url)
  if (!label.trim() || !href) return null
  const bot = { key: `custom-${Date.now()}`, label: label.trim(), url: href }
  write(BOTS_KEY, JSON.stringify([...customBots().map(({ key, label, url }) => ({ key, label, url })), bot]))
  return { ...bot, custom: true }
}
export function removeBot(key: string) {
  write(BOTS_KEY, JSON.stringify(customBots().filter((b) => b.key !== key).map(({ key, label, url }) => ({ key, label, url }))))
}

/** The Copy button's choice; a removed one falls back to Copy only. */
export function currentBot(): Bot {
  const k = read(BOT_KEY)
  return allBots().find((b) => b.key === k) ?? BUILT_IN[0]
}
export const setBot = (key: string) => write(BOT_KEY, key)

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
  const url = currentBot().url
  if (!url) return // copy only
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
