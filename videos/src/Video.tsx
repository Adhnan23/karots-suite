import { AbsoluteFill, Audio, Sequence, interpolate, staticFile } from 'remotion'
import durations from './durations.json'
import { EndCard } from './EndCard'
import { Scene } from './Scene'
import { FPS } from './theme'
import type { Lang, VideoId } from './types'
import { VIDEOS } from './videos'

export type VideoProps = { video: VideoId; lang: Lang; music: boolean }

const PAD = 0.4 // breath after each voice line
const MIN = 1.8 // a montage scene still gets time to be read
const LEAD = 0.25 // the voice starts just after the scene lands

function voiceSeconds(lang: Lang, id: string): number {
  const s = (durations as Record<string, Record<string, number>>)[lang]?.[id]
  if (!s) throw new Error(`no voice for ${lang} ${id} — run bun run voice`)
  return s
}
const frames = (s: number) => Math.round(s * FPS)

/** Each scene's length (frames) and the end card's, from its voice lines. */
export function timeline(video: VideoId, lang: Lang) {
  const v = VIDEOS.find((x) => x.id === video)!
  const scenes = v.scenes.map((s) => ({ scene: s, len: frames(Math.max(MIN, LEAD + voiceSeconds(lang, s.id) + PAD)) }))
  const soon = voiceSeconds(lang, 'end-soon')
  const end = frames(LEAD + soon + 0.3 + voiceSeconds(lang, 'end') + 1.6)
  return { scenes, end, soon: frames(LEAD + soon + 0.3), total: scenes.reduce((n, s) => n + s.len, 0) + end }
}

export function Video({ video, lang, music }: VideoProps) {
  const tl = timeline(video, lang)
  let at = 0
  const endAt = tl.total - tl.end
  return (
    <AbsoluteFill>
      {tl.scenes.map(({ scene, len }) => {
        const from = at
        at += len
        return (
          <Sequence key={`${scene.id}-${from}`} from={from} durationInFrames={len}>
            <Scene scene={scene} lang={lang} />
            <Sequence from={frames(LEAD)}><Audio src={staticFile(`voice/${lang}/${scene.id}.mp3`)} /></Sequence>
          </Sequence>
        )
      })}
      <Sequence from={endAt}>
        <EndCard lang={lang} />
        <Sequence from={frames(LEAD)}><Audio src={staticFile(`voice/${lang}/end-soon.mp3`)} /></Sequence>
        <Sequence from={tl.soon}><Audio src={staticFile(`voice/${lang}/end.mp3`)} /></Sequence>
      </Sequence>
      {music && (
        <Audio loop src={staticFile('music.mp3')}
          volume={(f) => interpolate(f, [0, 8, endAt, endAt + 15, tl.total - 20, tl.total], [0, 0.12, 0.12, 0.3, 0.3, 0], { extrapolateRight: 'clamp' })} />
      )}
    </AbsoluteFill>
  )
}
