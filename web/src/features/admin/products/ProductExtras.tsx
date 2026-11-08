import { type AssistFill, slots } from '@/plugins/slots'

export type Extras = Record<string, unknown>
/** What a helper offered each section (AssistFill.sections), by section key. */
export type Suggested = Record<string, unknown>

/** Plugin blocks on the product / intake forms (slots.productFormSections). Each
 * one's value lands in extras[key], sent as ProductRequest.extra. Renders nothing
 * in a core-only build. */
export function ProductExtras({ extras, setExtras, suggested, ...p }: { productId: number; sourceId: number; kind?: 'copy' | 'loose'; categoryId: number; where: 'form' | 'intake'; extras: Extras; setExtras: (f: (x: Extras) => Extras) => void; suggested?: Suggested }) {
  return slots.productFormSections.map(({ key, Section }) => (
    <Section key={key} {...p} value={extras[key]} suggested={suggested?.[key]} onChange={(v) => setExtras((x) => ({ ...x, [key]: v }))} />
  ))
}

/** The extras to send: undefined (no key at all) when no plugin filled any. */
export const extraBody = (x: Extras) => (Object.keys(x).length ? x : undefined)

/** Plugin helpers under the Name field (slots.productAssists); fill patches the form and puts
 * the plugin's extra in extras[key]. Renders nothing in a core-only build. */
export function ProductAssists({ name, barcode, where, onFill, setExtras, setSuggested }: { name: string; barcode: string; where: 'form' | 'intake'; onFill: (f: { name?: string; categoryId?: number }) => void; setExtras: (f: (x: Extras) => Extras) => void; setSuggested: (s: Suggested) => void }) {
  return slots.productAssists.map(({ key, Assist }) => (
    <Assist key={key} name={name} barcode={barcode} where={where} fill={({ extra, sections, ...f }: AssistFill) => {
      onFill(f)
      if (sections) setSuggested({ ...sections }) // a fresh object, so the same values offered again still land
      if (extra !== undefined) setExtras((x) => ({ ...x, [key]: extra }))
    }} />
  ))
}
