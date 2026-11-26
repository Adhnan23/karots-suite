/** The sign-in page's "Last closed" pill text (DRAW-20): the time only — today, yesterday or the date. */
export function lastClosedText(iso: string | undefined, now = new Date()): string | null {
  if (!iso) return null
  const at = new Date(iso)
  if (Number.isNaN(at.getTime())) return null
  const time = at.toLocaleTimeString('en-GB', { hour: 'numeric', minute: '2-digit', hour12: true }).replace(/\s?([ap])m$/i, ' $1m').toLowerCase()
  const day = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
  const days = Math.round((day(now) - day(at)) / 86_400_000)
  if (days <= 0) return `Last closed ${time}`
  if (days === 1) return `Last closed yesterday ${time}`
  return `Last closed ${at.toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })}, ${time}`
}
