import { useEffect, useLayoutEffect, useRef } from 'react'
import { useHold } from '../cart-ui/useHold'
import { hotkeyFor } from '../hotkeys'
import { useTillUi } from '../TillUi'
import { useCheckoutCtx } from './CheckoutProvider'

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
  const live = useRef({ ui, co, hold })
  // Before paint, not after: a key pressed the moment a scan lands must see that line.
  useLayoutEffect(() => {
    live.current = { ui, co, hold }
  })

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const { ui, co, hold } = live.current
      const dialogOpen = !!document.querySelector('[role="dialog"],[role="alertdialog"]')
      const a = hotkeyFor(e.key, { dialogOpen, saleDone: !!co.done })
      if (!a) return
      e.preventDefault() // also stops F1 opening browser help, and Enter double-firing the focused button
      if (a === 'reset') ui.reset()
      else if (a === 'search') focus('till-search')
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
