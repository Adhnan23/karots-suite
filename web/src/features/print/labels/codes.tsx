import JsBarcode from 'jsbarcode'
import QRCode from 'qrcode'

const JS_FORMAT: Record<string, string> = { '': 'CODE128', CODE128: 'CODE128', EAN13: 'EAN13', EAN8: 'EAN8', UPC: 'UPC', CODE39: 'CODE39' }

/** A 1D code's modules (true = bar), or null when the value isn't valid for the format. */
export function barModules(code: string, format: string): boolean[] | null {
  if (!code) return null
  try {
    const out: { encodings?: { data: string }[] } = {}
    JsBarcode(out, code, { format: JS_FORMAT[format] ?? 'CODE128', flat: true })
    return (out.encodings ?? []).flatMap((e) => [...e.data].map((c) => c === '1'))
  } catch {
    return null
  }
}

/** Quiet zone each side, in modules (1D ≥ 10, QR ≥ 2 — spec). */
export const BAR_QUIET = 10
export const QR_QUIET = 2
/** The thinnest a module may print and still scan off 203 dpi paper. */
export const MIN_MODULE = 2

/** The module width (dots) that fits `modules` + quiet zones in `width`, capped
 * at `max`; 0 when even MIN_MODULE doesn't fit. */
export function fitModule(modules: number, quiet: number, width: number, max = 3): number {
  const m = Math.min(max, Math.floor(width / (modules + 2 * quiet)))
  return m >= MIN_MODULE ? m : 0
}

/** QR modules per side for `value` (ECC M). */
export const qrSize = (value: string) => QRCode.create(value, { errorCorrectionLevel: 'M' }).modules.size

/** A 1D barcode at whole-dot `module` width, quiet zone included, centred;
 * `stretch` fills its box's height instead of `height` dots. */
export function Bars({ code, format, module, height, stretch }: { code: string; format: string; module: number; height: number; stretch?: boolean }) {
  const mods = barModules(code, format)
  if (!mods || module < MIN_MODULE) return null
  const side = (mods.length + 2 * BAR_QUIET) * module
  let path = ''
  mods.forEach((on, i) => { if (on) path += `M${(i + BAR_QUIET) * module} 0h${module}v${height}h-${module}z` })
  return (
    <svg data-code="bars" width={side} height={stretch ? '100%' : height} viewBox={`0 0 ${side} ${height}`} preserveAspectRatio="none" shapeRendering="crispEdges"
      style={{ display: 'block', margin: '0 auto', background: '#fff', flex: 'none' }}>
      <path d={path} fill="#000" />
    </svg>
  )
}

/** A QR of exactly `value` at whole-dot `module` width with a 2-module quiet zone. */
export function Qr({ value, module }: { value: string; module: number }) {
  if (!value || module < MIN_MODULE) return null
  const { modules } = QRCode.create(value, { errorCorrectionLevel: 'M' })
  const n = modules.size
  let path = ''
  for (let y = 0; y < n; y++) for (let x = 0; x < n; x++) if (modules.get(y, x)) path += `M${(x + QR_QUIET) * module} ${(y + QR_QUIET) * module}h${module}v${module}h-${module}z`
  const side = (n + 2 * QR_QUIET) * module
  return (
    <svg data-code="qr" width={side} height={side} viewBox={`0 0 ${side} ${side}`} shapeRendering="crispEdges" style={{ display: 'block', background: '#fff', flex: 'none' }}>
      <path d={path} fill="#000" />
    </svg>
  )
}
