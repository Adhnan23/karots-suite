/** "Make it with AI": a prompt the system user pastes into any AI, and the strict JSON
 *  reply pasted back. JSON is only the exchange; the palette is still saved as CSS. */
import { TOKENS } from '@/theme/palettes/tokens'
import type { PaletteDoc } from './paletteCss'

const WHAT: Record<string, string> = {
  '--background': 'page background',
  '--foreground': 'main text',
  '--card': 'cards and panels (may be see-through for glass)',
  '--popover': 'menus and dialogs (always solid)',
  '--muted': 'quiet fills: inputs, chips, hover',
  '--accent': 'HOVER background of menu items and plain buttons (a quiet fill like --muted, NOT a highlight colour)',
  '--accent-foreground': 'text on --accent (usually var(--foreground))',
  '--secondary': 'quiet fill for secondary buttons (like --muted)',
  '--muted-foreground': 'secondary text',
  '--faint': 'least important text (hints, captions)',
  '--border': 'lines and outlines',
  '--scrim': 'dimmed layer behind dialogs',
  '--brand': 'the shop colour: main buttons, active dock item',
  '--brand-foreground': 'text on --brand',
  '--brand-soft': 'light tint of the brand for highlights',
  '--brand-2': 'second accent colour (badges)',
  '--brand-2-foreground': 'text on --brand-2',
  '--selected': 'pressed tabs, segments, toggles',
  '--selected-foreground': 'text on --selected',
  '--success': 'money in, OK states (green family)',
  '--danger': 'money out, delete, errors (red family)',
  '--danger-foreground': 'text on --danger buttons',
  '--warning': 'warnings, low stock (amber family)',
  '--cheque': 'anything paid by cheque (its own hue, not brand / green / red / amber)',
  '--cheque-foreground': 'text on --cheque',
  '--hero': 'dark showcase panel (sale total, login hero)',
  '--hero-foreground': 'text on --hero',
  '--hero-muted': 'secondary text on --hero',
  '--bar': 'floating dock and tab bar background',
  '--glass': 'backdrop-filter for glass surfaces, or none',
  '--bg-art': 'page background image (gradients) or none',
  '--hero-art': 'gradients over --hero',
  '--shadow': 'card shadow (inset rims allowed)',
  '--shadow-float': 'floating things: dock, dialogs',
  '--glow': 'glow around main buttons, or none',
  '--radius': 'card corner radius',
  '--radius-sm': 'button / input corner radius',
  '--pill': 'pill radius: 9999px (round) or var(--radius-sm) (soft)',
  '--dur-fast': 'quick transitions (≤ 350ms)',
  '--dur': 'normal transitions (≤ 350ms)',
  '--dur-slow': 'big transitions (≤ 350ms)',
  '--ease': 'easing curve, cubic-bezier(...)',
  '--ease-spring': 'springy easing for pop-ins, cubic-bezier(...)',
  '--lift': 'hover rise of buttons/tiles: 0px calm, 2px lively',
}

const SURPRISE = `Surprise me: I'm not telling you what I want. Invent an original, tasteful palette with a clear personality that would make a small shop proud (not a copy of the current one, not plain grey). Pick everything yourself: colours, glass or solid, corners, shadows and motion.`

export function buildPrompt(doc: PaletteDoc, { surprise = false } = {}): string {
  const lines = TOKENS.map((t) => `- ${t.name}: ${WHAT[t.name] ?? (t.sub ? `${t.sub.toLowerCase()} colour` : t.group)}`)
  const current = JSON.stringify({ name: doc.name, light: doc.light, dark: doc.dark }, null, 2)
  return `You are designing a colour palette for Karots, a point-of-sale and back-office web app used by small retail shops in Sri Lanka (a till for cashiers, admin pages on phones and PCs).

${surprise ? SURPRISE : `THE LOOK I WANT:
<describe it here, e.g. "warm and friendly like a tea shop, cream background, deep green brand, rounded, lively">`}

A palette is a set of CSS custom properties, given twice: once for light mode and once for dark mode. Here is every token and what it is for:
${lines.join('\n')}

Rules:
- Values are plain CSS values: hex (#RRGGBB), rgb(r g b / a), color-mix(...), gradients for --bg-art / --hero-art, box-shadows, lengths, times, cubic-bezier(...). A token may point at another with var(--token).
- Never use ";", "{" or "}" inside a value.
- Text must stay readable: at least 4.5:1 contrast for text on its background (--foreground on --background and --card, each *-foreground on its colour), 3:1 for --success / --danger / --warning on --card.
- Keep money colours recognisable: --success green family, --danger red family, --warning amber family.
- Durations at most 350ms.
- You may leave out tokens you don't want to change; they keep their current value.

Here is the current palette${surprise ? ' (only to show the format; change as much as you like)' : ', for reference'}:
\`\`\`json
${current}
\`\`\`

Answer with only the JSON, in exactly this shape, and nothing else:
{"name": "<short palette name>", "light": {"--token": "value", ...}, "dark": {"--token": "value", ...}}`
}

const KNOWN = new Set(TOKENS.map((t) => t.name))

/** Merges an AI reply into doc. Unknown tokens and unsafe values are left out and listed. */
export function applyAiReply(doc: PaletteDoc, reply: string) {
  const text = /```(?:json)?\s*([\s\S]*?)```/.exec(reply)?.[1] ?? reply.slice(reply.indexOf('{'), reply.lastIndexOf('}') + 1)
  let data: { name?: unknown; light?: unknown; dark?: unknown }
  try {
    data = JSON.parse(text)
  } catch {
    throw new Error("That reply isn't the JSON the prompt asks for. Ask the AI to answer with only the JSON.")
  }
  const ignored = new Set<string>()
  let applied = 0
  const merge = (base: Record<string, string>, part: unknown) => {
    const out = { ...base }
    if (part && typeof part === 'object')
      for (const [k, v] of Object.entries(part)) {
        if (KNOWN.has(k) && typeof v === 'string' && v.trim() && !/[;{}]/.test(v)) {
          out[k] = v.trim()
          applied++
        } else ignored.add(k)
      }
    return out
  }
  const next: PaletteDoc = { ...doc, light: merge(doc.light, data.light), dark: merge(doc.dark, data.dark) }
  if (typeof data.name === 'string' && data.name.trim()) next.name = data.name.trim().slice(0, 60)
  return { doc: next, applied, ignored: [...ignored] }
}
