import QRCode from 'qrcode'
import { code128 } from './barcode'

/** A Code 128 of `value`, bars at whole dots (`module` dots per module) with a
 * 10-module quiet zone each side, centred. Solid black on white — it scans off
 * thermal paper. */
export function Code128({ value, module = 2, height = 48 }: { value: string; module?: number; height?: number }) {
  const widths = code128(value)
  const quiet = 10 * module
  let x = quiet
  const bars = widths.map((w, i) => {
    const bar = i % 2 === 0 ? { left: x, width: w * module } : null
    x += w * module
    return bar
  })
  return (
    <div style={{ position: 'relative', width: x + quiet, height, margin: '0 auto', background: '#fff' }}>
      {bars.map((b, i) => b && <div key={i} style={{ position: 'absolute', top: 0, left: b.left, width: b.width, height: '100%', background: '#000' }} />)}
    </div>
  )
}

/** A QR of `value` about `size` dots square (whole-dot modules, 4-module quiet zone). */
export function QR({ value, size }: { value: string; size: number }) {
  const { modules } = QRCode.create(value, { errorCorrectionLevel: 'M' })
  const n = modules.size
  const m = Math.max(2, Math.floor(size / (n + 8)))
  let path = ''
  for (let y = 0; y < n; y++) for (let x = 0; x < n; x++) if (modules.get(y, x)) path += `M${(x + 4) * m} ${(y + 4) * m}h${m}v${m}h-${m}z`
  const side = (n + 8) * m
  return (
    <svg width={side} height={side} viewBox={`0 0 ${side} ${side}`} shapeRendering="crispEdges" style={{ display: 'block', margin: '0 auto', background: '#fff' }}>
      <path d={path} fill="#000" />
    </svg>
  )
}
