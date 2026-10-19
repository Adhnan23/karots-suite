import { RefreshCw } from 'lucide-react'
import { useEffect, useState } from 'react'
import { onReloading } from '@/lib/version'

/** The moment before the page reloads into a new version (no toast: it must
 * show above dialogs too). */
export function UpdatedBanner() {
  const [on, setOn] = useState(false)
  useEffect(() => onReloading(() => setOn(true)), [])
  if (!on) return null
  return (
    <div role="status" className="fixed inset-x-0 top-0 z-[100] flex items-center justify-center gap-2 bg-brand px-4 py-3 text-sm font-bold text-brand-foreground shadow-lg">
      <RefreshCw size={16} className="animate-spin" /> Karots was updated — reloading
    </div>
  )
}
