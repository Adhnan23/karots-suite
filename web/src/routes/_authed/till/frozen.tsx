import { createFileRoute } from '@tanstack/react-router'
import { FrozenTab } from '@/features/counter/frozen/FrozenTab'

export const Route = createFileRoute('/_authed/till/frozen')({ component: FrozenTab })
