import type { SlipData } from '@/api/types.gen'
import { SaleHead, SaleItems, SaleMoney } from '../../templates/SaleSlip'
import type { SlipKit } from '../kit'
import { display, money, neg, pos } from '../money'
import { Line, Logo, Para, ShopLines } from '../parts'

/** Poster's sale: the amount is the headline, the shop name runs sideways down
 * the left edge (on 58 mm it sits on top — the roll is too narrow for both). */
export function PosterSale({ d, kit: K }: { d: SlipData; kit: SlipKit }) {
  const s = d.sale!
  const wide = d.dots >= 576
  const [label, amount] = neg(s.total) ? ['YOU GOT', s.change] : s.credit ? ['TOTAL', s.total] : ['YOU PAID', s.total]
  const body = (
    <div style={{ flex: 1, minWidth: 0 }}>
      <div
        style={{
          fontSize: 17,
          lineHeight: '22px',
          fontWeight: 700,
          letterSpacing: 4,
        }}
      >
        {`${label} · ${d.currency.toUpperCase()}`}
      </div>
      <div
        style={{
          fontWeight: 900,
          fontSize: wide ? 76 : 60,
          lineHeight: wide ? '80px' : '64px',
          letterSpacing: -2,
          whiteSpace: 'nowrap',
        }}
      >
        {display(amount)}
      </div>
      {pos(s.youSaved) && <div style={{ fontWeight: 700 }}>{`You saved ${money(d.currency, s.youSaved)}`}</div>}
      <Line w={6} gap={8} />
      <SaleItems s={s} K={K} />
      <K.Rule />
      <SaleMoney s={s} sym={d.currency} K={K} />
      <K.Rule />
      <SaleHead s={s} K={K} />
    </div>
  )
  return (
    <K.Page d={d}>
      {wide ? (
        <>
          <Logo shop={d.shop} dots={d.dots} />
          <div style={{ display: 'flex', gap: 14, marginTop: 6 }}>
            <div
              style={{
                writingMode: 'vertical-rl',
                transform: 'rotate(180deg)',
                fontWeight: 900,
                fontSize: 44,
                letterSpacing: 6,
                textTransform: 'uppercase',
                borderLeft: '6px solid #000',
                paddingLeft: 8,
                whiteSpace: 'nowrap',
              }}
            >
              {d.shop.name}
            </div>
            {body}
          </div>
        </>
      ) : (
        <>
          <K.Header shop={d.shop} dots={d.dots} />
          {body}
        </>
      )}
      {wide && (
        <div style={{ marginTop: 10 }}>
          {d.shop.nameLocal && (
            <Para center style={{ fontSize: Math.floor(d.dots / 16), lineHeight: 1.4 }}>
              {d.shop.nameLocal}
            </Para>
          )}
          <ShopLines shop={d.shop} style={{ fontSize: 17, lineHeight: '22px' }} />
        </div>
      )}
      <K.Footer shop={d.shop} number={s.receiptNo} />
    </K.Page>
  )
}
