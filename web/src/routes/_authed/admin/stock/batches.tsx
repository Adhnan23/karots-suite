import { createFileRoute } from '@tanstack/react-router'
import { BatchesPage } from '@/features/admin/stock/BatchesPage'

export const Route = createFileRoute('/_authed/admin/stock/batches')({ component: BatchesPage })
