import { createFileRoute, stripSearchParams } from '@tanstack/react-router'
import { LockersPage } from '@/features/money/lockers/LockersPage'

const defaults = { disabled: false }

export const Route = createFileRoute('/_authed/admin/lockers/')({
  validateSearch: (s: Record<string, unknown>): Partial<typeof defaults> => ({ disabled: s.disabled === true || s.disabled === 'true' }),
  search: { middlewares: [stripSearchParams(defaults)] },
  component: LockersPage,
})
