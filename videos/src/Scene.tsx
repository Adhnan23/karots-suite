import { AbsoluteFill, Img, OffthreadVideo, interpolate, spring, staticFile, useCurrentFrame, useVideoConfig } from 'remotion'
import { fontFor } from './fonts'
import { LINES } from './lines'
import { SAFE, W, t } from './theme'
import type { Lang, Scene as SceneT } from './types'

// One scene: the caption on top, the screen in a card below it. The camera pans
// slowly across the screen (down a phone page, along the till) unless the scene
// pins a focus.

const CARD = { top: 640, height: 930, side: 56 }
// Till clips are recorded at 1440 wide: pan along them. Phone clips and docs
// shots are taller than the card: pan down them.
const DESK_CLIPS = ['lowstock', 'till-close', 'till-lock', 'sell', 'search-local', 'hold']

export function captionSize(text: string): number {
  return text.length > 34 ? 66 : text.length > 22 ? 76 : 88
}

export function Caption({ lang, text, color = t.ink, top = SAFE.top }: { lang: Lang; text: string; color?: string; top?: number }) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const s = spring({ frame, fps, config: { damping: 200 } })
  return (
    <div style={{
      position: 'absolute', top, left: SAFE.side, right: SAFE.side,
      fontFamily: fontFor(lang), fontWeight: 800, fontSize: captionSize(text), lineHeight: 1.18, color,
      opacity: s, transform: `translateY(${(1 - s) * 40}px)`, textWrap: 'balance',
    }}>
      {text}
      <div style={{ marginTop: 26, width: 140 * s, height: 12, borderRadius: 6, background: t.brand }} />
    </div>
  )
}

function Pill({ lang, text, bg, color, style }: { lang: Lang; text: string; bg: string; color: string; style: React.CSSProperties }) {
  return (
    <div style={{
      position: 'absolute', padding: '14px 30px', borderRadius: 999, background: bg, color,
      fontFamily: fontFor(lang), fontWeight: 800, fontSize: 38, ...style,
    }}>{text}</div>
  )
}

export function Scene({ scene, lang }: { scene: SceneT; lang: Lang }) {
  const frame = useCurrentFrame()
  const { fps, durationInFrames } = useVideoConfig()
  const v = scene.visual
  if (v.kind === 'problem') return <Problem lang={lang} text={LINES[lang][scene.id].caption} />
  const lift = spring({ frame: frame - 4, fps, config: { damping: 200 } })
  const p = interpolate(frame, [0, durationInFrames], [0, 1], { extrapolateRight: 'clamp' })
  const desk = v.kind === 'clip' && DESK_CLIPS.includes(v.src)
  const f = v.focus
  const media: React.CSSProperties = {
    width: '100%', height: '100%', objectFit: 'cover',
    objectPosition: f ? `${f.x * 100}% ${f.y * 100}%` : desk ? `${p * 100}% 50%` : `50% ${p * 30}%`,
    transform: f ? `scale(${1 + (f.zoom - 1) * p})` : undefined,
    transformOrigin: f ? `${f.x * 100}% ${f.y * 100}%` : undefined,
  }
  const lines = LINES[lang]
  return (
    <AbsoluteFill style={{ background: t.bg }}>
      {scene.addon && (
        <Pill lang={lang} text={lines['addon-tag'].caption} bg={t.brandSoft} color={t.ink} style={{ top: SAFE.top - 86, left: SAFE.side }} />
      )}
      <Caption lang={lang} text={lines[scene.id].caption} />
      <div style={{
        position: 'absolute', top: CARD.top, left: CARD.side, width: W - CARD.side * 2, height: CARD.height,
        borderRadius: t.radius, overflow: 'hidden', background: t.card, border: `2px solid ${t.border}`,
        boxShadow: '0 30px 80px rgba(21,22,26,.18)', opacity: lift, transform: `translateY(${(1 - lift) * 120}px)`,
      }}>
        {v.kind === 'clip'
          ? <OffthreadVideo muted src={staticFile(`clips/${v.src}.webm`)} startFrom={Math.round((v.from ?? 0) * fps)} style={media} />
          : <Img src={staticFile(`shots/${v.src}.jpg`)} style={media} />}
        {scene.note === 'internet' && (
          <Pill lang={lang} text={`📶 ${lines['internet-note'].caption}`} bg="rgba(21,22,26,.86)" color="#fff" style={{ bottom: 28, left: 28, fontSize: 32 }} />
        )}
      </div>
    </AbsoluteFill>
  )
}

/** The owner's headache, big, on the dark brand colour. */
export function Problem({ lang, text }: { lang: Lang; text: string }) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const s = spring({ frame, fps, config: { damping: 14, mass: 0.6 } })
  return (
    <AbsoluteFill style={{ background: t.dark, justifyContent: 'center', padding: `0 ${SAFE.side}px` }}>
      <div style={{
        fontFamily: fontFor(lang), fontWeight: 800, fontSize: captionSize(text) * 1.25, lineHeight: 1.15, color: '#fff',
        transform: `scale(${0.85 + 0.15 * s})`, opacity: Math.min(1, s * 1.4), textWrap: 'balance',
      }}>
        {text}
      </div>
      <div style={{ marginTop: 40, width: 200 * s, height: 16, borderRadius: 8, background: t.brand }} />
    </AbsoluteFill>
  )
}
