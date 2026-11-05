import { Component, Suspense, type ReactNode } from 'react'
import { Sheet, SheetContent, SheetDescription, SheetHeader, SheetTitle } from '@/components/ui/sheet'

class Boundary extends Component<{ children: ReactNode }, { failed: boolean }> {
  state = { failed: false }
  static getDerivedStateFromError() { return { failed: true } }
  render() {
    if (this.state.failed) return <p role="alert" className="m-auto p-8 text-center text-faint">This needs the PC. Try again when it's on.</p>
    return this.props.children
  }
}

/** A till action's panel draws its own dialog, as on the PC. */
export function BarePanel({ children }: { children: ReactNode }) {
  return <Boundary><Suspense fallback={null}>{children}</Suspense></Boundary>
}

/** A plugin's till screen on the phone: the same component as on the PC, full height. */
export function PluginPanel({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent side="bottom" className="h-[92dvh] gap-2 overflow-y-auto p-3">
        <SheetHeader className="p-0 pr-8">
          <SheetTitle>{title}</SheetTitle>
          <SheetDescription className="sr-only">{title} on this phone</SheetDescription>
        </SheetHeader>
        <Boundary>
          <Suspense fallback={<p className="m-auto p-8 text-center text-faint">Opening…</p>}>{children}</Suspense>
        </Boundary>
      </SheetContent>
    </Sheet>
  )
}
