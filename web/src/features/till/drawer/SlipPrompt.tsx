import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { printMoneySlip } from '../print'
import { useTillUi } from '../TillUi'

/** "Ask before printing" mode: Print or Skip the CR- slip after a cash move. */
export function SlipPrompt({ receiptId }: { receiptId: number }) {
  const ui = useTillUi()
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>Print the receipt?</DialogTitle>
          <DialogDescription>A cash receipt was recorded for this drawer move.</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={ui.close}>Skip</Button>
          <Button autoFocus onClick={() => { void printMoneySlip(receiptId); ui.close() }}>Print</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
