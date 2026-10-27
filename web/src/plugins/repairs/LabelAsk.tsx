import { Printer } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { LabelPreview } from '@/features/counter/labels/LabelPreview'
import { LabelView } from '@/features/print/labels/LabelView'
import { ticketRequest, type TicketLabel } from './labels'

/** The ticket label as it will print (the picture, or core's Classic preview), then Print. Labels
 * aren't slips: Settings' "ask before printing" doesn't govern them. */
export function LabelAsk({ t, onPrint, onClose }: { t: TicketLabel; onPrint: () => void; onClose: () => void }) {
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-[460px]">
        <DialogHeader>
          <DialogTitle>Print the ticket label?</DialogTitle>
          <DialogDescription>Stick it on the device — scanning it opens the job.</DialogDescription>
        </DialogHeader>
        <div className="rounded-[var(--radius-sm)] bg-muted p-4"><LabelView kind="repairs.ticket" data={t} size="" classic={<LabelPreview req={ticketRequest(t)} mode="classic" />} /></div>
        <DialogFooter>
          <Button variant="ghost" onClick={onClose}>Cancel</Button>
          <Button autoFocus onClick={() => { onPrint(); onClose() }}><Printer size={16} /> Print</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
