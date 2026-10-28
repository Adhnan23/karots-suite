import { expect, test } from 'bun:test'
import { existsSync, statSync } from 'node:fs'
import { LINES } from '../src/lines'
import { LANGS } from '../src/types'

// Run after `bun run voice`: every spoken line has its mp3 and a length.
test('every spoken line has a voice file and a duration', async () => {
  const durations = (await import('../src/durations.json')).default as Record<string, Record<string, number>>
  for (const lang of LANGS) {
    for (const [id, line] of Object.entries(LINES[lang])) {
      if (!line.voice) continue
      const file = `public/voice/${lang}/${id}.mp3`
      expect(existsSync(file) && statSync(file).size > 1024, file).toBe(true)
      expect(durations[lang]?.[id] ?? 0, `${lang} ${id}`).toBeGreaterThan(0.3)
    }
  }
})
