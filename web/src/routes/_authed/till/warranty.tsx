import { createFileRoute } from '@tanstack/react-router'
import { WarrantyTab } from '@/features/counter/warranty/WarrantyTab'

export const Route = createFileRoute('/_authed/till/warranty')({ component: WarrantyTab })
