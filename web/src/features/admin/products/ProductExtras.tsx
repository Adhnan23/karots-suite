import { type AssistFill, slots } from '@/plugins/slots'

export type Extras = Record<string, unknown>

/** Plugin blocks on the product / intake forms (slots.productFormSections). Each
 * one's value lands in extras[key], sent as ProductRequest.extra. Renders nothing
 * in a core-only build. */
export function ProductExtras({ extras, setExtras, ...p }: { productId: number; sourceId: number; kind?: 'copy' | 'loose'; categoryId: number; where: 'form' | 'intake'; extras: Extras; setExtras: (f: (x: Extras) => Extras) => void }) {
  return slots.productFormSections.map(({ key, Section }) => (
    <Section key={key} {...p} value={extras[key]} onChange={(v) => setExtras((x) => ({ ...x, [key]: v }))} />
  ))
}

/** The extras to send: undefined (no key at all) when no plugin filled any. */
export const extraBody = (x: Extras) => (Object.keys(x).length ? x : undefined)

/** Plugin helpers under the Name field (slots.productAssists); fill patches the form and puts
 * the plugin's extra in extras[key]. Renders nothing in a core-only build. */
export function ProductAssists({ name, barcode, where, onFill, setExtras }: { name: string; barcode: string; where: 'form' | 'intake'; onFill: (f: { name?: string; categoryId?: number }) => void; setExtras: (f: (x: Extras) => Extras) => void }) {
  return slots.productAssists.map(({ key, Assist }) => (
    <Assist key={key} name={name} barcode={barcode} where={where} fill={({ extra, ...f }: AssistFill) => {
      onFill(f)
      if (extra !== undefined) setExtras((x) => ({ ...x, [key]: extra }))
    }} />
  ))
}
