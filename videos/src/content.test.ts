import { expect, test } from 'bun:test'
import { existsSync } from 'node:fs'
import { CLIPS } from './clips'
import { LINES } from './lines'
import { LANGS, SHARED_LINES, SILENT_LINES } from './types'
import { SCENE_IDS, VIDEOS } from './videos'

const ids = [...SCENE_IDS, ...SHARED_LINES]

test('every language has a caption and (unless silent) a voice line for every id', () => {
  for (const lang of LANGS) {
    for (const id of ids) {
      const l = LINES[lang][id]
      expect(l?.caption.trim(), `${lang} ${id} caption`).toBeTruthy()
      if (!SILENT_LINES.includes(id)) expect(l.voice.trim(), `${lang} ${id} voice`).toBeTruthy()
    }
  }
})

test('no language carries lines nothing uses', () => {
  for (const lang of LANGS) expect(Object.keys(LINES[lang]).filter((k) => !ids.includes(k))).toEqual([])
})

test('English scene captions stay short enough to read on a phone (end card copy is fixed by the spec)', () => {
  for (const id of SCENE_IDS) expect(LINES.en[id].caption.split(/\s+/).length, id).toBeLessThanOrEqual(8)
})

test('every screen exists: a known clip or a docs screenshot', () => {
  for (const v of VIDEOS) {
    for (const s of v.scenes) {
      if (s.visual.kind === 'clip') expect(CLIPS as readonly string[], s.id).toContain(s.visual.src)
      if (s.visual.kind === 'shot') expect(existsSync(`../docs/images/${s.visual.src}.jpg`), s.id).toBe(true)
    }
  }
})

test('no prices and no tech words on screen or in the voice', () => {
  const banned = /\b(rs\.?|lkr|plugin|database|server|cloud|ai|react|demo room)\b/i
  for (const lang of LANGS) {
    for (const [id, l] of Object.entries(LINES[lang])) expect(`${l.caption} ${l.voice}`, `${lang} ${id}`).not.toMatch(banned)
  }
})
