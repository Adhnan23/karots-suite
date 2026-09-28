export type HotkeyAction = 'reset' | 'search' | 'hold' | 'discount' | 'pay' | 'blur' | 'newSale'

/** F1 ⟲ menu (search only — never the cart) · F2/F3 search · F4 hold · F9 bill
 * discount · F10 amount (then Enter completes) · Esc blur · Enter after a sale = next. */
export function hotkeyFor(key: string, ctx: { dialogOpen: boolean; saleDone: boolean }): HotkeyAction | null {
  switch (key) {
    case 'F1':
      return 'reset'
    case 'F2':
    case 'F3':
      return 'search'
    case 'F4':
      return ctx.dialogOpen ? null : 'hold'
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
