import { createContext, useContext } from 'react'
import { SANS_FAMILY, SANS_FONTS } from '../../paper'
import type { SlipKit } from '../kit'
import { Credit, Line, Logo, LR, NumberCode, Para, Paper, pretty, Space } from '../parts'

// Stamp: playful — a big outlined shop name, a tilted round stamp beside it
// (PAID on a sale — sale.tsx says which word), items on two lines with the
// price set hard right, titles as tilted box stamps.

const small = { fontSize: 17, lineHeight: '22px' }

/** The round stamp's word and date; null = no stamp (slips other than a sale). */
export const StampWord = createContext<{ word: string; date: string } | null>(null)

function Round({ word, date }: { word: string; date: string }) {
  return (
    <div
      style={{
        flexShrink: 0,
        width: 112,
        height: 112,
        border: '5px solid #000',
        borderRadius: '50%',
        transform: 'rotate(-16deg)',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        textAlign: 'center',
      }}
    >
      <div style={{ fontWeight: 900, fontSize: word.length > 5 ? 18 : 28, lineHeight: '30px', letterSpacing: 2 }}>{word}</div>
      <div style={{ fontSize: 13, lineHeight: '16px' }}>{date.slice(0, 10)}</div>
    </div>
  )
}

export const stamp: SlipKit = {
  Page: ({ d, children }) => (
    <Paper dots={d.dots} family={SANS_FAMILY}>
      {children}
    </Paper>
  ),
  Header: function Header({ shop, dots }) {
    const st = useContext(StampWord)
    return (
      <>
        <Logo shop={shop} dots={dots} />
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 6 }}>
          <div style={{ flex: 1, minWidth: 0 }}>
            <Para
              center
              style={{
                fontWeight: 900,
                fontSize: 46,
                lineHeight: '52px',
                color: '#fff',
                WebkitTextStroke: '4px #000', paintOrder: 'stroke fill', // stroke under the fill: no crossing lines where a glyph's shapes overlap
                letterSpacing: 2,
                textTransform: 'uppercase',
              }}
            >
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
          </div>
          {st && <Round word={st.word} date={st.date} />}
        </div>
      </>
    )
  },
  Title: ({ children }) => (
    <div style={{ textAlign: 'center', margin: '14px 0 8px' }}>
      <span
        style={{
          display: 'inline-block',
          border: '4px solid #000',
          padding: '2px 12px',
          fontWeight: 900,
          letterSpacing: 2,
          textTransform: 'uppercase',
          transform: 'rotate(-3deg)',
        }}
      >
        {children}
      </span>
    </div>
  ),
  Rule: () => <Line w={2} kind="dashed" gap={8} />,
  Gap: () => <Space h={8} />,
  Info: ({ l, r }) => <LR drop l={l} r={r} style={small} />,
  Item: ({ name, children }) => (
    <div style={{ marginTop: 6 }}>
      <Para bold>{name}</Para>
      {children}
    </div>
  ),
  Sub: ({ l, r, kind }) =>
    kind === 'total' ? <LR l="" r={<b>{r}</b>} style={{ fontSize: 22 }} /> : <LR l={pretty(l)} r={kind === 'given' ? <b>{r}</b> : r} style={small} />,
  Member: ({ children }) => <Para style={small}>{`+ ${pretty(children)}`}</Para>,
  Note: ({ children, level, kind }) => <Para style={{ ...small, paddingLeft: level === 1 ? 0 : 24 }}>{kind === 'serial' ? `SN ${children}` : children}</Para>,
  Money: ({ l, r, bold }) => <LR l={l} r={r} style={{ ...small, fontWeight: bold ? 700 : 400 }} />,
  Total: ({ label, amount }) => <LR l={label} r={amount} style={{ fontWeight: 900, fontSize: 34, lineHeight: '42px', margin: '4px 0' }} />,
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
      <Para center bold style={{ marginTop: 6 }}>
        Thanks for shopping with us!
      </Para>
      <Credit style={{ marginTop: 8 }} />
    </>
  ),
  fonts: [...SANS_FONTS],
}
