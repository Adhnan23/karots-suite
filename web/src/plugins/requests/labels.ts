const day = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()

/** "today", "yesterday", "5 days ago" — by calendar day on this device. */
export function whenLabel(iso: string, now = new Date()): string {
  const n = Math.round((day(now) - day(new Date(iso))) / 86_400_000)
  return n <= 0 ? 'today' : n === 1 ? 'yesterday' : `${n} days ago`
}

/** "Asked 3× · last asked 2 days ago", or "Asked today" for one ask. */
export function askLabel(count: number, lastAsked: string, now = new Date()): string {
  const when = whenLabel(lastAsked, now)
  return count <= 1 ? `Asked ${when}` : `Asked ${count}× · last asked ${when}`
}
