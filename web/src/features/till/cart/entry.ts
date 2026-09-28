export type Entry =
  | { kind: 'empty' }
  | { kind: 'quick'; barcode: string; qty: number } // phone Quick-Sell rush handoff
  | { kind: 'barcode'; code: string } // digits: try an exact barcode first (the scan path)
  | { kind: 'search'; q: string }

export function parseEntry(raw: string): Entry {
  const code = raw.trim()
  if (!code) return { kind: 'empty' }
  const m = /^KQ(\d+)X(\d+)$/i.exec(code)
  if (m) return { kind: 'quick', barcode: m[1], qty: Number(m[2]) }
  if (/^\d+$/.test(code)) return { kind: 'barcode', code }
  return { kind: 'search', q: code }
}
