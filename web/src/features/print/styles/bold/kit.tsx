import { SANS_FAMILY, SANS_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Line, Logo, LR, MARGIN, NumberCode, Para, Paper, pretty, Space, Tag } from '../parts'

// Bold: a black header band, clean sans, savings as tags, a solid black TOTAL
// band, the slip number as a barcode at the foot.

const small = { fontSize: 17, lineHeight: '22px' }
const band = { background: '#000', color: '#fff', margin: `0 -${MARGIN}px`, padding: `12px ${MARGIN}px` }

export const bold: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={SANS_FAMILY}>
      {children}
    </Paper>
  ),
  Header: ({ shop, dots }) => (
    <>
      <Logo shop={shop} dots={dots} />
      <div style={{ ...band, padding: `18px ${MARGIN}px 14px`, textAlign: 'center', marginBottom: 12 }}>
        <Para center bold style={{ fontSize: 40, lineHeight: '46px', letterSpacing: 1, textTransform: 'uppercase' }}>
          {shop.name}
        </Para>
        {shop.nameLocal && <Para center style={{ fontSize: Math.floor(dots / 16), lineHeight: 1.4 }}>{shop.nameLocal}</Para>}
        {[shop.address, shop.phone && `Tel: ${shop.phone}`, shop.vatNo && `VAT: ${shop.vatNo}`].filter(Boolean).map((s, i) => (
          <Para key={i} center style={small}>
            {s}
          </Para>
        ))}
      </div>
    </>
  ),
  Title: ({ children }) => (
    <div style={{ ...band, padding: `6px ${MARGIN}px`, margin: `8px -${MARGIN}px`, textAlign: 'center', fontWeight: 900, letterSpacing: 3, textTransform: 'uppercase' }}>{children}</div>
  ),
  Rule: () => <Line w={2} />,
  Gap: () => <Space h={10} />,
  Info: ({ l, r }) => <LR drop l={l} r={r} style={small} />,
  Item: ({ name, amount, children }) => (
    <div style={{ marginTop: 8 }}>
      <LR l={<b>{name}</b>} r={<b>{amount}</b>} />
      {children}
    </div>
  ),
  Sub: ({ l, r, kind }) => {
    if (kind === 'total') return null // on the name row
    if (kind === 'saving') return <div style={{ ...small, margin: '2px 0' }}><Tag>{`saved ${r.replace(/^-/, '')}${l.replace('You saved', '')}`}</Tag></div>
    return <LR l={pretty(l)} r={kind === 'given' ? undefined : r} style={small} />
  },
  Member: ({ children }) => <Para style={{ ...small, paddingLeft: 16 }}>{`· ${pretty(children)}`}</Para>,
  Note: ({ children, level, kind }) => <Para style={{ ...small, paddingLeft: level === 1 ? 0 : 16 }}>{kind === 'serial' ? `SN ${children}` : children}</Para>,
  Money: ({ l, r, bold: b }) => <LR l={l} r={r} style={b ? { fontWeight: 700 } : undefined} />,
  Total: ({ label, amount }) =>
    /^(TOTAL|GIVEN)/.test(label) ? (
      <LR l={label} r={amount} style={{ ...band, margin: `6px -${MARGIN}px`, fontSize: 32, lineHeight: '40px', fontWeight: 900 }} />
    ) : (
      <LR l={label} r={amount} style={{ fontSize: 26, lineHeight: '34px', fontWeight: 700 }} />
    ),
  Text: ({ children, center, bold: b }) => (
    <Para center={center} bold={b}>
      {children}
    </Para>
  ),
  Footer: ({ shop, number }) => (
    <>
      <NumberCode number={number} />
      {shop.footer && <Para center style={{ marginTop: 8 }}>{shop.footer}</Para>}
      <Para center bold style={{ marginTop: 8 }}>Thank you! Come again.</Para>
      <Credit style={{ marginTop: 10 }} />
    </>
  ),
  fonts: [...SANS_FONTS],
}
