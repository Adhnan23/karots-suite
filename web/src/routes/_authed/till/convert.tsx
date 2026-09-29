import { createFileRoute } from '@tanstack/react-router'
import { ConvertTab } from '@/features/counter/stock/ConvertTab'
import { conversionsQuery } from '@/features/counter/queries'

export const Route = createFileRoute('/_authed/till/convert')({
  loader: ({ context }) => context.queryClient.ensureQueryData(conversionsQuery),
  component: ConvertTab,
})
