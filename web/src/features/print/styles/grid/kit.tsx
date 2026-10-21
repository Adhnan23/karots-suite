import { SANS_FAMILY, SANS_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Line, Logo, LR, MARGIN, NumberCode, Para, Paper, pretty, Space } from '../parts'

// Grid: an invoice — the shop name left and the slip number right, ruled rows,
// a black table head (the sale is a real QTY / ITEM / OFF / AMT table — sale.tsx),
// totals in a block on the right with a black TOTAL row.

export const small = { fontSize: 17, lineHeight: '22px' }
export const cell = { borderBottom: '2px solid #000', padding: '3px 0' }

export const grid: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={SANS_FAMILY}>
      {children}
    </Paper>
  ),
  Header: ({ shop, dots, number }) => (
    <>
      <Logo shop={shop} dots={dots} />
      <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'space-between', alignItems: 'flex-end', columnGap: 12, marginTop: 4 }}>
        <div style={{ minWidth: 0, overflowWrap: 'break-word' }}>
          <div style={{ fontWeight: 900, fontSize: 32, lineHeight: '36px' }}>{shop.name}</div>
          {shop.nameLocal && <div style={{ fontSize: Math.floor(dots / 18), lineHeight: 1.4 }}>{shop.nameLocal}</div>}
        </div>
        {number && (
          <div style={{ ...small, textAlign: 'right', marginLeft: 'auto' }}>
            <div style={{ letterSpacing: 2 }}>RECEIPT</div>
            <div style={{ fontWeight: 700, fontSize: 22 }}>{number}</div>
          </div>
        )}
      </div>
      {[shop.address, shop.phone && `Tel: ${shop.phone}`, shop.vatNo && `VAT: ${shop.vatNo}`].filter(Boolean).map((s, i) => (
        <Para key={i} style={small}>
          {s}
        </Para>
      ))}
    </>
  ),
  Title: ({ children }) => (
    <div
      style={{
        background: '#000',
        color: '#fff',
        fontWeight: 700,
        letterSpacing: 2,
        textTransform: 'uppercase',
        padding: `4px ${MARGIN}px`,
        margin: '8px 0 0',
      }}
    >
      {children}
    </div>
  ),
  Rule: () => <Line w={4} gap={6} />,
  Gap: () => <Space h={6} />,
  Info: ({ l, r }) => <LR drop l={l} r={r} style={{ ...small, ...cell }} />,
  Item: ({ name, amount, children }) => (
    <div style={cell}>
      <LR l={<b>{name}</b>} r={<b>{amount}</b>} />
      <div style={small}>{children}</div>
    </div>
  ),
  Sub: ({ l, r, kind }) => (kind === 'total' || kind === 'given' ? null : <LR l={pretty(l)} r={r} />),
  Member: ({ children }) => <Para style={small}>{`· ${pretty(children)}`}</Para>,
  Note: ({ children }) => <Para style={small}>{children}</Para>,
  Money: ({ l, r, bold }) => <LR l={l} r={r} style={{ ...cell, borderBottomWidth: 1, fontWeight: bold ? 700 : 400 }} />,
  Total: ({ label, amount }) => (
    <LR
      l={label}
      r={amount}
      style={{ background: '#000', color: '#fff', fontWeight: 900, fontSize: 24, lineHeight: '32px', padding: '4px 8px', margin: '4px 0' }}
    />
  ),
  Text: ({ children, center, bold }) => (
    <Para center={center} bold={bold}>
      {children}
    </Para>
  ),
  Footer: ({ shop, number }) => (
    <>
      <NumberCode number={number} />
      {shop.footer && (
        <Para center style={{ ...small, marginTop: 6 }}>
          {shop.footer}
        </Para>
      )}
      <Para center style={{ marginTop: 6 }}>
        Thank you!
      </Para>
      <Credit style={{ marginTop: 8 }} />
    </>
  ),
  fonts: [...SANS_FONTS],
}
