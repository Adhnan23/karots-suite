import { Dialog, DialogContent, DialogDescription, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import { Kbd } from '../bits'
import { DrawerActions } from '../drawer/DrawerActions'
import { slots } from '@/plugins/slots'
import { CHIP, PANEL } from '../ui'
import { Browser, ProductList, SearchRow, useCatalog } from './CatalogPanel'

/** Search only (Sell page layout): the search bar across the top, what's typed drops a
 * list below it (↑ ↓ Enter), a scan adds straight away. The plugin menus sit on the button
 * row and open in Browse (F1), with the groups and frequently sold. */
export function SearchStrip() {
  const cat = useCatalog(true)
  const { ui } = cat
  return (
    <>
      <div className="relative">
        <SearchRow cat={cat} strip />
        {cat.searching && (
          <div className={cn(PANEL, 'absolute inset-x-0 top-full z-30 mt-2 flex max-h-[60vh] flex-col overflow-hidden')}>
            <div className="flex items-center justify-between px-4 py-2 text-till-sm font-semibold text-faint">
              {cat.title}
              <span className="flex items-center gap-1.5">{cat.products.length > 1 && <><Kbd>↑ ↓</Kbd> Enter ·</>}{cat.products.length} items</span>
            </div>
            <div className="min-h-0 overflow-auto">
              {cat.products.length ? <ProductList cat={cat} rows /> : <p className="py-6 text-center text-faint">No product matches. Press Enter to add it as a quick item.</p>}
            </div>
          </div>
        )}
      </div>
      <DrawerActions lead={slots.tillMenu.map((m) => (
        <button key={m.key} type="button" className={CHIP} onClick={() => { ui.reset(); ui.setMenu(m.key) }}>
          {m.emoji} {m.label}
        </button>
      ))} />
      <BrowsePopup />
    </>
  )
}

/** Browse: the groups, frequently sold and plugin menus the products column would show.
 * Open while Browse / F1 asked for it or a group or menu is picked (Ctrl K too). */
function BrowsePopup() {
  const cat = useCatalog()
  const { ui } = cat
  const open = ui.browse || !!ui.menu || ui.stack.length > 0
  return (
    <Dialog open={open} onOpenChange={(o) => !o && ui.reset()}>
      <DialogContent className="flex h-[85vh] flex-col gap-3 sm:max-w-[min(1200px,92vw)]"
        onCloseAutoFocus={(e) => { e.preventDefault(); document.getElementById('till-search')?.focus() }}>
        <DialogHeader>
          <DialogTitle>Browse</DialogTitle>
          <DialogDescription>Tap a product to add it; the pop-up stays open. Esc or F1 closes it.</DialogDescription>
        </DialogHeader>
        <Browser cat={cat} popup />
      </DialogContent>
    </Dialog>
  )
}
