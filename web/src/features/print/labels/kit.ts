import type { ComponentType } from 'react'

/** One label text: which field it shows (`name`, `price`, `f:<label>` for a
 * "print on label" field, `free` for typed or plugin text, `none`) and the text. */
export type Slot = { key: string; text: string }

/** A price (barcode) label. First / second are the shop's two text choices;
 * a style decides where each goes. Lines 1–4 and size '1'–'4' (XS–L) per text. */
export type PriceLabel = {
  first: Slot
  second: Slot
  code: string
  format: string
  firstLines: number
  secondLines: number
  firstSize: string
  secondSize: string
}

/** The sticker: millimetres and printer dots (8 per mm, 203 dpi). */
export type LabelPaper = { wMm: number; hMm: number; w: number; h: number }

export const DOTS_PER_MM = 8

/** "WxH" mm → the paper; blank / junk → `fallback`. Clamped to 10–200 mm like the server. */
export function paperOf(size: string, fallback = '50x25'): LabelPaper {
  const m = /^\s*(\d+)\s*x\s*(\d+)\s*$/i.exec(size) ?? /^(\d+)x(\d+)$/.exec(fallback) ?? ['', '50', '25']
  const clamp = (n: number) => Math.min(200, Math.max(10, n))
  const wMm = clamp(Number(m[1]))
  const hMm = clamp(Number(m[2]))
  return { wMm, hMm, w: wMm * DOTS_PER_MM, h: hMm * DOTS_PER_MM }
}

export type LabelTemplateProps<D> = { d: D; paper: LabelPaper }

/** One look for a label kind. `minWidthMm`: narrower paper draws `fallback`
 * instead; `fits`: false when the label is too small for this style's codes. */
export type LabelStyle<D> = {
  key: string
  label: string
  Template: ComponentType<LabelTemplateProps<D>>
  minWidthMm?: number
  fallback?: string
  fits?: (d: D, paper: LabelPaper) => boolean
}

/** A kind of label (core `price`; plugins add theirs through slots.labelTemplates). */
// eslint-disable-next-line @typescript-eslint/no-explicit-any -- a kind's data is its own; the registry holds them all
export type LabelKind<D = any> = {
  kind: string
  label: string
  /** A made-up label for Appearance's thumbnails and Settings' preview. */
  sample: D
  /** Must include `classic`, the first entry. */
  styles: LabelStyle<D>[]
}
