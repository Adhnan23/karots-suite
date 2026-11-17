import type { SellLayout } from '@/api/types.gen'

/** "Try here" (Appearance → Sell page layout): an unsaved layout the system user is
 * trying on this tab's Sell page, and the Appearance draft to come back to. Per tab;
 * storage that throws (private window) just means no trial. */
const KEY = 'ks-sell-try'

export type Trial = { layout: SellLayout; draft: Record<string, SellLayout>; sel: string }

export function readTrial(): Trial | null {
  try {
    const t = JSON.parse(sessionStorage.getItem(KEY) ?? 'null') as Trial | null
    return t?.layout && t.draft ? t : null
  } catch {
    return null
  }
}

export function startTrial(t: Trial) {
  try { sessionStorage.setItem(KEY, JSON.stringify(t)) } catch { /* no trial */ }
}

export function endTrial() {
  try { sessionStorage.removeItem(KEY) } catch { /* nothing kept */ }
}
