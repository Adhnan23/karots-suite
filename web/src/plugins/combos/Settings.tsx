import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { queryClient } from '@/api/query-client'
import { Toggle } from '@/components/admin/fields'
import { errMsg } from '@/features/till/api'
import { saveSettings, settingsQuery } from './api'

/** Settings → Combos: whether the till rings a full set as the combo by itself. */
export function ComboSettings() {
  const s = useQuery(settingsQuery).data
  if (!s) return null
  function set(autoApply: boolean) {
    queryClient.setQueryData(settingsQuery.queryKey, { autoApply }) // flips now; the save refetches it
    saveSettings({ autoApply }).then(() => toast.success(autoApply ? 'Combos apply automatically' : 'Combos wait for Apply'), (e) => toast.error(errMsg(e)))
  }
  return (
    <Toggle label="Apply combos automatically"
      hint="When the cart holds a full set of a combo, the till rings it as the combo without the cashier tapping Apply."
      checked={s.autoApply} onChange={set} />
  )
}
