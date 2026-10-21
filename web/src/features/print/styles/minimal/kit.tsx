import { Code128, QR } from '../../Barcode'
import { SANS_FAMILY, SANS_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Leader, Line, Logo, LR, Para, Paper, pretty, Space } from '../parts'

// Minimal: lots of white, spaced capitals, thin rules, prices on dotted leaders,
// a big quiet total; the slip number as a barcode and a QR at the foot.

const small = { fontSize: 17, lineHeight: '22px' }
const caps = { ...small, letterSpacing: 2, textTransform: 'uppercase' as const }

export const minimal: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={SANS_FAMILY} line={28}>
      <Space h={8} />
      {children}
    </Paper>
  ),
  Header: ({ shop, dots }) => (
    <>
      <Logo shop={shop} dots={dots} />
      <Para center style={{ fontSize: 34, lineHeight: '42px', letterSpacing: 6, textTransform: 'uppercase' }}>
        {shop.name}
      </Para>
      {shop.nameLocal && <Para center style={{ fontSize: Math.floor(dots / 18), lineHeight: 1.4 }}>{shop.nameLocal}</Para>}
      {[shop.address, shop.phone, shop.vatNo && `VAT ${shop.vatNo}`].filter(Boolean).map((s, i) => (
        <Para key={i} center style={caps}>
          {s}
        </Para>
      ))}
      <Space h={16} />
    </>
  ),
  Title: ({ children }) => (
    <Para center style={{ ...caps, letterSpacing: 6, fontWeight: 700, margin: '10px 0' }}>
      {children}
    </Para>
  ),
  Rule: () => <Line w={2} gap={12} />,
  Gap: () => <Space h={12} />,
  Info: ({ l, r }) => <LR drop l={<span style={caps}>{l.replace(/:$/, '')}</span>} r={r} style={small} />,
  Item: ({ name, amount, children }) => (
    <div style={{ marginTop: 6 }}>
      <Leader l={name} r={amount} />
      <div style={{ paddingLeft: 24 }}>{children}</div>
    </div>
  ),
  Sub: ({ l, r, kind }) => {
    if (kind === 'total' || kind === 'given') return null // on the leader
    if (kind === 'saving') return <Para style={small}>{`less ${r.replace(/^-/, '')}${l.replace('You saved', '')}`}</Para>
    return <LR l={pretty(l)} r={r} style={small} />
  },
  Member: ({ children }) => <Para style={small}>{pretty(children)}</Para>,
  Note: ({ children, level, kind }) => <Para style={{ ...small, paddingLeft: level === 1 ? 0 : 20 }}>{kind === 'serial' ? `SN ${children}` : children}</Para>,
  Money: ({ l, r, bold: b }) => <LR l={<span style={caps}>{l}</span>} r={r} style={{ ...small, fontWeight: b ? 700 : 400 }} />,
  Total: ({ label, amount }) => (
    <LR l={<span style={caps}>{label}</span>} r={<span style={{ fontSize: 44, lineHeight: '54px' }}>{amount}</span>} style={{ margin: '6px 0' }} />
  ),
  Text: ({ children, center, bold: b }) => (
    <Para center={center} bold={b} style={small}>
      {children}
    </Para>
  ),
  Footer: ({ shop, number }) => (
    <>
      {number && (
        <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', justifyContent: 'center', gap: 8, margin: '16px 0 4px' }}>
          <div>
            <Code128 value={number} height={56} />
            <div style={{ ...caps, textAlign: 'center' }}>{number}</div>
          </div>
          <QR value={number} size={96} />
        </div>
      )}
      {shop.footer && <Para center style={{ ...small, marginTop: 10 }}>{shop.footer}</Para>}
      <Para center style={{ ...caps, letterSpacing: 6, marginTop: 12 }}>Thank you</Para>
      <Credit style={{ marginTop: 12 }} />
    </>
  ),
  fonts: [...SANS_FONTS],
}
