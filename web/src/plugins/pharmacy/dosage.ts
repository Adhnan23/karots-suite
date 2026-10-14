/** Mirrors Go DosesPerDay: Sri Lankan morning-noon-night counts (1-0-1 = 2 a day,
 * 1/2-0-1/2 = 1, 1-1-1-1 = 4) or od / bd / tds / qid. 0 = can't tell. */
const WORDS: Record<string, number> = { od: 1, mane: 1, nocte: 1, bd: 2, bid: 2, tds: 3, tid: 3, qds: 4, qid: 4 }

function amount(p: string): number | null {
  const [a, b] = p.split('/')
  if (b !== undefined) {
    const n = Number(a), d = Number(b)
    return a !== '' && Number.isFinite(n) && d > 0 ? n / d : null
  }
  const v = Number(p)
  return p !== '' && Number.isFinite(v) && v >= 0 ? v : null
}

export function dosesPerDay(dosage: string): number {
  const s = dosage.trim().toLowerCase()
  if (WORDS[s]) return WORDS[s]
  const parts = s.replaceAll('½', '1/2').split('-')
  if (parts.length < 3 || parts.length > 4) return 0
  let sum = 0
  for (const p of parts) {
    const v = amount(p.trim())
    if (v === null) return 0
    sum += v
  }
  return sum
}

/** Whole days tablets last at perDay a day (0 = not tracked). */
export function supplyDays(tablets: number, perDay: number): number {
  return perDay > 0 && tablets > 0 ? Math.floor(tablets / perDay) : 0
}

/** Tablets for perDay a day over days (a part tablet rounds up: never short). */
export function tabletsFor(perDay: number, days: number): number {
  return perDay > 0 && days > 0 ? Math.ceil(perDay * days - 1e-9) : 0
}
