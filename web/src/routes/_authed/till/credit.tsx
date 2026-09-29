import { createFileRoute } from '@tanstack/react-router'
import { CreditTab } from '@/features/counter/credit/CreditTab'

export const Route = createFileRoute('/_authed/till/credit')({ component: CreditTab })
