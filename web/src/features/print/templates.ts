import type { ComponentType } from 'react'
import type { SlipData } from '@/api/types.gen'
import { DebtSlip, MoneySlip, RefundSlip, TestSlip, WarrantySlip } from './templates/CounterSlips'
import { SaleSlip } from './templates/SaleSlip'

/** A slip kind's React template (Image mode); a kind not here prints its decoded Classic lines. */
export const templates: Partial<Record<string, ComponentType<{ d: SlipData }>>> = {
  sale: SaleSlip,
  refund: RefundSlip,
  money: MoneySlip,
  debt: DebtSlip,
  warranty: WarrantySlip,
  test: TestSlip,
}
