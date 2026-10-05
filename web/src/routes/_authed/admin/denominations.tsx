import { createFileRoute } from '@tanstack/react-router'
import { DenominationsPage } from '@/features/money/tills/DenominationsPage'

export const Route = createFileRoute('/_authed/admin/denominations')({ component: DenominationsPage })
