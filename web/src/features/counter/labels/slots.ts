import type { DetailRow } from '@/api/types.gen'

/** Label sizes offered everywhere (mm, WxH); the shop default goes first. */
export const LABEL_SIZES = ['50x25', '50x30', '40x30', '38x25', '100x50']

// The top/bottom field choice sticks per device, shared by the counter, admin
// labels and stock intake.
export const slotDefault = (which: 'top' | 'bottom', fallback: string) => {
  try { return localStorage.getItem(`label.slot.${which}`) || fallback } catch { return fallback }
}
export const saveSlot = (which: 'top' | 'bottom', v: string) => {
  try { localStorage.setItem(`label.slot.${which}`, v) } catch { /* private window: just don't remember */ }
}

export type SlotOption = { key: string; label: string; value: string }

/** What a label line can show: name, price, each "print on label" field, or nothing. */
export const slotOptions = (name: string, price: string, fields: DetailRow[]): SlotOption[] => [
  { key: 'name', label: 'Name', value: name },
  { key: 'price', label: 'Price', value: price },
  ...fields.map((f) => ({ key: `f:${f.label}`, label: f.label, value: f.value })),
  { key: 'none', label: '— none —', value: '' },
]
export const slotValue = (opts: SlotOption[], key: string) => opts.find((o) => o.key === key)?.value ?? ''

/** Stickers for a quantity: whole, 1..200. */
export const labelCount = (qty: number) => (Number.isFinite(qty) && qty >= 1 ? Math.min(200, Math.floor(qty)) : 1)
