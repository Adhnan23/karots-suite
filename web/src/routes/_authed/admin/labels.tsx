import { createFileRoute } from '@tanstack/react-router'
import { LabelsPage } from '@/features/admin/labels/LabelsPage'

export const Route = createFileRoute('/_authed/admin/labels')({ component: LabelsPage })
