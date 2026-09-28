import { useRef } from 'react'
import { slots } from '@/plugins/slots'

/** Plugin drawer sections for the Open/Close dialog. runAll() saves each; false aborts. */
export function useDrawerSections(which: 'open' | 'close') {
  const saves = useRef(new Map<string, () => Promise<boolean>>())
  const nodes = slots.drawerSections.map((s) => (
    <s.Section key={s.key} which={which} register={(fn) => saves.current.set(s.key, fn)} />
  ))
  async function runAll() {
    for (const save of saves.current.values()) if (!(await save())) return false
    return true
  }
  return { nodes, runAll }
}
