import { createRootRouteWithContext, Outlet } from '@tanstack/react-router'
import type { QueryClient } from '@tanstack/react-query'
import { Toaster } from '@/components/ui/sonner'
import { UpdatedBanner } from '@/features/data/UpdatedBanner'

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: () => (
    <>
      <Outlet />
      <UpdatedBanner />
      <Toaster richColors position="top-center" />
    </>
  ),
  notFoundComponent: () => (
    <div className="flex min-h-screen items-center justify-center text-muted-foreground">Not found</div>
  ),
})
