import { useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '@/components/ui/dialog'

/** A destructive action behind a plain "are you sure" (never window.confirm). */
/** plain = an everyday action worth a second look (not red). */
export function ConfirmButton({ label, title, body, confirm, onConfirm, className, plain }: {
  label: ReactNode
  title: string
  body?: ReactNode
  confirm: string
  onConfirm: () => Promise<unknown> | void
  className?: string
  plain?: boolean
}) {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  return (
    <>
      <Button type="button" variant={plain ? 'ghost' : 'destructive'} className={className ?? 'h-10 px-4'}
        aria-label={typeof label === 'string' ? undefined : title} onClick={() => setOpen(true)}>{label}</Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className="gap-3 p-5 sm:max-w-sm">
          <DialogTitle className="font-display text-lg font-bold">{title}</DialogTitle>
          {body && <DialogDescription>{body}</DialogDescription>}
          <div className="mt-2 flex justify-end gap-2">
            <Button type="button" variant="ghost" className="h-10 px-4" onClick={() => setOpen(false)}>Keep it</Button>
            <Button type="button" disabled={busy} className={plain ? 'h-10 px-4 font-bold' : 'h-10 bg-danger px-4 font-bold text-white hover:bg-danger/90'}
              onClick={async () => {
                setBusy(true)
                try { await onConfirm(); setOpen(false) } catch (e) { toast.error(errMsg(e)) } finally { setBusy(false) }
              }}>
              {confirm}
            </Button>
          </div>
        </DialogContent>
      </Dialog>
    </>
  )
}
