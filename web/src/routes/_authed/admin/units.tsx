import { createFileRoute } from '@tanstack/react-router'
import { UnitsPage } from '@/features/admin/units/UnitsPage'

export const Route = createFileRoute('/_authed/admin/units')({ component: UnitsPage })
