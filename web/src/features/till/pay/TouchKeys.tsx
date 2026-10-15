import { useState } from 'react'
import { Seg } from '../bits'
import { NoteTray } from './NoteTray'
import { TOUCH_ONLY, TouchPad } from './TouchPad'
import { cn } from '@/lib/utils'

type Pane = 'notes' | 'keys'
const KEY = 'ks-touch-pane'
const saved = (): Pane => { try { return localStorage.getItem(KEY) === 'keys' ? 'keys' : 'notes' } catch { return 'notes' } }

/** The touch-screen helpers, one at a time: the money (cash only) or the keypad —
 * a Notes / Keys switch between them, remembered on this device. */
export function TouchKeys({ cash, notes, coins, amount, onAmount }: { cash: boolean; notes: number[]; coins: number[]; amount: string; onAmount: (v: string) => void }) {
  const [pane, setPane] = useState<Pane>(saved)
  const show: Pane = cash ? pane : 'keys'
  const switcher = cash && (
    <Seg label="Touch input" value={pane} options={[['notes', 'Notes'], ['keys', 'Keypad']]}
      onChange={(p) => { setPane(p); try { localStorage.setItem(KEY, p) } catch { /* private mode */ } }} />
  )
  return (
    <div className={cn(TOUCH_ONLY, 'gap-2')}>
      {show === 'notes' ? <NoteTray notes={notes} coins={coins} amount={amount} onChange={onAmount} switcher={switcher} /> : (
        <>
          {switcher && <div className="flex justify-end">{switcher}</div>}
          <TouchPad />
        </>
      )}
    </div>
  )
}
