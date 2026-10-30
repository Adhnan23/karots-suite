import type { TillCopy } from '@/api/types.gen'
import { Screen } from '../bits'
import type { Who } from '../session'
import type { PhoneId } from '../store/phone'
import type { SyncState } from '../sync'

/** The signed-in phone: Sell / Held / Menu (built in the next steps). */
export function Phone({ who }: { who: Who; copy: TillCopy | null; sync: SyncState; phone: PhoneId; onSignOut: () => void }) {
  return <Screen><p className="p-4 font-semibold">{who.name}</p></Screen>
}
