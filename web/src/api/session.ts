import { queryOptions } from '@tanstack/react-query'
import { api, ApiError } from './client'
import type { Me } from './types.gen'

const ROUTES: Record<string, '/login' | '/lock' | '/account/pin'> = {
  UNAUTHORIZED: '/login',
  SESSION_LOCKED: '/lock',
  PIN_CHANGE_REQUIRED: '/account/pin',
}

/** Where a failed call should send the user, or null to let the caller handle it. */
export function redirectFor(err: unknown) {
  if (!(err instanceof ApiError) || err.path.startsWith('/auth/')) return null
  return ROUTES[err.code] ?? null
}

export const meQuery = queryOptions({ queryKey: ['me'], queryFn: () => api<Me>('/me'), staleTime: 60_000, retry: false })
