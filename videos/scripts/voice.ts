// bun run voice [--lang si] [--force] [--samples]
// One mp3 per spoken line (public/voice/<lang>/<id>.mp3) through edge-tts, and
// src/durations.json with each one's length. Only lines whose text or voice
// changed are made again. Any failure stops the run: a silent scene never ships.
import { $ } from 'bun'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { LINES } from '../src/lines'
import { LANGS, type Lang } from '../src/types'
import { SAMPLE_VOICES, VOICE } from '../src/voices'

const arg = (k: string) => process.argv.includes(k)
const only = process.argv[process.argv.indexOf('--lang') + 1] as Lang | undefined
const langs = arg('--lang') && only ? [only] : LANGS
const DUR = 'src/durations.json'

// The voice service drops connections now and then: try a few times before giving up.
async function say(voice: string, text: string, file: string) {
  for (let i = 1; ; i++) {
    try {
      return await $`uvx edge-tts --voice ${voice} --text ${text} --write-media ${file}`.quiet()
    } catch (e) {
      if (i === 5) throw e
      await Bun.sleep(2000 * i)
    }
  }
}
async function seconds(file: string): Promise<number> {
  const out = await $`ffprobe -v error -show_entries format=duration -of csv=p=0 ${file}`.text()
  return Math.round(parseFloat(out) * 100) / 100
}

if (arg('--samples')) {
  mkdirSync('out/samples', { recursive: true })
  for (const lang of LANGS) {
    for (const v of SAMPLE_VOICES[lang]) {
      await say(v, LINES[lang]['credit-list'].voice, `out/samples/${v}.mp3`)
      console.log(`out/samples/${v}.mp3`)
    }
  }
  process.exit(0)
}

const durations: Record<string, Record<string, number>> = existsSync(DUR) ? JSON.parse(readFileSync(DUR, 'utf8')) : {}
for (const lang of langs) {
  const dir = `public/voice/${lang}`
  mkdirSync(dir, { recursive: true })
  const hashFile = `${dir}/.hashes.json`
  const hashes: Record<string, string> = existsSync(hashFile) ? JSON.parse(readFileSync(hashFile, 'utf8')) : {}
  const next: Record<string, number> = {}
  for (const [id, line] of Object.entries(LINES[lang])) {
    if (!line.voice) continue
    const file = `${dir}/${id}.mp3`
    const hash = Bun.hash(`${VOICE[lang]}|${line.voice}`).toString(36)
    if (arg('--force') || hashes[id] !== hash || !existsSync(file)) {
      await say(VOICE[lang], line.voice, file)
      hashes[id] = hash
      writeFileSync(hashFile, JSON.stringify(hashes, null, 1)) // a rerun after a failure skips what's done
      console.log(`${lang} ${id}`)
    }
    next[id] = await seconds(file)
  }
  durations[lang] = next
}
writeFileSync(DUR, JSON.stringify(durations, null, 1) + '\n')
