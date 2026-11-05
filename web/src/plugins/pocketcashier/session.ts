/** Who is signed in on the phone: in memory, and in sessionStorage so a reload
 * keeps them (closing the app signs them out). */
/** session: signed in through the PC, so the phone's cookie is this person's. */
export type Who = { id: number; name: string; role: string; since?: string; session?: boolean }

const KEY = 'ks-pocket-who'

export function loadWho(): Who | null {
  try {
    const v = JSON.parse(sessionStorage.getItem(KEY) ?? 'null') as Who | null
    return v && typeof v.id === 'number' ? v : null
  } catch {
    return null
  }
}

export function saveWho(w: Who | null) {
  try {
    if (w) sessionStorage.setItem(KEY, JSON.stringify({ since: new Date().toISOString(), ...w }))
    else sessionStorage.removeItem(KEY)
  } catch { /* storage blocked: memory only */ }
}
