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

const SPARE_ROWS = 24 // 3 mm of paper
const SIZE_PROPS = ['width', 'inline-size', 'height', 'block-size']
const KEEP = 'data-ks-keep-size'

/** Notes which sizes the slip's own styles set; the rest only came from measuring. */
function markSizesToKeep(root: HTMLElement) {
  for (const el of [root, ...Array.from(root.querySelectorAll<HTMLElement>('*'))]) {
    const whole = el === root || el instanceof SVGElement || el.matches('img, canvas')
    const set = whole ? SIZE_PROPS : SIZE_PROPS.filter((p) => el.style.getPropertyValue(p))
    el.setAttribute(KEEP, set.join(' '))
  }
}

/** The picture library freezes every box at its measured size. A browser whose text
 * measures a little differently from how it draws (a real Linux desktop's Chromium)
 * then wraps "TOTAL" inside its box and stacks lines on each other, so a box only keeps
 * the sizes the slip set and lays out its text inside the picture. */
function letItLayOut(cloned: Node) {
  if (!(cloned instanceof HTMLElement)) return
  const keep = cloned.getAttribute(KEEP)?.split(' ') ?? []
  for (const p of SIZE_PROPS) if (!keep.includes(p)) cloned.style.removeProperty(p)
}

/** A slip node drawn at printer dots (1 CSS px = 1 dot) → a 1-bit PNG exactly
 * `dots` wide (and exactly `rows` tall when given — a label). Throws if it can't
 * be made exactly; the caller prints Classic. */
export async function rasterize(node: HTMLElement, dots: number, fonts: readonly string[], rows?: number): Promise<Blob> {
  await fontsFor(node, fonts)
  await Promise.all(Array.from(node.querySelectorAll('img'), (i) => i.decode().catch(() => undefined)))
  // The full content height: a fractional or overflowing box must never cut off the last
  // line, and the picture's own layout (letItLayOut) can run a few dots taller than measured.
  const height = rows ?? Math.ceil(Math.max(node.scrollHeight, node.getBoundingClientRect().height)) + SPARE_ROWS
  markSizesToKeep(node)
  const canvas = await domToCanvas(node, { width: dots, height, scale: 1, backgroundColor: '#ffffff', onCloneEachNode: letItLayOut })
  if (canvas.width !== dots || (rows && canvas.height !== rows)) throw new Error(`picture is ${canvas.width}×${canvas.height}, printer wants ${dots}×${rows ?? 'any'}`)
  const ctx = canvas.getContext('2d')
  if (!ctx) throw new Error('no 2d canvas')
  const img = ctx.getImageData(0, 0, canvas.width, canvas.height)
  toOneBit(img.data)
  ctx.putImageData(img, 0, 0)
  return new Promise((ok, fail) => canvas.toBlob((b) => (b ? ok(b) : fail(new Error('no PNG'))), 'image/png'))
}
