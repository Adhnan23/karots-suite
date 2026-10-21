import { createContext, useContext, type CSSProperties, type ReactNode } from 'react'
import type { SlipShop } from '@/api/types.gen'
import { PAPER_FAMILY } from './paper'

// The template kit: a slip laid out like Classic's `escpos` lines (classic style),
// at printer dots (1 CSS px = 1 dot). Font A is a 12 × 24 dot cell (JetBrains Mono
// 20 px), Font B 9 × 17. Black on white: it's paper, not themed UI.
const A = { size: 20, line: 24, col: 12 }
const B = { size: 15, line: 17 }

const Cols = createContext(48)

export function ReceiptPage({ dots, columns, children }: { dots: number; columns: number; children: ReactNode }) {
  return (
    <Cols.Provider value={columns}>
      <div style={{ width: dots, background: '#fff', color: '#000', fontFamily: PAPER_FAMILY, fontVariantLigatures: 'none', fontSize: A.size, lineHeight: `${A.line}px` }}>
        {children}
      </div>
    </Cols.Provider>
  )
}

// ---- money, as `money.Format` / `money.Display` print it ----

const thousands = (s: string) => {
  const neg = s.startsWith('-')
  const [int, frac] = (neg ? s.slice(1) : s).split('.')
  return (neg ? '-' : '') + int.replace(/\B(?=(\d{3})+(?!\d))/g, ',') + (frac !== undefined ? '.' + frac : '')
}
/** "1250.5" → "1,250.50" */
export const display = (s: string) => thousands(Number(s).toFixed(2))
/** "Rs." + "1250.00" → "Rs. 1,250.00" */
export const money = (sym: string, s: string) => `${sym} ${display(s)}`
export const pos = (s: string) => Number(s) > 0
export const neg = (s: string) => Number(s) < 0
/** "-12.00" → "12.00" */
export const abs = (s: string) => s.replace(/^-/, '')

/** Text that wraps at the line's columns like `escpos.wrap` (words, a too-long
 * word hard-split). Leading spaces indent the first line only, as Classic does. */
export function Text({ children, bold, center, indent = 0 }: { children: ReactNode; bold?: boolean; center?: boolean; indent?: number }) {
  return (
    <div style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontWeight: bold ? 700 : 400, textAlign: center ? 'center' : 'left', paddingLeft: indent * A.col }}>
      {children || ' '}
    </div>
  )
}

/** A hanging line: `lead` (e.g. "   - ") then text that wraps under itself. */
export function Hang({ lead, children }: { lead: string; children: ReactNode }) {
  return (
    <div style={{ display: 'flex', whiteSpace: 'pre' }}>
      <span style={{ flexShrink: 0 }}>{lead}</span>
      <span style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', minWidth: 0 }}>{children}</span>
    </div>
  )
}

/** `escpos.leftRight`: left text and right text on one line, at least a column
 * apart; the left side is cut when both don't fit. `big` is the double-height
 * bold line Classic prints for TOTAL, CHANGE and DUE. */
export function Row({ l, r, bold, big }: { l: ReactNode; r: ReactNode; bold?: boolean; big?: boolean }) {
  const row: CSSProperties = { display: 'flex', height: A.line, whiteSpace: 'pre' }
  const inner = (
    <div style={big ? { ...row, transform: 'scaleY(2)', transformOrigin: 'top' } : row}>
      <span style={{ flex: '1 1 auto', minWidth: 0, overflowX: 'clip', overflowY: 'visible' }}>{l}</span>
      <span style={{ flexShrink: 0, paddingLeft: A.col }}>{r}</span>
    </div>
  )
  return <div style={{ height: big ? A.line * 2 : A.line, fontWeight: bold || big ? 700 : 400 }}>{inner}</div>
}

/** The classic body rule: a full line of "-". */
export function Rule() {
  return <div style={{ whiteSpace: 'pre', height: A.line }}>{'-'.repeat(useContext(Cols))}</div>
}

export const Blank = () => <div style={{ height: A.line }} />

/** Font B lines (the credit), centred over the same width. */
export function Small({ children }: { children: ReactNode }) {
  return <div style={{ fontSize: B.size, lineHeight: `${B.line}px`, height: B.line, textAlign: 'center', whiteSpace: 'pre' }}>{children}</div>
}

/** `escpos.Title`: a blank line, then "*** TITLE ***" in bold. */
export function Title({ children }: { children: string }) {
  return (
    <>
      <Blank />
      <Text bold center>{`*** ${children.toUpperCase()} ***`}</Text>
    </>
  )
}

/** `escpos.wrap` for the shop name, whose lines are drawn one by one. */
function wrap(s: string, w: number): string[] {
  const out: string[] = []
  let cur = ''
  for (let word of s.split(/\s+/).filter(Boolean)) {
    while (word.length > w) {
      if (cur) out.push(cur), (cur = '')
      out.push(word.slice(0, w))
      word = word.slice(w)
    }
    if (!cur) cur = word
    else if (cur.length + 1 + word.length <= w) cur += ' ' + word
    else out.push(cur), (cur = word)
  }
  if (cur) out.push(cur)
  return out.length ? out : ['']
}

/** `escpos.Header` (classic style): logo, the name double size when it fits in
 * half the columns (else double height, wrapped), the local-language name,
 * address, phone, VAT and the rule. */
export function Header({ shop, dots }: { shop: SlipShop; dots: number }) {
  const w = useContext(Cols)
  const name = shop.name
  return (
    <>
      {shop.logo && (
        <>
          <img src={shop.logo} alt="" style={{ display: 'block', width: dots, imageRendering: 'pixelated' }} />
          <Blank />
        </>
      )}
      {name.length <= w / 2 ? (
        <div style={{ fontWeight: 700, fontSize: A.size * 2, lineHeight: `${A.line * 2}px`, height: A.line * 2, textAlign: 'center', whiteSpace: 'pre' }}>{name}</div>
      ) : (
        wrap(name, w).map((ln, i) => (
          <div key={i} style={{ height: A.line * 2, fontWeight: 700, textAlign: 'center', whiteSpace: 'pre' }}>
            <div style={{ transform: 'scaleY(2)', transformOrigin: 'top' }}>{ln}</div>
          </div>
        ))
      )}
      {/* Classic draws this name as a raster dots/14 px tall (receiptimg.SubName). */}
      {shop.nameLocal && <div style={{ fontSize: Math.floor(dots / 14), lineHeight: 1.4, textAlign: 'center' }}>{shop.nameLocal}</div>}
      <Blank />
      {shop.address && <Text center>{shop.address}</Text>}
      {shop.phone && <Text center>{`Tel: ${shop.phone}`}</Text>}
      {shop.vatNo && <Text center>{`VAT: ${shop.vatNo}`}</Text>}
      <Rule />
    </>
  )
}

/** `escpos.Footer` (classic style): rule, the shop's footer, thank-you, the
 * credit in Font B. The printer job adds the feed and cut. */
export function Footer({ shop }: { shop: SlipShop }) {
  return (
    <>
      <Rule />
      {shop.footer && <Text center>{shop.footer}</Text>}
      <Text center>Thank you! Come again.</Text>
      <Blank />
      <Small>Karots Business Suite</Small>
      <Small>adhnanmsa@gmail.com | 0769626396</Small>
    </>
  )
}
