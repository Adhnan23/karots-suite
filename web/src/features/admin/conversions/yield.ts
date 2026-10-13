import { qtyText, r2 } from '@/lib/money'

/**
 * The line under a run's "Got" box when the real yield differs from the ratio:
 * "0.4 kg less than 25 — cost per kg Rs 2.03 higher". The bags' cost spreads
 * over what really came out, so less out = each unit costs more. Leave cost out
 * (the counter) and only the quantity is said. Empty when nothing differs.
 */
export function yieldNote(packs: number, expected: number, got: number, unit: string, cost?: { sym: string; perPack: number }): string {
  const diff = Number((got - expected).toFixed(3))
  if (!(packs > 0) || !(got > 0) || diff === 0) return ''
  const qty = `${qtyText(Math.abs(diff))} ${unit} ${diff < 0 ? 'less' : 'more'} than ${qtyText(expected)}`
  if (!cost || !(cost.perPack > 0)) return qty
  const delta = r2((cost.perPack * packs) / got - (cost.perPack * packs) / expected)
  if (delta === 0) return qty
  return `${qty} — cost per ${unit} ${cost.sym} ${Math.abs(delta).toFixed(2)} ${delta > 0 ? 'higher' : 'lower'}`
}
