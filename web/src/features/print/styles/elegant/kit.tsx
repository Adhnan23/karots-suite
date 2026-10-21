import { SERIF_FAMILY, SERIF_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Logo, LR, NumberCode, Para, Paper, pretty, Space } from '../parts'

// Elegant: serif, centred, diamond ornaments, item names in italic, the total
// centred between double rules. Ornaments are drawn, not glyphs (no font has to have ◆).

const small = { fontSize: 17, lineHeight: '23px' }
const Diamond = () => <span style={{ display: 'inline-block', width: 10, height: 10, background: '#000', transform: 'rotate(45deg)' }} />

/** ◆ ─── ◆ ─── ◆ */
function Ornament({ lines = true }: { lines?: boolean }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 12, margin: '10px 0' }}>
      <Diamond />
      {lines && (
        <>
          <span style={{ width: 80, borderTop: '3px solid #000' }} />
          <Diamond />
          <span style={{ width: 80, borderTop: '3px solid #000' }} />
          <Diamond />
        </>
      )}
    </div>
  )
}

const Double = () => <div style={{ borderTop: '6px double #000', margin: '8px 0' }} />

export const elegant: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={SERIF_FAMILY} line={28}>
      <Space h={6} />
      {children}
    </Paper>
  ),
  Header: ({ shop, dots }) => (
    <>
      <Logo shop={shop} dots={dots} />
      <Para center style={{ fontSize: 40, lineHeight: '48px', letterSpacing: 3 }}>
        {shop.name}
      </Para>
      {shop.nameLocal && (
        <Para center style={{ fontSize: Math.floor(dots / 16), lineHeight: 1.4 }}>
          {shop.nameLocal}
        </Para>
      )}
      {[shop.address, shop.phone && `Tel. ${shop.phone}`, shop.vatNo && `VAT ${shop.vatNo}`].filter(Boolean).map((s, i) => (
        <Para key={i} center style={{ ...small, fontStyle: 'italic' }}>
          {s}
        </Para>
      ))}
      <Ornament />
    </>
  ),
  Title: ({ children }) => (
    <Para center style={{ letterSpacing: 5, textTransform: 'uppercase', margin: '4px 0 8px' }}>
      {children}
    </Para>
  ),
  Rule: () => <div style={{ borderTop: '2px solid #000', margin: '10px 80px' }} />,
  Gap: () => <Space h={8} />,
  Info: ({ l, r }) => <LR drop l={l.replace(/:$/, '')} r={r} style={small} />,
  Item: ({ name, amount, children }) => (
    <div style={{ marginTop: 6 }}>
      <LR l={<i>{name}</i>} r={amount} />
      {children}
    </div>
  ),
  Sub: ({ l, r, kind }) => {
    if (kind === 'total' || kind === 'given') return null
    if (kind === 'saving') return <Para style={{ ...small, fontStyle: 'italic' }}>{`${l.replace('You saved', 'saved')} ${r.replace(/^-/, '')}`}</Para>
    return <LR l={pretty(l)} r={r} style={small} />
  },
  Member: ({ children }) => <Para style={{ ...small, paddingLeft: 20 }}>{pretty(children)}</Para>,
  Note: ({ children, level, kind }) => (
    <Para style={{ ...small, paddingLeft: level === 1 ? 0 : 20 }}>{kind === 'serial' ? `serial ${children}` : children}</Para>
  ),
  Money: ({ l, r, bold }) => <LR l={l} r={r} style={{ ...small, fontWeight: bold ? 700 : 400 }} />,
  Total: ({ label, amount }) => (
    <div style={{ textAlign: 'center', margin: '4px 0' }}>
      <Double />
      <div style={{ ...small, letterSpacing: 6 }}>{label}</div>
      <div style={{ fontSize: 44, lineHeight: '54px', whiteSpace: 'nowrap' }}>{amount}</div>
      <Double />
    </div>
  ),
  Text: ({ children, center, bold }) => (
    <Para center={center} bold={bold}>
      {children}
    </Para>
  ),
  Footer: ({ shop, number }) => (
    <>
      <Ornament lines={false} />
      {shop.footer && (
        <Para center style={small}>
          {shop.footer}
        </Para>
      )}
      <Para center style={{ fontStyle: 'italic', marginTop: 4 }}>
        With thanks, until next time.
      </Para>
      <NumberCode number={number} />
      <Credit style={{ marginTop: 8 }} />
    </>
  ),
  fonts: [...SERIF_FONTS],
}
