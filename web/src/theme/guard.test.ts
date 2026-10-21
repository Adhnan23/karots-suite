import { describe, expect, it } from 'vitest'

// Every look value goes through palette tokens (docs/design/theme.md §2). Printouts
// (paper/ink), the login games (canvas art) and plugins (ported separately) are out of scope.
const files = import.meta.glob<string>(
  ['/src/**/*.tsx', '!/src/plugins/**', '!/src/features/print/**', '!/src/routes/_authed/print/**', '!/src/features/auth/*Game.tsx', '!/src/**/*.test.tsx'],
  { query: '?raw', import: 'default', eager: true },
)

const RULES: [string, RegExp][] = [
  ['raw colour class', /\b(bg|text|border|ring|from|via|to|fill|stroke|shadow|outline|decoration|divide)-(white|black|slate|gray|zinc|neutral|stone|red|orange|amber|yellow|lime|green|emerald|teal|cyan|sky|blue|indigo|violet|purple|fuchsia|pink|rose)\b/],
  ['raw duration', /\bduration-\d/],
  ['raw easing', /\bease-(in|out|in-out|linear)\b/],
  ['hex colour', /(className|style)=[^\n]*#[0-9a-fA-F]{3,8}\b/],
  ['pixel corner (use --radius / --radius-sm / pill)', /\brounded(-[trbl]{1,2})?-\[\d{2,}px\]/],
  ['literal motion timing', /transition=\{\{[^}]*duration:\s*[\d.]/],
]

describe('no raw look values outside tokens', () => {
  it('scans files', () => expect(Object.keys(files).length).toBeGreaterThan(100))
  for (const [what, re] of RULES) {
    it(what, () => {
      const hits: string[] = []
      for (const [path, src] of Object.entries(files))
        src.split('\n').forEach((line, i) => {
          if (re.test(line) && !line.includes('not a UI transition')) hits.push(`${path}:${i + 1}`)
        })
      expect(hits, hits.join('\n')).toEqual([])
    })
  }
})
