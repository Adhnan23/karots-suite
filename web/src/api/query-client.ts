import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query'
import { redirectFor } from './session'

/** Any query/mutation hitting an expired/locked/forced-PIN session navigates away. */
function onSessionError(err: unknown) {
  const to = redirectFor(err)
  if (to && window.location.pathname !== to) window.location.assign(to)
}

export const queryClient = new QueryClient({
  queryCache: new QueryCache({ onError: onSessionError }),
  mutationCache: new MutationCache({ onError: onSessionError }),
  defaultOptions: { queries: { retry: (n, e) => !redirectFor(e) && n < 2 } },
})
