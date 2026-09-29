import { createFileRoute } from '@tanstack/react-router'
import { ReviewPage } from '@/features/admin/products/ReviewPage'

export const Route = createFileRoute('/_authed/admin/products/review')({ component: ReviewPage })
