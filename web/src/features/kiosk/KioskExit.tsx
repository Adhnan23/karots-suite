import { toast } from 'sonner'
import { api } from '@/api/client'
import { useKonami } from '@/lib/konami'

/** Armed only when the caller has canExitKiosk (admin/system). Renders nothing. */
export function KioskExit() {
  useKonami(() => {
    api('/kiosk/exit', { method: 'POST' }).catch(() => toast.error('Kiosk exit failed'))
  })
  return null
}
