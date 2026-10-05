import { createFileRoute } from '@tanstack/react-router'
import { ReceiptsPage } from '@/features/money/receipts/ReceiptsPage'

export const Route = createFileRoute('/_authed/admin/receipts/cash/$id')({
  component: function CashReceipt() {
    return <ReceiptsPage cashId={Number(Route.useParams().id)} />
  },
})
