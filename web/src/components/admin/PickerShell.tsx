import type { ReactNode } from 'react'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet'
import { useIsPhone } from '@/lib/useMedia'
import { cn } from '@/lib/utils'

/** The pop-up behind a picker field: a dialog on desktop, a tall bottom sheet on a phone.
 * fit = size to the content instead of a fixed height (a detail view, not a search list). */
export function PickerShell({ open, onOpenChange, title, fit, children }: { open: boolean; onOpenChange: (o: boolean) => void; title: string; fit?: boolean; children: ReactNode }) {
  const phone = useIsPhone()
  const head = <span className="font-display text-lg font-bold">{title}</span>
  return phone ? (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="bottom" className={cn('gap-3 rounded-t-xl p-4', fit ? 'max-h-[85vh]' : 'data-[side=bottom]:h-[85vh]')}>
        <SheetTitle asChild>{head}</SheetTitle>
        {children}
      </SheetContent>
    </Sheet>
  ) : (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={cn('flex flex-col gap-3 p-5 sm:max-w-md', fit ? 'max-h-[85vh]' : 'h-[min(620px,85vh)]')}>
        <DialogTitle asChild>{head}</DialogTitle>
        {children}
      </DialogContent>
    </Dialog>
  )
}
