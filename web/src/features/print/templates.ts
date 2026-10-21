import { createElement, type ReactElement } from 'react'
import type { SlipData } from '@/api/types.gen'
import { styleOf } from './styles'
import type { SlipTemplateFC } from './styles/kit'
import { slots } from '@/plugins/slots'
import { DebtSlip, MoneySlip, RefundSlip, TestSlip, WarrantySlip } from './templates/CounterSlips'
import { SaleSlip } from './templates/SaleSlip'

/** A slip kind's shared template (Image mode), drawn with a style's kit; a kind not
 * here prints its decoded Classic lines. */
export const templates: Partial<Record<string, SlipTemplateFC>> = {
  sale: SaleSlip,
  refund: RefundSlip,
  money: MoneySlip,
  debt: DebtSlip,
  warranty: WarrantySlip,
  test: TestSlip,
}

/** The slip drawn in a style (the style's own slip wins over the shared one; a
 * plugin kind's per-style override over its template); null = no template. */
export function slipElement(d: SlipData, style: string): ReactElement | null {
  const s = styleOf(style)
  const T = s.overrides?.[d.kind] ?? templates[d.kind]
  if (T) return createElement(T, { d, kit: s.kit })
  const p = slots.slipTemplates.find((t) => t.kind === d.kind)
  const P = p?.overrides?.[s.key] ?? p?.Template
  return P ? createElement(P, { d, kit: s.kit, style: s.key }) : null
}
