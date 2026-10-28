import { AbsoluteFill, Img, spring, staticFile, useCurrentFrame, useVideoConfig } from 'remotion'
import { display, fontFor } from './fonts'
import { LINES } from './lines'
import { SAFE, t } from './theme'
import type { Lang } from './types'

export const PHONE_NUMBER = '076 962 6396'

// Every video ends here: who we are, Pocket Cashier coming soon (a drawn phone,
// since the app isn't built yet), and the one thing to do — WhatsApp us.
export function EndCard({ lang }: { lang: Lang }) {
  const frame = useCurrentFrame()
  const { fps } = useVideoConfig()
  const at = (delay: number) => spring({ frame: frame - delay, fps, config: { damping: 200 } })
  const rise = (delay: number): React.CSSProperties => ({ opacity: at(delay), transform: `translateY(${(1 - at(delay)) * 50}px)` })
  const l = LINES[lang]
  return (
    <AbsoluteFill style={{ background: t.bg, alignItems: 'center', paddingTop: SAFE.top - 40 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 28, ...rise(0) }}>
        <Img src={staticFile('logo.jpg')} style={{ width: 120, height: 120, borderRadius: 30 }} />
        <span style={{ fontFamily: display, fontWeight: 800, fontSize: 84, color: t.ink }}>Karots Suite</span>
      </div>

      <div style={{ marginTop: 50, display: 'flex', flexDirection: 'column', alignItems: 'center', ...rise(8) }}>
        <div style={{ fontFamily: fontFor(lang), fontWeight: 800, fontSize: 46, color: t.ink, background: t.brand, padding: '12px 34px', borderRadius: 999, zIndex: 1 }}>
          {l['soon-title'].caption}
        </div>
        <Phone />
        <div style={{ marginTop: 26, width: 860, textAlign: 'center', fontFamily: fontFor(lang), fontWeight: 600, fontSize: 40, lineHeight: 1.3, color: t.muted, textWrap: 'balance' }}>
          {l['end-soon'].caption}
        </div>
      </div>

      <div style={{
        position: 'absolute', bottom: SAFE.bottom, left: SAFE.side, right: SAFE.side, padding: '30px 36px', borderRadius: t.radius,
        background: t.whatsapp, color: '#fff', display: 'flex', alignItems: 'center', gap: 30, ...rise(30),
      }}>
        <WhatsApp size={110} />
        <div>
          <div style={{ fontFamily: fontFor(lang), fontWeight: 800, fontSize: 42, lineHeight: 1.2 }}>{l.end.caption}</div>
          <div style={{ fontFamily: display, fontWeight: 800, fontSize: 64, marginTop: 6 }}>{PHONE_NUMBER}</div>
        </div>
      </div>
    </AbsoluteFill>
  )
}

/** A phone with a little till on it: a few lines, a total, a Pay button. */
function Phone() {
  const frame = useCurrentFrame()
  const rows = [['Samba Rice 1kg', '265.00'], ['Anchor Milk 400g', '1,150.00'], ['Sunlight Soap', '95.00']]
  const shown = Math.min(rows.length, Math.floor(Math.max(0, frame - 20) / 12))
  return (
    <div style={{ marginTop: -26, width: 380, height: 640, borderRadius: 64, background: t.dark, padding: 16, boxShadow: '0 30px 70px rgba(21,22,26,.3)' }}>
      <div style={{ width: '100%', height: '100%', borderRadius: 50, background: t.card, padding: '64px 26px 26px', display: 'flex', flexDirection: 'column', fontFamily: '"Figtree", sans-serif' }}>
        <div style={{ fontFamily: display, fontWeight: 800, fontSize: 30, color: t.ink }}>New sale</div>
        <div style={{ marginTop: 18, display: 'flex', flexDirection: 'column', gap: 12 }}>
          {rows.slice(0, shown).map(([n, p]) => (
            <div key={n} style={{ display: 'flex', justifyContent: 'space-between', padding: '14px 16px', borderRadius: 16, background: t.bg, fontSize: 22, fontWeight: 600, color: t.ink }}>
              <span>{n}</span><span>{p}</span>
            </div>
          ))}
        </div>
        <div style={{ marginTop: 'auto', display: 'flex', justifyContent: 'space-between', fontSize: 26, fontWeight: 800, color: t.ink }}>
          <span>Total</span><span>{shown === 3 ? '1,510.00' : '—'}</span>
        </div>
        <div style={{ marginTop: 16, padding: '20px 0', borderRadius: 999, background: t.brand, textAlign: 'center', fontSize: 28, fontWeight: 800, color: t.brandInk }}>Pay</div>
      </div>
    </div>
  )
}

function WhatsApp({ size }: { size: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 32 32" aria-hidden>
      <path fill="#fff" d="M16 3C8.8 3 3 8.7 3 15.8c0 2.5.7 4.9 2 7L3 29l6.4-2c2 1.1 4.3 1.7 6.6 1.7 7.2 0 13-5.7 13-12.8S23.2 3 16 3Zm0 23.4c-2.1 0-4.1-.6-5.9-1.7l-.4-.3-3.8 1.2 1.2-3.7-.3-.4a10.4 10.4 0 0 1-1.7-5.7C5.1 9.9 10 5.2 16 5.2s10.9 4.7 10.9 10.6S22 26.4 16 26.4Zm6-7.9c-.3-.2-1.9-1-2.2-1-.3-.1-.5-.2-.7.2l-1 1.2c-.2.2-.4.2-.7.1-.3-.2-1.4-.5-2.6-1.6-1-.9-1.6-1.9-1.8-2.2-.2-.3 0-.5.1-.7l.5-.6.3-.5c.1-.2 0-.4 0-.6l-1-2.4c-.3-.6-.5-.5-.7-.5h-.6c-.2 0-.6.1-.9.4-.3.3-1.1 1.1-1.1 2.7s1.2 3.1 1.3 3.3c.2.2 2.3 3.5 5.6 4.9 2.8 1.1 3.3.9 3.9.8.6-.1 1.9-.8 2.2-1.5.3-.7.3-1.4.2-1.5-.1-.2-.3-.2-.6-.4Z" />
    </svg>
  )
}
