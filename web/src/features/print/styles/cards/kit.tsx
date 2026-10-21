import { SANS_FAMILY, SANS_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Logo, LR, NumberCode, Para, Paper, pretty, Space } from '../parts'

// Cards: like an app screen — every item in its own rounded box, the slip
// number in a pill, the total a big black rounded pill.

const small = { fontSize: 17, lineHeight: '22px' }
const box = { border: '3px solid #000', borderRadius: 16, padding: '6px 12px', margin: '8px 0' }

export const cards: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={SANS_FAMILY}>
      {children}
    </Paper>
  ),
  Header: ({ shop, dots, number }) => (
    <>
      <Logo shop={shop} dots={dots} />
      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', justifyContent: 'space-between', columnGap: 12, marginTop: 6 }}>
        <div style={{ fontWeight: 700, fontSize: 32, lineHeight: '40px', minWidth: 0, overflowWrap: 'break-word' }}>{shop.name}</div>
        {number && (
          <div style={{ ...small, fontWeight: 700, border: '3px solid #000', borderRadius: 20, padding: '2px 14px', marginLeft: 'auto', whiteSpace: 'nowrap' }}>
            {number}
          </div>
        )}
      </div>
      {shop.nameLocal && <Para style={{ fontSize: Math.floor(dots / 16), lineHeight: 1.4 }}>{shop.nameLocal}</Para>}
      {[shop.address, shop.phone && `Tel: ${shop.phone}`, shop.vatNo && `VAT: ${shop.vatNo}`].filter(Boolean).map((s, i) => (
        <Para key={i} style={small}>
          {s}
        </Para>
      ))}
    </>
  ),
  Title: ({ children }) => (
    <div style={{ ...box, borderRadius: 30, textAlign: 'center', fontWeight: 700, letterSpacing: 2, textTransform: 'uppercase' }}>{children}</div>
  ),
  Rule: () => <Space h={6} />,
  Gap: () => <Space h={6} />,
  Info: ({ l, r }) => <LR drop l={l} r={r} style={small} />,
  Item: ({ name, amount, children }) => (
    <div style={box}>
      <LR l={<b>{name}</b>} r={<b>{amount}</b>} />
      <div style={small}>{children}</div>
    </div>
  ),
  Sub: ({ l, r, kind }) => (kind === 'total' || kind === 'given' ? null : <LR l={pretty(l)} r={r} />),
  Member: ({ children }) => <Para>{`· ${pretty(children)}`}</Para>,
  Note: ({ children, kind }) => <Para>{kind === 'serial' ? `SN ${children}` : children}</Para>,
  Money: ({ l, r, bold }) => <LR l={l} r={r} style={{ ...small, fontWeight: bold ? 700 : 400 }} />,
  Total: ({ label, amount }) => (
    <LR
      l={label}
      r={amount}
      style={{ background: '#000', color: '#fff', borderRadius: 32, padding: '8px 20px', margin: '8px 0', fontWeight: 700, fontSize: 28, lineHeight: '36px' }}
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
      {number && (
        <Para center style={small}>
          Scan at the counter for returns
        </Para>
      )}
      {shop.footer && (
        <Para center style={{ ...small, marginTop: 6 }}>
          {shop.footer}
        </Para>
      )}
      <Para center bold style={{ marginTop: 6 }}>
        Thank you!
      </Para>
      <Credit style={{ marginTop: 8 }} />
    </>
  ),
  fonts: [...SANS_FONTS],
}
