import { Qr, QR_QUIET, qrSize } from '@/features/print/labels/codes'
import { Fit } from '@/features/print/labels/fit'
import { INK, type LabelPaper, type LabelTemplateProps } from '@/features/print/labels/kit'
import { SANS_FAMILY } from '@/features/print/paper'
import type { TicketLabel } from '.'

const PAD = 12
const k = (p: LabelPaper) => Math.min(p.h / 200, p.w / 400)
/** The QR's whole-dot module: the label's height less padding, at most 45% of its width, ≤ 6 dots. */
const qrModule = (t: TicketLabel, p: LabelPaper) => Math.min(6, Math.floor(Math.min(p.h - 2 * PAD * k(p), p.w * 0.45) / (qrSize(t.code) + 2 * QR_QUIET)))
export const jobFits = (t: TicketLabel, p: LabelPaper) => !t.code || qrModule(t, p) >= 2

/** R-0142 → R-142: shorter to read across a shelf of phones. */
const short = (no: string) => no.replace(/-0+(?=\d)/, '-')
const day = (iso: string) => new Date(iso.length === 10 ? `${iso}T00:00` : iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short' })

/** A big job number, device, customer and dates on the left; a QR of the ticket
 * code (Sell opens the job when it's scanned) on the right. */
export function JobTicket({ d, paper }: LabelTemplateProps<TicketLabel>) {
  const s = k(paper)
  return (
    <div style={{ height: '100%', padding: `${PAD * s}px ${14 * s}px`, boxSizing: 'border-box', fontFamily: SANS_FAMILY, display: 'flex', gap: 14 * s, alignItems: 'center' }}>
      <div style={{ flex: 1, minWidth: 0, lineHeight: 1.2 }}>
        <Fit lines={1} style={{ fontSize: 44 * s, fontWeight: 900, lineHeight: 1.05, letterSpacing: -s }}>{short(d.ticketNo)}</Fit>
        <div style={{ height: Math.max(2, Math.round(3 * s)), background: INK, margin: `${4 * s}px 0 ${6 * s}px` }} />
        <Fit lines={1} style={{ fontSize: 18 * s, fontWeight: 700 }}>{d.device}</Fit>
        <Fit lines={1} style={{ fontSize: 16 * s }}>{[d.customer, d.phone].filter(Boolean).join(' · ')}</Fit>
        <Fit lines={1} style={{ fontSize: 15 * s, fontWeight: 600 }}>In {day(d.received)}{d.readyBy && ` → ${day(d.readyBy)}`}</Fit>
      </div>
      <Qr value={d.code} module={qrModule(d, paper)} />
    </div>
  )
}
