import type { ReactNode } from 'react'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet'
import { useIsPhone } from '@/lib/useMedia'

/** The pop-up behind a picker field: a dialog on desktop, a tall bottom sheet on a phone. */
export function PickerShell({ open, onOpenChange, title, children }: { open: boolean; onOpenChange: (o: boolean) => void; title: string; children: ReactNode }) {
  const phone = useIsPhone()
  const head = <span className="font-display text-lg font-bold">{title}</span>
  return phone ? (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="bottom" className="h-[85vh] gap-3 rounded-t-[22px] p-4">
        <SheetTitle asChild>{head}</SheetTitle>
        {children}
      </SheetContent>
    </Sheet>
  ) : (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="flex h-[min(620px,85vh)] flex-col gap-3 p-5 sm:max-w-md">
        <DialogTitle asChild>{head}</DialogTitle>
        {children}
      </DialogContent>
    </Dialog>
  )
}
