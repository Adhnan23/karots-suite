import type { SlipShop } from '@/api/types.gen'
import type { SlipKit } from '@/features/print/styles/kit'
import { display, money, pos } from '@/features/print/styles/money'

/** What the job print path's ?data=1 answers (Go: slipData in api_print.go). */
type JobSlipData = {
  dots: number
  columns: number
  currency: string
  shop: SlipShop
  job: {
    ticketNo: string
    received: string
    customer: string
    device: string
    fault: string
    open: boolean
    readyBy: string
    warrantyUntil: string
    parts: { name: string; qty: string }[] | null
    charges: string[] | null
    estimate: string
    deposit: string
    paidOn: string
  }
}

/** The repair job slip (Image mode), line for line from `repairSlip`: a claim
 * ticket while it's open, the work done and its warranty once collected. Drawn
 * with the style's kit, so it follows every receipt style; the ticket number
 * is the slip's number (its barcode opens the job at the till). */
export function JobSlip({ d: raw, kit: K }: { d: unknown; kit: SlipKit }) {
  const d = raw as JobSlipData
  const j = d.job
  const parts = j.parts ?? []
  const charges = j.charges ?? []
  return (
    <K.Page d={d}>
      <K.Header shop={d.shop} dots={d.dots} number={j.ticketNo} />
      <K.Title>Repair</K.Title>
      <K.Rule />
      <K.Money bold l="Ticket:" r={j.ticketNo} />
      <K.Info l="Received:" r={j.received} />
      {j.customer && <K.Info l="Customer:" r={j.customer} />}
      {j.device && <K.Text>{j.device}</K.Text>}
      {j.fault && <K.Text>{`Fault: ${j.fault}`}</K.Text>}
      {j.readyBy && <K.Info l="Ready by:" r={j.readyBy} />}
      {j.warrantyUntil && <K.Info l="Warranty until:" r={j.warrantyUntil} />}
      {parts.length + charges.length > 0 && (
        <>
          <K.Rule />
          {parts.map((p, i) => (
            <K.Money key={i} l={p.name} r={'x' + display(p.qty)} />
          ))}
          {charges.map((c, i) => (
            <K.Text key={i}>{c}</K.Text>
          ))}
        </>
      )}
      <K.Rule />
      {j.open ? (
        <>
          {pos(j.estimate) && <K.Money l="Estimate" r={money(d.currency, j.estimate)} />}
          {pos(j.deposit) && <K.Money bold l="Deposit paid" r={money(d.currency, j.deposit)} />}
          <K.Text center>Bring this slip to collect.</K.Text>
        </>
      ) : (
        j.paidOn && <K.Info l="Paid on receipt" r={j.paidOn} />
      )}
      <K.Footer shop={d.shop} number={j.ticketNo} />
    </K.Page>
  )
}
