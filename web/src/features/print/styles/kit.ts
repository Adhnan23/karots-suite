import type { FC, ReactNode } from 'react'
import type { SlipData, SlipShop } from '@/api/types.gen'

/** The parts a slip is drawn from. Every style implements them its own way; the
 * shared templates (`templates/`) use only these, so one template follows every
 * style. Black on white at printer dots (1 CSS px = 1 dot). */
export type SlipKit = {
  Page: FC<{ d: { dots: number; columns: number }; children: ReactNode }>
  /** Logo, names, address, phone, VAT. `number` = the slip's number (styles that print it at the top). */
  Header: FC<{ shop: SlipShop; dots: number; number?: string }>
  /** "REFUND", "CREDIT PAYMENT"… */
  Title: FC<{ children: string }>
  Rule: FC
  /** A blank line between blocks. */
  Gap: FC
  /** A label / value line in the slip's head (Receipt:, Date:, Cashier:…). */
  Info: FC<{ l: string; r: string }>
  /** A line and its sub-lines. `amount` = the line's total, for styles that put it on the name row. */
  Item: FC<{ name: string; amount?: string; children?: ReactNode }>
  /** A sub-line under an item: qty × price, a saving, the line total (also on
   * the Item's `amount`, so a style may draw it there instead), a payout's "Given". */
  Sub: FC<{ l: string; r: string; kind: 'qty' | 'saving' | 'total' | 'given' }>
  /** A line group's member ("Bread x2"). */
  Member: FC<{ children: string }>
  /** A warranty or serial line under an item (level 1) or a group member (level 2). */
  Note: FC<{ children: string; level: 1 | 2; kind: 'warranty' | 'serial' }>
  /** A money line (Subtotal, a tender, Amount…); `saving` marks a discount. */
  Money: FC<{ l: ReactNode; r: ReactNode; bold?: boolean; saving?: boolean }>
  /** TOTAL / CHANGE / GIVEN / DUE / TOTAL DUE / ADVANCE LEFT. */
  Total: FC<{ label: string; amount: string }>
  Text: FC<{ children: ReactNode; center?: boolean; bold?: boolean }>
  /** The shop's footer + the Karots credit; `number` = the slip's number (styles that print its barcode). */
  Footer: FC<{ shop: SlipShop; number?: string }>
  /** Font families the picture must wait for before it is taken. */
  fonts: string[]
}

export const STYLE_KEYS = ['classic', 'bold', 'minimal', 'ticket', 'poster', 'grid', 'stamp', 'elegant', 'cards', 'sections'] as const
export type StyleKey = (typeof STYLE_KEYS)[number]

export type SlipTemplateFC = FC<{ d: SlipData; kit: SlipKit }>

export type StyleDef = {
  key: StyleKey
  label: string
  kit: SlipKit
  /** Whole slips this style draws itself (by kind), where the shared shape can't express it. */
  overrides?: Partial<Record<string, SlipTemplateFC>>
}
