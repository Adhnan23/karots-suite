import { Hand } from 'lucide-react'
import { useState } from 'react'
import { getTouch, setTouch, type Touch } from './touch'

const NEXT: Record<Touch, Touch> = { auto: 'on', on: 'off', off: 'auto' }
const SAYS: Record<Touch, string> = {
  auto: 'Touch keys: Auto — shown on a touch screen',
  on: 'Touch keys: On — notes and keypad always shown',
  off: 'Touch keys: Off — never shown',
}

/** Till header: the note tray and keypad on this device — Auto → On → Off. */
export function TouchToggle({ className }: { className: string }) {
  const [t, setT] = useState<Touch>(getTouch)
  return (
    <button type="button" className={className} aria-label={SAYS[t]} title={SAYS[t]} aria-pressed={t === 'on'}
      onClick={() => { const n = NEXT[t]; setTouch(n); setT(n) }}>
      <Hand size={16} /> <span className="text-[12px] font-semibold">{t === 'auto' ? 'Auto' : t === 'on' ? 'On' : 'Off'}</span>
    </button>
  )
}
