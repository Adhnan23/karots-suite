import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { slipElement } from '../templates'
import { sample } from '../styles/sample'
import { STYLES } from '../styles'

// Paper is black and white: a cheque is marked by its text (CHEQUE, number,
// bank, date), whatever receipt style the shop picked.
const text = (html: string) => {
  const el = document.createElement('div')
  el.innerHTML = html
  return el.textContent ?? ''
}
const cheque = 'CHEQUE 123456 Bank of Ceylon 12/11/26'

describe('cheques print in every receipt style', () => {
  for (const st of STYLES) {
    it(`${st.key}: sale`, () => {
      const d = sample('busy', '80')
      d.sale!.payments = [{ method: cheque, amount: '5000.00' }, { method: 'Cash', amount: '1500.00' }]
      const t = text(renderToStaticMarkup(slipElement(d, st.key)!))
      expect(t).toContain(cheque)
      expect(t).toContain('5,000.00')
    })
    it(`${st.key}: credit payment`, () => {
      const d = sample('debt', '80')
      d.debt = { ...d.debt!, method: 'Cheque', cheques: [{ method: cheque, amount: '2000.00' }] }
      const t = text(renderToStaticMarkup(slipElement(d, st.key)!))
      expect(t).toContain(cheque)
    })
  }
})
