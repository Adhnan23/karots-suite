// Money, as `money.Format` / `money.Display` print it.

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
