import { useEffect, useLayoutEffect, useRef } from 'react'
import { useHold } from '../cart-ui/useHold'
import { hotkeyFor } from '../hotkeys'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { isStrip } from '../layout/arrangements'
import { useSellLayout } from '../layout/useSellLayout'
import { useCheckoutCtx } from './CheckoutProvider'

/** F10 in the amount box: the next payment method the sale can take (it refocuses the box). */
function nextMethod() {
  const all = [...document.querySelectorAll<HTMLButtonElement>('[aria-label="Payment method"] button:not(:disabled)')]
  if (!all.length) return
  const at = all.findIndex((b) => b.getAttribute('aria-pressed') === 'true')
  all[(at + 1) % all.length].click()
}

const focus = (id: string) => {
  const el = document.getElementById(id) as HTMLInputElement | null
  el?.focus()
  el?.select?.()
}

/** Till keyboard + the number-input scroll-wheel guard. */
export function Hotkeys() {
  const ui = useTillUi()
  const co = useCheckoutCtx()
  const hold = useHold()
  const lines = useTill().state.lines.length
  const strip = isStrip(useSellLayout().layout)
  const live = useRef({ ui, co, hold, lines, strip })
  // Before paint, not after: a key pressed the moment a scan lands must see that line.
  useLayoutEffect(() => {
    live.current = { ui, co, hold, lines, strip }
  })

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const { ui, co, hold, lines, strip } = live.current
      const dialogOpen = !!document.querySelector('[role="dialog"],[role="alertdialog"]')
      const a = hotkeyFor(e.key, { dialogOpen, saleDone: !!co.done })
      if (!a) return
      e.preventDefault() // also stops F1 opening browser help, and Enter double-firing the focused button
      if (a === 'reset') {
        // Search only: F1 opens Browse (the groups' pop-up), and again closes it.
        const browsing = ui.browse || !!ui.menu || ui.stack.length > 0
        ui.reset()
        if (strip && !browsing) ui.setBrowse(true)
      }
      else if (a === 'search') focus('till-search')
      else if (a === 'qty') focus('till-qty-last')
      else if (a === 'lineDiscount') document.getElementById('till-disc-last')?.click()
      else if (a === 'customer') document.getElementById('till-customer')?.click()
      else if (a === 'held') ui.open({ kind: 'held' })
      else if (a === 'clear') {
        if (co.done || !lines) co.newSale()
        else ui.open({ kind: 'clearSale' })
      } else if (a === 'pay' && document.activeElement?.id === 'till-amount') nextMethod()
      else if (a === 'discount') focus('till-bill-discount')
      else if (a === 'pay') focus('till-amount')
      else if (a === 'hold') void hold()
      else if (a === 'newSale') co.newSale()
      else if (a === 'blur') (document.activeElement as HTMLElement | null)?.blur()
    }
    // A focused number input changes on scroll — a silent way to mis-enter money.
    const onWheel = (e: WheelEvent) => {
      const el = e.target as HTMLInputElement | null
      if (el?.tagName === 'INPUT' && el.type === 'number' && el === document.activeElement) el.blur()
    }
    window.addEventListener('keydown', onKey)
    document.addEventListener('wheel', onWheel, { passive: true })
    return () => {
      window.removeEventListener('keydown', onKey)
      document.removeEventListener('wheel', onWheel)
    }
  }, [])
  return null
}
