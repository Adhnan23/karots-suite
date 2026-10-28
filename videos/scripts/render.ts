// bun run render [--lang si] [--video credit]
// Renders out/<lang>/karots-suite-<video>-<lang>.mp4 for every video × language,
// then checks each one (size, length) and prints a table. Exits non-zero when a
// video is the wrong size or far off its length, or a file is missing.
import { $ } from 'bun'
import { copyFileSync, existsSync, mkdirSync } from 'node:fs'
import { dirname } from 'node:path'
import { CLIPS } from '../src/clips'
import { LANGS, type Lang } from '../src/types'
import { VIDEOS } from '../src/videos'

const opt = (k: string) => (process.argv.includes(k) ? process.argv[process.argv.indexOf(k) + 1] : undefined)
const langs = opt('--lang') ? [opt('--lang') as Lang] : LANGS
const videos = VIDEOS.filter((v) => !opt('--video') || v.id === opt('--video'))

// The logo and the docs screenshots the scenes use, copied in (public/ is what Remotion serves).
copyFileSync('../docs/images/logo.jpg', 'public/logo.jpg')
for (const v of VIDEOS) {
  for (const s of v.scenes) {
    if (s.visual.kind !== 'shot') continue
    const to = `public/shots/${s.visual.src}.jpg`
    mkdirSync(dirname(to), { recursive: true })
    copyFileSync(`../docs/images/${s.visual.src}.jpg`, to)
  }
}
const missing = CLIPS.filter((c) => !existsSync(`public/clips/${c}.webm`))
if (missing.length) throw new Error(`clips missing (make videos-clips): ${missing.join(', ')}`)
const music = existsSync('public/music.mp3')
if (!music) console.warn('! no public/music.mp3 — rendering without music')

const rows: string[][] = []
let bad = 0
for (const lang of langs) {
  mkdirSync(`out/${lang}`, { recursive: true })
  for (const v of videos) {
    const file = `out/${lang}/karots-suite-${v.id}-${lang}.mp4`
    const props = JSON.stringify({ video: v.id, lang, music })
    await $`bunx remotion render src/index.ts ${`${v.id}-${lang}`} ${file} --props=${props} --log=error`
    const [w, h, secs] = (await $`ffprobe -v error -select_streams v:0 -show_entries stream=width,height:format=duration -of csv=p=0:s=, ${file}`.text())
      .split(/[\n,]/).filter(Boolean).map(Number)
    const [lo, hi] = v.id === 'promo' ? [55, 100] : [10, 25]
    const ok = w === 1080 && h === 1920 && secs >= lo && secs <= hi
    if (!ok) bad++
    rows.push([file, `${w}×${h}`, `${secs.toFixed(1)} s`, `${((await Bun.file(file).size) / 1e6).toFixed(1)} MB`, ok ? 'ok' : `OUT OF RANGE (${lo}–${hi} s)`])
  }
}
console.table(rows)
process.exit(bad ? 1 : 0)
