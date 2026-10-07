import { createFileRoute } from '@tanstack/react-router'
import { DuesPage } from '@/features/reports/people/Dues'

export const Route = createFileRoute('/_authed/admin/reports/supplier-dues')({
  component: () => <DuesPage kind="supplier" />,
})
