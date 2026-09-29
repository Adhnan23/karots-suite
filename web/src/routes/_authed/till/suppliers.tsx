import { createFileRoute } from '@tanstack/react-router'
import { SuppliersTab } from '@/features/counter/suppliers/SuppliersTab'

export const Route = createFileRoute('/_authed/till/suppliers')({ component: SuppliersTab })
