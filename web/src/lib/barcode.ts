/** True when a scanned / typed code is the product's main barcode or one of its extras. */
export function hasBarcode(p: { barcode: string; barcodes?: string[] }, code: string): boolean {
  const c = code.trim()
  return c !== '' && (p.barcode === c || (p.barcodes ?? []).includes(c))
}

const day = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()

/** "scanned today / yesterday / 12 days ago / 4 months ago", by calendar day on this
 * device; "never scanned" for '' (not scanned since it was added). */
export function scannedLabel(iso: string, now = new Date()): string {
  if (!iso) return 'never scanned'
  const n = Math.round((day(now) - day(new Date(iso))) / 86_400_000)
  if (n <= 0) return 'scanned today'
  if (n === 1) return 'scanned yesterday'
  return n < 60 ? `scanned ${n} days ago` : `scanned ${Math.floor(n / 30)} months ago`
}
