import { createFileRoute } from '@tanstack/react-router'
import { ReturnsTab } from '@/features/counter/returns/ReturnsTab'

export const Route = createFileRoute('/_authed/till/returns')({ component: ReturnsTab })
