import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { slots } from '@/plugins/slots'
import { ProductInfo } from './catalog/ProductInfo'
import { QuickItem } from './catalog/QuickItem'
import { HeldSales } from './cart-ui/HeldSales'
import { PricePick } from './cart-ui/PricePick'
import { Suggestion } from './cart-ui/Suggestion'
import { useCheckoutCtx } from './checkout/CheckoutProvider'
import { CloseRegister } from './drawer/CloseRegister'
import { MoneyMove } from './drawer/MoneyMove'
import { AddCustomer } from './pay/AddCustomer'
import { useTillApi, useTillUi } from './TillUi'

function ClearSale() {
  const ui = useTillUi()
  const co = useCheckoutCtx()
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle>Clear this sale?</DialogTitle>
          <DialogDescription>Every line, the customer and any payments are removed. Hold it instead to finish it later.</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={ui.close}>Keep it</Button>
          <Button variant="destructive" autoFocus onClick={() => { ui.close(); co.newSale() }}>Clear sale</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

/** Renders whichever till dialog is open. Tasks 10–13 add their cases here. */
export function TillDialogs() {
  const ui = useTillUi()
  const till = useTillApi()
  const d = ui.dialog
  if (!d) return null
  switch (d.kind) {
    case 'withdraw':
    case 'deposit':
      return <MoneyMove kind={d.kind} />
    case 'close':
      return <CloseRegister logout={d.logout} />
    case 'info':
      return <ProductInfo product={d.product} />
    case 'quickItem':
      return <QuickItem barcode={d.barcode} name={d.name} />
    case 'pricePick':
      return <PricePick product={d.product} lots={d.lots} />
    case 'suggestion':
      return <Suggestion product={d.product} />
    case 'held':
      return <HeldSales />
    case 'addCustomer':
      return <AddCustomer />
    case 'clearSale':
      return <ClearSale />
    case 'action': {
      const a = slots.tillActions.find((x) => x.key === d.key)
      return a ? <a.Panel till={till} onClose={ui.close} /> : null
    }
    default:
      return null
  }
}
