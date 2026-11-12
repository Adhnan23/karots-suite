import { SANS_FAMILY, SANS_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Line, LR, MARGIN, Para, Paper, pretty, Space } from '../parts'

// Compact: the shortest slip (Iman, 2026-11-12: "receipts take long paper").
// The logo beside the shop's name, address and phone; each product one row
// (sale.tsx); tight lines; no barcode.

export const small = { fontSize: 17, lineHeight: '21px' }

export const compact: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={SANS_FAMILY} line={24}>
      {children}
    </Paper>
  ),
  Header: ({ shop, dots }) => {
    const lines = [shop.address, shop.phone && `Tel: ${shop.phone}`, shop.vatNo && `VAT: ${shop.vatNo}`].filter(Boolean)
    const details = (
      <div style={{ minWidth: 0, flex: 1, overflowWrap: 'break-word', textAlign: shop.logo ? 'left' : 'center' }}>
        <div style={{ fontWeight: 800, fontSize: 24, lineHeight: '28px' }}>{shop.name}</div>
        {shop.nameLocal && <div style={small}>{shop.nameLocal}</div>}
        {lines.map((s, i) => (
          <div key={i} style={small}>
            {s}
          </div>
        ))}
      </div>
    )
    if (!shop.logo) return details
    // The logo raster is the paper's full width; drawn here at about a third of it, beside the details.
    const w = Math.round((dots - 2 * MARGIN) * 0.36)
    return (
      <div style={{ display: 'flex', alignItems: 'center', gap: 10 }}>
        <img src={shop.logo} alt="" style={{ display: 'block', width: w, flexShrink: 0 }} />
        {details}
      </div>
    )
  },
  Title: ({ children }) => (
    <Para center bold style={{ textTransform: 'uppercase', letterSpacing: 1, margin: '4px 0' }}>
      {children}
    </Para>
  ),
  Rule: () => <Line w={2} gap={5} />,
  Gap: () => <Space h={4} />,
  Info: ({ l, r }) => <LR drop l={l} r={r} style={small} />,
  Item: ({ name, amount, children }) => (
    <div>
      <LR l={name} r={amount} />
      <div style={{ ...small, paddingLeft: 12 }}>{children}</div>
    </div>
  ),
  Sub: ({ l, r, kind }) => (kind === 'total' || kind === 'given' ? null : <LR l={pretty(l)} r={r} />),
  Member: ({ children }) => <Para style={small}>{`· ${pretty(children)}`}</Para>,
  Note: ({ children, level }) => <Para style={{ ...small, paddingLeft: level === 1 ? 0 : 12 }}>{children}</Para>,
  Money: ({ l, r, bold }) => <LR l={l} r={r} style={{ ...small, fontWeight: bold ? 700 : 400 }} />,
  Total: ({ label, amount }) => <LR l={label} r={amount} style={{ fontWeight: 800, fontSize: 24, lineHeight: '30px' }} />,
  Text: ({ children, center, bold }) => (
    <Para center={center} bold={bold} style={small}>
      {children}
    </Para>
  ),
  Footer: ({ shop }) => (
    <>
      {shop.footer && (
        <Para center style={{ ...small, marginTop: 4 }}>
          {shop.footer}
        </Para>
      )}
      <Para center bold style={{ marginTop: 2 }}>
        {shop.thanks || 'Thank you! Come again.'}
      </Para>
      <Credit style={{ marginTop: 4 }} />
    </>
  ),
  fonts: [...SANS_FONTS],
}
