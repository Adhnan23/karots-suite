export type HotkeyAction = 'reset' | 'search' | 'qty' | 'lineDiscount' | 'held' | 'customer' | 'clear' | 'hold' | 'discount' | 'pay' | 'blur' | 'newSale'

/** F1 ⟲ menu (search only — never the cart) · F2/F3 search · F4 hold · F5 held sales · F6 the last line's
 * quantity · F7 its discount · F8 customer · F9 bill discount · F10 amount (again: the next payment method;
 * Enter completes) · F12 new sale · Esc blur · Enter after a sale = next. The whole Sell page without a mouse. */
export function hotkeyFor(key: string, ctx: { dialogOpen: boolean; saleDone: boolean }): HotkeyAction | null {
  switch (key) {
    case 'F1':
      return 'reset'
    case 'F2':
    case 'F3':
      return 'search'
    case 'F4':
      return ctx.dialogOpen ? null : 'hold'
    case 'F5':
      return ctx.dialogOpen ? null : 'held'
    case 'F6':
      return ctx.dialogOpen ? null : 'qty'
    case 'F7':
      return ctx.dialogOpen ? null : 'lineDiscount'
    case 'F8':
      return ctx.dialogOpen ? null : 'customer'
    case 'F12':
      return ctx.dialogOpen ? null : 'clear'
    case 'F9':
      return 'discount'
    case 'F10':
      return ctx.dialogOpen ? null : 'pay'
    case 'Escape':
      return ctx.dialogOpen ? null : 'blur' // Radix closes its own dialog
    case 'Enter':
      return ctx.saleDone ? 'newSale' : null
  }
  return null
}

/** In a discount box (a line's or the bill's): / switches between the amount and a percentage. */
export const flipsDiscount = (key: string) => key === '/'
export const flipped = (t: 'fixed' | 'percent') => (t === 'percent' ? 'fixed' : 'percent')
