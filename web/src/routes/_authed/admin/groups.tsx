import { createFileRoute } from '@tanstack/react-router'
import { GroupsPage } from '@/features/admin/groups/GroupsPage'

export const Route = createFileRoute('/_authed/admin/groups')({ component: GroupsPage })
