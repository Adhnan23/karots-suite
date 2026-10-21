import type { SlipData } from '@/api/types.gen'
import { SaleSlip } from '../../templates/SaleSlip'
import type { SlipKit } from '../kit'
import { neg } from '../money'
import { StampWord } from './kit'

/** Stamp's sale: the shared sale, with the round stamp saying how it was settled. */
export function StampSale({ d, kit }: { d: SlipData; kit: SlipKit }) {
  const s = d.sale!
  const word = s.credit ? 'CREDIT' : neg(s.total) ? 'PAID OUT' : 'PAID'
  return (
    <StampWord.Provider value={{ word, date: s.date }}>
      <SaleSlip d={d} kit={kit} />
    </StampWord.Provider>
  )
}
