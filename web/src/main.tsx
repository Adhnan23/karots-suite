import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider, createRouter } from '@tanstack/react-router'
import { routeTree } from './routeTree.gen'
import { queryClient } from '@/api/query-client'
import { startLockdown } from '@/lib/lockdown'
import { meQuery } from '@/api/session'
import type { Me } from '@/api/types.gen'
import './app.css'
import '@/plugins/enabled-plugins' // the compiled-in plugins register into the slots

export const router = createRouter({ routeTree, context: { queryClient }, defaultPreload: 'intent' })
declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}

startLockdown(() => void router.navigate({ to: '/till' }), () => !!queryClient.getQueryData<Me>(meQuery.queryKey)?.flags.isSystem)

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  </StrictMode>,
)
