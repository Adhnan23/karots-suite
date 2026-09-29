import { createFileRoute } from '@tanstack/react-router'
import { ConversionsPage } from '@/features/admin/conversions/ConversionsPage'

export const Route = createFileRoute('/_authed/admin/conversions')({ component: ConversionsPage })
