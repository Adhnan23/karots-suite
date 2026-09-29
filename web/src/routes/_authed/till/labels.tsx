import { createFileRoute } from '@tanstack/react-router'
import { LabelsTab } from '@/features/counter/labels/LabelsTab'

export const Route = createFileRoute('/_authed/till/labels')({ component: LabelsTab })
