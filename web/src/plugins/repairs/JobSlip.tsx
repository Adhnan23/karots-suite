import type { SlipShop } from '@/api/types.gen'
import { display, Footer, Header, money, pos, ReceiptPage, Row, Rule, Text, Title } from '@/features/print/kit'

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
 * ticket while it's open, the work done and its warranty once collected. */
export function JobSlip({ d: raw }: { d: unknown }) {
  const d = raw as JobSlipData
  const j = d.job
  const parts = j.parts ?? []
  const charges = j.charges ?? []
  return (
    <ReceiptPage dots={d.dots} columns={d.columns}>
      <Header shop={d.shop} dots={d.dots} />
      <Title>Repair</Title>
      <Rule />
      <Row bold l="Ticket:" r={j.ticketNo} />
      <Row l="Received:" r={j.received} />
      {j.customer && <Row l="Customer:" r={j.customer} />}
      {j.device && <Text>{j.device}</Text>}
      {j.fault && <Text>{`Fault: ${j.fault}`}</Text>}
      {j.readyBy && <Row l="Ready by:" r={j.readyBy} />}
      {j.warrantyUntil && <Row l="Warranty until:" r={j.warrantyUntil} />}
      {parts.length + charges.length > 0 && (
        <>
          <Rule />
          {parts.map((p, i) => <Row key={i} l={p.name} r={'x' + display(p.qty)} />)}
          {charges.map((c, i) => <Text key={i}>{c}</Text>)}
        </>
      )}
      <Rule />
      {j.open ? (
        <>
          {pos(j.estimate) && <Row l="Estimate" r={money(d.currency, j.estimate)} />}
          {pos(j.deposit) && <Row bold l="Deposit paid" r={money(d.currency, j.deposit)} />}
          <Text center>Bring this slip to collect.</Text>
        </>
      ) : (
        j.paidOn && <Row l="Paid on receipt" r={j.paidOn} />
      )}
      <Footer shop={d.shop} />
    </ReceiptPage>
  )
}
