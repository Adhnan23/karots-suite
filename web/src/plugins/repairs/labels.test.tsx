import { BinaryBitmap, DecodeHintType, HybridBinarizer, QRCodeReader, RGBLuminanceSource } from '@zxing/library'
import QRCode from 'qrcode'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { QR_QUIET } from '@/features/print/labels/codes'
import { paperOf } from '@/features/print/labels/kit'
import { LabelPage } from '@/features/print/labels/picture'
import { slots } from '../slots'
import repairs from '.'
import type { TicketLabel } from './labels'

repairs(slots)

const T: TicketLabel = { ticketNo: 'R-0142', code: 'RPR000142', device: 'Samsung A12', customer: 'Kamal', phone: '077 123 4567', received: '2026-10-26T09:00:00+05:30', readyBy: '2026-10-28' }

const draw = (style: string, size = '50x25') => {
  const el = document.createElement('div')
  el.innerHTML = renderToStaticMarkup(<LabelPage kind="repairs.ticket" data={T} paper={paperOf(size)} style={style} />)
  return el.firstElementChild as HTMLElement
}

function decodeQr(svg: Element) {
  const module = Number(/h(\d+)v/.exec(svg.querySelector('path')!.getAttribute('d')!)![1])
  const { modules } = QRCode.create(T.code, { errorCorrectionLevel: 'M' })
  const side = (modules.size + 2 * QR_QUIET) * module
  // Read the drawn squares back as pixels.
  const on = new Set<string>()
  for (const m of svg.querySelector('path')!.getAttribute('d')!.matchAll(/M(\d+) (\d+)/g)) on.add(`${Number(m[1]) / module},${Number(m[2]) / module}`)
  const px = new Uint8ClampedArray(side * side)
  for (let y = 0; y < side; y++) for (let x = 0; x < side; x++) px[y * side + x] = on.has(`${Math.floor(x / module)},${Math.floor(y / module)}`) ? 0 : 255
  const hints = new Map([[DecodeHintType.PURE_BARCODE, true]])
  return { module, text: new QRCodeReader().decode(new BinaryBitmap(new HybridBinarizer(new RGBLuminanceSource(px, side, side))), hints).getText() }
}

describe('repairs.ticket', () => {
  it('registers Classic, Job', () => {
    expect(slots.labelTemplates.find((k) => k.kind === 'repairs.ticket')?.styles.map((s) => s.key)).toEqual(['classic', 'job'])
  })
  it('Classic is today’s ticket label: ticket + device on top, bars of the code, customer under', () => {
    const root = draw('classic')
    expect(root.textContent).toContain('R-0142 · Samsung A12')
    expect(root.textContent).toContain('Kamal 077 123 4567')
    expect(root.querySelector('[data-code="bars"]')).not.toBeNull()
  })
  for (const size of ['50x25', '38x25'])
    it(`Job at ${size}: big R-142, device, customer, and a QR of the ticket code`, () => {
      const root = draw('job', size)
      expect(root.dataset.labelStyle).toBe('job')
      for (const s of ['R-142', 'Samsung A12', 'Kamal', '077 123 4567', '26 Oct', '28 Oct']) expect(root.textContent).toContain(s)
      const q = decodeQr(root.querySelector('[data-code="qr"]')!)
      expect(q.module).toBeGreaterThanOrEqual(2)
      expect(q.text).toBe(T.code)
    })
})
