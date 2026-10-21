import { PAPER_FAMILY, PAPER_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Line, Logo, LR, MARGIN, NumberCode, Para, Paper, pretty, Space } from '../parts'

// Sections: the slip in labelled parts under black bars (ITEMS, YOU SAVED,
// PAYMENT — sale.tsx), so each part is found at a glance.

const small = { fontSize: 17, lineHeight: '22px' }

/** A black section bar: ■ LABEL. */
export function Bar({ children }: { children: string }) {
  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: 10,
        background: '#000',
        color: '#fff',
        fontWeight: 700,
        letterSpacing: 3,
        textTransform: 'uppercase',
        margin: `12px -${MARGIN}px 6px`,
        padding: `3px ${MARGIN}px`,
      }}
    >
      <span style={{ width: 12, height: 12, background: '#fff', flexShrink: 0 }} />
      {children}
    </div>
  )
}

export const sections: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={PAPER_FAMILY} line={24}>
      {children}
    </Paper>
  ),
  Header: ({ shop, dots }) => (
    <>
      <Logo shop={shop} dots={dots} />
      <Para center bold style={{ fontSize: 32, lineHeight: '40px', marginTop: 6 }}>
        {shop.name}
      </Para>
      {shop.nameLocal && (
        <Para center style={{ fontSize: Math.floor(dots / 16), lineHeight: 1.4 }}>
          {shop.nameLocal}
        </Para>
      )}
      {[shop.address, shop.phone && `Tel: ${shop.phone}`, shop.vatNo && `VAT: ${shop.vatNo}`].filter(Boolean).map((s, i) => (
        <Para key={i} center style={small}>
          {s}
        </Para>
      ))}
    </>
  ),
  Title: ({ children }) => <Bar>{children}</Bar>,
  Rule: () => <Line w={2} kind="dashed" gap={6} />,
  Gap: () => <Space h={6} />,
  Info: ({ l, r }) => <LR drop l={l} r={r} style={small} />,
  Item: ({ name, amount, children }) => (
    <div style={{ marginTop: 4 }}>
      <LR l={name} r={amount} />
      <div style={{ ...small, paddingLeft: 24 }}>{children}</div>
    </div>
  ),
  Sub: ({ l, r, kind }) => (kind === 'total' || kind === 'given' ? null : <LR l={pretty(l)} r={r} />),
  Member: ({ children }) => <Para>{`- ${pretty(children)}`}</Para>,
  Note: ({ children }) => <Para>{children}</Para>,
  Money: ({ l, r, bold }) => <LR l={l} r={r} style={bold ? { fontWeight: 700 } : undefined} />,
  Total: ({ label, amount }) => <LR l={label} r={amount} style={{ fontWeight: 700, fontSize: 28, lineHeight: '36px', margin: '2px 0' }} />,
  Text: ({ children, center, bold }) => (
    <Para center={center} bold={bold}>
      {children}
    </Para>
  ),
  Footer: ({ shop, number }) => (
    <>
      <NumberCode number={number} />
      {shop.footer && (
        <Para center style={{ marginTop: 6 }}>
          {shop.footer}
        </Para>
      )}
      <Para center>Thank you! Come again.</Para>
      <Credit style={{ marginTop: 8 }} />
    </>
  ),
  fonts: [...PAPER_FONTS],
}
