import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'

/** "Ask before printing" (Settings): the question is a dialog, never a toast. */
export function AskPrint({ onPrint, onClose }: { onPrint: () => void; onClose: () => void }) {
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Print the receipt?</DialogTitle>
          <DialogDescription>It prints at the counter printer.</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Skip</Button>
          <Button onClick={() => { onPrint(); onClose() }}>Print</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
