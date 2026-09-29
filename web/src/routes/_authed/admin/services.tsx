import { createFileRoute } from '@tanstack/react-router'
import { ServicesPage } from '@/features/admin/services/ServicesPage'

export const Route = createFileRoute('/_authed/admin/services')({ component: ServicesPage })
