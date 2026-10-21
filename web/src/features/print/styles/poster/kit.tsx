import { SANS_FAMILY, SANS_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Line, Logo, LR, NumberCode, Para, Paper, pretty, ShopLines, Space } from '../parts'

// Poster: heavy type, the shop name as a block at the top of every slip (the
// sale turns it sideways and leads with the amount — see sale.tsx), a quiet list.

const small = { fontSize: 17, lineHeight: '22px' }

export const poster: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={SANS_FAMILY}>
      {children}
    </Paper>
  ),
  Header: ({ shop, dots }) => (
    <>
      <Logo shop={shop} dots={dots} />
      <Para
        style={{
          fontWeight: 900,
          fontSize: 48,
          lineHeight: '52px',
          letterSpacing: 2,
          textTransform: 'uppercase',
          marginTop: 6,
        }}
      >
        {shop.name}
      </Para>
      {shop.nameLocal && <Para style={{ fontSize: Math.floor(dots / 16), lineHeight: 1.4 }}>{shop.nameLocal}</Para>}
      <ShopLines shop={shop} style={{ ...small, textAlign: 'left' }} />
      <Line w={6} gap={8} />
    </>
  ),
  Title: ({ children }) => (
    <Para
      style={{
        fontWeight: 900,
        fontSize: 32,
        lineHeight: '38px',
        letterSpacing: 1,
        textTransform: 'uppercase',
        margin: '6px 0',
      }}
    >
      {children}
    </Para>
  ),
  Rule: () => <Line w={2} gap={8} />,
  Gap: () => <Space h={8} />,
  Info: ({ l, r }) => <LR drop l={l} r={r} style={small} />,
  Item: ({ name, amount, children }) => (
    <div style={{ marginTop: 6 }}>
      <LR l={<b>{name}</b>} r={<b>{amount}</b>} />
      {children}
    </div>
  ),
  Sub: ({ l, r, kind }) => {
    if (kind === 'total' || kind === 'given') return null
    return <LR l={kind === 'saving' ? `${l.replace('You saved', 'off')} ${r}` : pretty(l)} r={kind === 'saving' ? undefined : r} style={small} />
  },
  Member: ({ children }) => <Para style={small}>{`· ${pretty(children)}`}</Para>,
  Note: ({ children, level }) => <Para style={{ ...small, paddingLeft: level === 1 ? 0 : 16 }}>{children}</Para>,
  Money: ({ l, r, bold }) => <LR l={l} r={r} style={{ ...small, fontWeight: bold ? 700 : 400 }} />,
  Total: ({ label, amount }) => <LR l={label} r={amount} style={{ fontWeight: 900, fontSize: 26, lineHeight: '34px' }} />,
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
      <Para center style={{ fontWeight: 900, marginTop: 6, letterSpacing: 2 }}>
        THANK YOU!
      </Para>
      <Credit style={{ marginTop: 10 }} />
    </>
  ),
  fonts: [...SANS_FONTS],
}
