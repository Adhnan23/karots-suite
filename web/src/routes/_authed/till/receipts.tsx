import { createFileRoute } from '@tanstack/react-router'
import { ReceiptsTab } from '@/features/counter/receipts/ReceiptsTab'

export const Route = createFileRoute('/_authed/till/receipts')({ component: ReceiptsTab })
