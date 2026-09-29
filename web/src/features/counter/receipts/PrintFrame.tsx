import { Printer } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from '@/components/ui/button'

/** A printable page: the toolbar shows on screen only. */
export function PrintFrame({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="min-h-screen bg-background py-6 print:bg-paper print:py-0">
      <div className="mx-auto mb-4 flex max-w-[760px] items-center justify-between px-4 print:hidden">
        <b className="text-muted-foreground">{title}</b>
        <Button onClick={() => window.print()}><Printer size={16} /> Print</Button>
      </div>
      {children}
    </div>
  )
}
