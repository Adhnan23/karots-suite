import { Composition } from 'remotion'
import { FPS, H, W } from './theme'
import { LANGS } from './types'
import { Video, timeline, type VideoProps } from './Video'
import { VIDEOS } from './videos'

// One composition per video per language: <video>-<lang>, e.g. credit-si.
// Length follows the voice lines (src/durations.json).
export function Root() {
  return (
    <>
      {VIDEOS.flatMap((v) => LANGS.map((lang) => (
        <Composition
          key={`${v.id}-${lang}`}
          id={`${v.id}-${lang}`}
          component={Video}
          defaultProps={{ video: v.id, lang, music: false } satisfies VideoProps}
          calculateMetadata={({ props }) => ({ durationInFrames: timeline(props.video, props.lang).total })}
          durationInFrames={1}
          fps={FPS}
          width={W}
          height={H}
        />
      )))}
    </>
  )
}
