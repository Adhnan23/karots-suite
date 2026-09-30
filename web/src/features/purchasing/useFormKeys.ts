import { useEffect, useRef } from 'react'

/** F4 add a line · F10 save — karots-pos's purchase and return screens. Off while
 * a dialog is open (it owns the keyboard). */
export function useFormKeys(onF4: (() => void) | null, onF10: () => void) {
  const ref = useRef({ onF4, onF10 })
  useEffect(() => {
    ref.current = { onF4, onF10 }
  })
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if (document.querySelector('[role="dialog"]')) return
      if (e.key === 'F4' && ref.current.onF4) {
        e.preventDefault()
        ref.current.onF4()
      } else if (e.key === 'F10') {
        e.preventDefault()
        ref.current.onF10()
      }
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
  }, [])
}
