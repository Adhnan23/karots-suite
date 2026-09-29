import type { FormEvent, ReactNode } from 'react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog'
import { Sheet, SheetContent, SheetTitle } from '@/components/ui/sheet'
import { useIsPhone } from '@/lib/useMedia'
import { cn } from '@/lib/utils'

/** An admin form in a dialog (desktop) or a bottom sheet (phone, spec §6.3).
 * `extra` sits left of the submit button (e.g. Delete). */
export function FormSheet({ open, onOpenChange, title, submit, busy, onSubmit, extra, wide, children }: {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  submit: string
  busy?: boolean
  onSubmit: () => void
  extra?: ReactNode
  wide?: boolean
  children: ReactNode
}) {
  const phone = useIsPhone()
  const body = (
    <form autoComplete="off" className="flex min-h-0 flex-1 flex-col gap-4" onSubmit={(e: FormEvent) => { e.preventDefault(); onSubmit() }}>
      <div className="-mx-1 flex min-h-0 flex-col gap-3.5 overflow-auto px-1 pb-1">{children}</div>
      <div className="flex flex-wrap items-center gap-2">
        {extra}
        <Button type="button" variant="ghost" className="ml-auto h-10 px-4" onClick={() => onOpenChange(false)}>Cancel</Button>
        <Button type="submit" disabled={busy} className="h-10 px-5 font-bold">{submit}</Button>
      </div>
    </form>
  )
  const head = <span className="font-display text-lg font-bold">{title}</span>
  return phone ? (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="bottom" className="max-h-[92vh] gap-3 rounded-t-[22px] p-4">
        <SheetTitle asChild>{head}</SheetTitle>
        {body}
      </SheetContent>
    </Sheet>
  ) : (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className={cn('flex max-h-[88vh] flex-col gap-4 p-5 sm:max-w-md', wide && 'sm:max-w-2xl')}>
        <DialogTitle asChild>{head}</DialogTitle>
        {body}
      </DialogContent>
    </Dialog>
  )
}
