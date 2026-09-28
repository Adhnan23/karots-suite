import { MotionConfig } from 'motion/react'
import { cn } from '@/lib/utils'
import { TillUiProvider } from './TillUi'
import { PANEL } from './ui'

/** The sell screen (theme.md §7): catalogue | cart | pay at ≥1200px; below that the
 * cart and pay stack in a 360px right column. Sections are filled by Tasks 9–13. */
export function TillScreen() {
  return (
    <MotionConfig reducedMotion="user">
      <TillUiProvider>
        <div className="relative grid h-full min-h-0 grid-cols-[1fr_360px] grid-rows-[minmax(0,1fr)_auto] gap-[14px] p-[14px] [grid-template-areas:'cat_cart'_'cat_pay'] min-[1200px]:grid-cols-[1fr_392px_356px] min-[1200px]:grid-rows-[minmax(0,1fr)] min-[1200px]:[grid-template-areas:'cat_cart_pay']">
          <section aria-label="Catalogue" className="min-h-0 [grid-area:cat]" />
          <section aria-label="Cart" className={cn(PANEL, 'min-h-0 [grid-area:cart]')} />
          <section aria-label="Payment" className={cn(PANEL, 'min-h-0 [grid-area:pay]')} />
        </div>
      </TillUiProvider>
    </MotionConfig>
  )
}
