import { useState } from 'react'
import type { LabelRequest, ShopSettings } from '@/api/types.gen'
import { Field, INPUT } from '@/components/admin/fields'
import { LabelPreview } from '@/features/counter/labels/LabelPreview'
import { saveSlot, slotDefault } from '@/features/counter/labels/slots'
import { PRICE_SAMPLE } from '@/features/print/labels/price'
import { Segmented } from './kit'

const FONT = [{ value: '1', label: 'XS' }, { value: '2', label: 'S' }, { value: '3', label: 'M' }, { value: '4', label: 'L' }]
const LINES = [1, 2, 3, 4].map((n) => ({ value: n, label: String(n) }))
const SLOTS = [{ value: 'name', label: 'Name' }, { value: 'price', label: 'Price' }, { value: 'none', label: 'None' }]
const TEXT: Record<string, string> = { name: PRICE_SAMPLE.first.text, price: PRICE_SAMPLE.second.text, none: '' }

const int = (s: string) => Math.max(0, Math.trunc(Number(s) || 0))

/** Settings → Printers & labels, the label half: Image / Classic, the default
 * size, each text's lines and size — and the made-up product as it would print
 * with these (unsaved) choices. First / Second pick what the sample shows,
 * remembered on this device like every other label screen. */
export function LabelSettings({ v, set }: { v: ShopSettings; set: <K extends keyof ShopSettings>(k: K, x: ShopSettings[K]) => void }) {
  const [top, setTop] = useState(() => slotDefault('top', 'name'))
  const [bottom, setBottom] = useState(() => slotDefault('bottom', 'price'))
  const pick = (which: 'top' | 'bottom', k: string) => {
    saveSlot(which, k)
    ;(which === 'top' ? setTop : setBottom)(k)
  }
  const req: LabelRequest = {
    productId: 0, code: PRICE_SAMPLE.code, format: PRICE_SAMPLE.format, count: 1, size: `${v.labelWidthMm}x${v.labelHeightMm}`,
    top: TEXT[top] ?? '', bottom: TEXT[bottom] ?? '', topKey: top, bottomKey: bottom,
    look: { firstLines: v.labelNameLines, secondLines: v.labelBottomLines, firstSize: v.labelNameFont, secondSize: v.labelBottomFont },
  }
  return (
    <div className="grid gap-4 lg:grid-cols-[1fr_minmax(0,320px)]">
      <div className="grid min-w-0 gap-3">
        <div>
          <Segmented label="Print labels as" value={v.labelPrintAs} onChange={(x) => set('labelPrintAs', x)} options={[{ value: 'image', label: 'Image (new)' }, { value: 'classic', label: 'Classic' }]} />
          <small className="mt-1 block text-[12px] text-faint">Image prints the label in the shop’s label style (Sinhala and Tamil too). Classic is the old way.</small>
        </div>
        <div className="grid grid-cols-3 gap-3">
          <Field label="Width mm"><input className={INPUT} inputMode="numeric" value={v.labelWidthMm} onChange={(e) => set('labelWidthMm', int(e.target.value))} /></Field>
          <Field label="Height mm"><input className={INPUT} inputMode="numeric" value={v.labelHeightMm} onChange={(e) => set('labelHeightMm', int(e.target.value))} /></Field>
          <Field label="Gap mm"><input className={INPUT} inputMode="numeric" value={v.labelGapMm} onChange={(e) => set('labelGapMm', int(e.target.value))} /></Field>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <Segmented label="First text lines" value={v.labelNameLines} onChange={(x) => set('labelNameLines', x)} options={LINES} />
          <Segmented label="Second text lines" value={v.labelBottomLines} onChange={(x) => set('labelBottomLines', x)} options={LINES} />
          <Segmented label="First text size" value={v.labelNameFont} onChange={(x) => set('labelNameFont', x)} options={FONT} />
          <Segmented label="Second text size" value={v.labelBottomFont} onChange={(x) => set('labelBottomFont', x)} options={FONT} />
        </div>
      </div>
      <div className="flex min-w-0 flex-col gap-2 rounded-[var(--radius-sm)] bg-muted p-3">
        <span className="text-xs font-bold text-muted-foreground">Sample label</span>
        <Segmented label="First text" value={top} onChange={(k) => pick('top', k)} options={SLOTS} />
        <Segmented label="Second text" value={bottom} onChange={(k) => pick('bottom', k)} options={SLOTS} />
        <LabelPreview req={req} mode={v.labelPrintAs} />
      </div>
    </div>
  )
}
