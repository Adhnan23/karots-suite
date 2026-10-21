import { Code128 } from '../../Barcode'
import { PAPER_FAMILY, PAPER_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Line, Logo, LR, Para, Paper, pretty, Space } from '../parts'

// Ticket: a ticket stub — the slip number big in a framed box at the top with
// its barcode (scan it for returns), framed totals, a tear line before the footer.

const small = { fontSize: 17, lineHeight: '22px' }
const frame = { border: '4px solid #000', borderRadius: 8, padding: '8px 12px' }

export const ticket: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={PAPER_FAMILY} line={24}>
      {children}
    </Paper>
  ),
  Header: ({ shop, dots, number }) => (
    <>
      <Logo shop={shop} dots={dots} />
      <Para center bold style={{ fontSize: 30, lineHeight: '38px' }}>
        {shop.name}
      </Para>
      {shop.nameLocal && <Para center style={{ fontSize: Math.floor(dots / 16), lineHeight: 1.4 }}>{shop.nameLocal}</Para>}
      {[shop.address, shop.phone && `Tel: ${shop.phone}`, shop.vatNo && `VAT: ${shop.vatNo}`].filter(Boolean).map((s, i) => (
        <Para key={i} center style={small}>
          {s}
        </Para>
      ))}
      {number && (
        <div style={{ ...frame, margin: '14px 0 6px', textAlign: 'center' }}>
          <div style={{ ...small, letterSpacing: 4 }}>RECEIPT</div>
          <div style={{ fontWeight: 700, fontSize: 36, lineHeight: '44px' }}>{number}</div>
          <div style={{ margin: '6px 0 2px' }}>
            <Code128 value={number} height={56} />
          </div>
        </div>
      )}
    </>
  ),
  Title: ({ children }) => (
    <Para center bold style={{ margin: '10px 0 4px', padding: '4px 0', letterSpacing: 3, textTransform: 'uppercase', borderTop: '4px solid #000', borderBottom: '4px solid #000' }}>
      {children}
    </Para>
  ),
  Rule: () => <Line w={2} kind="dashed" gap={8} />,
  Gap: () => <Space h={8} />,
  Info: ({ l, r }) => <LR drop l={l} r={r} style={small} />,
  Item: ({ name, amount, children }) => (
    <div style={{ marginTop: 6 }}>
      <LR l={<b>{name}</b>} r={<b>{amount}</b>} />
      <div style={{ paddingLeft: 24 }}>{children}</div>
    </div>
  ),
  Sub: ({ l, r, kind }) => (kind === 'total' || kind === 'given' ? null : <LR l={pretty(l)} r={r} style={small} />),
  Member: ({ children }) => <Para style={small}>{`+ ${pretty(children)}`}</Para>,
  Note: ({ children, level, kind }) => <Para style={{ ...small, paddingLeft: level === 1 ? 0 : 24 }}>{kind === 'serial' ? `SN ${children}` : children}</Para>,
  Money: ({ l, r, bold }) => <LR l={l} r={r} style={bold ? { fontWeight: 700 } : undefined} />,
  Total: ({ label, amount }) => <LR l={label} r={amount} style={{ ...frame, margin: '6px 0', fontWeight: 700, fontSize: 26, lineHeight: '32px' }} />,
  Text: ({ children, center, bold }) => (
    <Para center={center} bold={bold}>
      {children}
    </Para>
  ),
  Footer: ({ shop }) => (
    <>
      <div style={{ display: 'flex', alignItems: 'center', gap: 8, margin: '14px 0 8px' }}>
        <span style={{ flex: 1, borderTop: '2px dashed #000' }} />
        <span style={{ ...small, letterSpacing: 3 }}>TEAR HERE</span>
        <span style={{ flex: 1, borderTop: '2px dashed #000' }} />
      </div>
      {shop.footer && <Para center>{shop.footer}</Para>}
      <Para center>Thank you! Come again.</Para>
      <Para center style={small}>Keep this slip for returns &amp; warranty</Para>
      <Credit style={{ marginTop: 10 }} />
    </>
  ),
  fonts: [...PAPER_FONTS],
}
