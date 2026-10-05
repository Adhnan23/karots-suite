import { createFileRoute } from '@tanstack/react-router'
import { TillPage } from '@/features/money/tills/TillPage'

export const Route = createFileRoute('/_authed/admin/tills/$id')({ component: TillPage })
