import { createFileRoute } from '@tanstack/react-router'
import { WriteOffTab } from '@/features/counter/stock/WriteOffTab'

export const Route = createFileRoute('/_authed/till/write-off')({ component: WriteOffTab })
