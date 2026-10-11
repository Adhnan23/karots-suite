import { slots } from '@/plugins/slots'

export type Extras = Record<string, unknown>

/** Plugin blocks on the product / intake forms (slots.productFormSections). Each
 * one's value lands in extras[key], sent as ProductRequest.extra. Renders nothing
 * in a core-only build. */
export function ProductExtras({ extras, setExtras, ...p }: { productId: number; sourceId: number; categoryId: number; where: 'form' | 'intake'; extras: Extras; setExtras: (f: (x: Extras) => Extras) => void }) {
  return slots.productFormSections.map(({ key, Section }) => (
    <Section key={key} {...p} value={extras[key]} onChange={(v) => setExtras((x) => ({ ...x, [key]: v }))} />
  ))
}

/** The extras to send: undefined (no key at all) when no plugin filled any. */
export const extraBody = (x: Extras) => (Object.keys(x).length ? x : undefined)
