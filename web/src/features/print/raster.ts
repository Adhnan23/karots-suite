import { domToCanvas } from 'modern-screenshot'

/** RGBA in place → pure ink (black) or paper (white). Threshold, never dither:
 * text must stay crisp at 203 dpi. Same rule as escpos.ImageJob (luminance < 160
 * is ink, transparent is paper), so the server reads back exactly these dots. */
export function toOneBit(px: Uint8ClampedArray) {
  for (let i = 0; i < px.length; i += 4) {
    const ink = px[i + 3] >= 128 && 0.299 * px[i] + 0.587 * px[i + 1] + 0.114 * px[i + 2] < 160
    const v = ink ? 0 : 255
    px[i] = px[i + 1] = px[i + 2] = v
    px[i + 3] = 255
  }
}

const FACES = ['400', '700', '900', 'italic 400']

/** The style's fonts this node's text needs, loaded — never a system fallback on paper. */
async function fontsFor(node: HTMLElement, fonts: readonly string[]) {
  const text = node.textContent ?? ''
  await Promise.all(fonts.flatMap((f) => FACES.map((w) => document.fonts.load(`${w} 20px "${f}"`, text))))
  await document.fonts.ready
  if (!document.fonts.check(`20px "${fonts[0]}"`, 'A')) throw new Error('receipt font not loaded')
}

/** A slip node drawn at printer dots (1 CSS px = 1 dot) → a 1-bit PNG exactly
 * `dots` wide. Throws if it can't be made exactly; the caller prints Classic. */
export async function rasterize(node: HTMLElement, dots: number, fonts: readonly string[]): Promise<Blob> {
  await fontsFor(node, fonts)
  await Promise.all(Array.from(node.querySelectorAll('img'), (i) => i.decode().catch(() => undefined)))
  // The full content height: a fractional or overflowing box must never cut off the last line.
  const height = Math.ceil(Math.max(node.scrollHeight, node.getBoundingClientRect().height))
  const canvas = await domToCanvas(node, { width: dots, height, scale: 1, backgroundColor: '#ffffff' })
  if (canvas.width !== dots) throw new Error(`receipt picture is ${canvas.width} wide, printer is ${dots}`)
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('no 2d canvas')
  const img = ctx.getImageData(0, 0, canvas.width, canvas.height)
  toOneBit(img.data)
  ctx.putImageData(img, 0, 0)
  return new Promise((ok, fail) => canvas.toBlob((b) => (b ? ok(b) : fail(new Error('no PNG'))), 'image/png'))
}
