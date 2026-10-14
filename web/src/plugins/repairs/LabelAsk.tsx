import { Printer } from 'lucide-react'
import type { LabelRequest } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { LabelPreview } from '@/features/counter/labels/LabelPreview'

/** The ticket label as it will print (core's label preview), then Print. Labels
 * aren't slips: Settings' "ask before printing" doesn't govern them. */
export function LabelAsk({ req, onPrint, onClose }: { req: LabelRequest; onPrint: () => void; onClose: () => void }) {
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>Print the ticket label?</DialogTitle>
          <DialogDescription>Stick it on the device — scanning it opens the job.</DialogDescription>
        </DialogHeader>
        <div className="rounded-[var(--radius-sm)] bg-muted p-4"><LabelPreview req={req} /></div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button autoFocus onClick={() => { onPrint(); onClose() }}><Printer size={16} /> Print</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
