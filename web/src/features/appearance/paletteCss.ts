/** Reads and writes palette files.
 *  A file is a header comment, a light token block, a dark token block, the same
 *  dark block under prefers-color-scheme, then any extra CSS ("Custom CSS"). */
import { PALETTES, THEME } from '@/theme/palettes'
import { TOKENS } from '@/theme/palettes/tokens'

export type PaletteDoc = {
  theme: string
  name: string
  base: string
  light: Record<string, string>
  dark: Record<string, string>
  extra: string
}

export const paletteId = (customId: number) => `custom-${customId}`

const HEADER = /\/\*\s*karots-palette theme=(\S+) name=(.*?) base=(\S+)\s*\*\//
const ROOT = /^:root\[data-ui="[^"]*"\]\[data-palette="[^"]*"\](\[data-theme="dark"\])?$/

/** Top-level items: [prelude, body] for blocks, [text, null] for comments. */
function items(css: string): [string, string | null][] {
  const out: [string, string | null][] = []
  let i = 0
  while (i < css.length) {
    if (/\s/.test(css[i])) {
      i++
    } else if (css.startsWith('/*', i)) {
      const end = css.indexOf('*/', i + 2)
      const stop = end < 0 ? css.length : end + 2
      out.push([css.slice(i, stop), null])
      i = stop
    } else {
      const open = css.indexOf('{', i)
      if (open < 0) break
      let depth = 1
      let j = open + 1
      for (; j < css.length && depth > 0; j++) {
        if (css.startsWith('/*', j)) j = css.indexOf('*/', j + 2) + 1 || css.length
        else if (css[j] === '{') depth++
        else if (css[j] === '}') depth--
      }
      out.push([css.slice(i, open).trim(), css.slice(open + 1, j - 1)])
      i = j
    }
  }
  return out
}

function decls(body: string): Record<string, string> {
  const out: Record<string, string> = {}
  for (const d of body.replace(/\/\*[\s\S]*?\*\//g, '').split(';')) {
    const k = d.indexOf(':')
    const name = d.slice(0, k).trim()
    if (k > 0 && name.startsWith('--')) out[name] = d.slice(k + 1).trim()
  }
  return out
}

export function parsePalette(css: string): PaletteDoc {
  const h = HEADER.exec(css)
  const doc: PaletteDoc = { theme: h?.[1] ?? THEME, name: h?.[2].trim() ?? '', base: h?.[3] ?? '', light: {}, dark: {}, extra: '' }
  const extra: string[] = []
  for (const [prelude, body] of items(css)) {
    if (body === null) continue // comments (the header included) aren't carried
    const root = ROOT.exec(prelude)
    if (root) Object.assign(root[1] ? doc.dark : doc.light, decls(body))
    else if (!/^@media\s*\(prefers-color-scheme:\s*dark\)$/.test(prelude)) extra.push(`${prelude} {${body}}`)
  }
  doc.extra = extra.join('\n\n')
  return doc
}

function block(tokens: Record<string, string>, indent: string) {
  const known = TOKENS.map((t) => t.name).filter((n) => n in tokens)
  const rest = Object.keys(tokens).filter((n) => !known.includes(n))
  return [...known, ...rest].map((n) => `${indent}${n}: ${tokens[n]};`).join('\n')
}

export function buildPalette(doc: PaletteDoc, id: string): string {
  const sel = `:root[data-ui="${doc.theme}"][data-palette="${id}"]`
  const parts = [
    `/* karots-palette theme=${doc.theme} name=${doc.name} base=${doc.base} */`,
    `${sel} {\n  color-scheme: light;\n\n${block(doc.light, '  ')}\n}`,
    `${sel}[data-theme="dark"] {\n  color-scheme: dark;\n\n${block(doc.dark, '  ')}\n}`,
    `@media (prefers-color-scheme: dark) {\n  ${sel}:not([data-theme="light"]) {\n    color-scheme: dark;\n\n${block(doc.dark, '    ')}\n  }\n}`,
  ]
  // Extra rules belong to this palette: a copy's rules point at the copy, not its source.
  if (doc.extra.trim()) parts.push(doc.extra.trim().replace(/\[data-palette="[^"]*"\]/g, `[data-palette="${id}"]`))
  return parts.join('\n\n') + '\n'
}

const BUILTIN = import.meta.glob<string>('/src/theme/palettes/dock/*.css', { query: '?raw', import: 'default', eager: true })

export const isBuiltin = (id: string) => `/src/theme/palettes/dock/${id}.css` in BUILTIN

/** A built-in palette as a doc (built-ins have no header). */
export function builtinDoc(id: string): PaletteDoc {
  const css = BUILTIN[`/src/theme/palettes/dock/${id}.css`] ?? ''
  const name = PALETTES.find((p) => p.id === id)?.name ?? id
  return { ...parsePalette(css), theme: THEME, name, base: id }
}
