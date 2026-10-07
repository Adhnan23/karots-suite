import { createFileRoute } from '@tanstack/react-router'
import { DuesPage } from '@/features/reports/people/Dues'

export const Route = createFileRoute('/_authed/admin/reports/customer-dues')({
  component: () => <DuesPage kind="customer" />,
})
