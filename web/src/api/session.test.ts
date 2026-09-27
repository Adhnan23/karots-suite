import { describe, expect, it } from 'vitest'
import { ApiError } from './client'
import { redirectFor } from './session'

const e = (status: number, code: string, path = '/products') => new ApiError(status, code, 'm', path)

describe('redirectFor', () => {
  it('routes session codes', () => {
    expect(redirectFor(e(401, 'UNAUTHORIZED'))).toBe('/login')
    expect(redirectFor(e(403, 'SESSION_LOCKED'))).toBe('/lock')
    expect(redirectFor(e(403, 'PIN_CHANGE_REQUIRED'))).toBe('/account/pin')
  })
  it('never redirects for /auth/* calls (wrong PIN is handled by the form)', () => {
    expect(redirectFor(e(401, 'UNAUTHORIZED', '/auth/login'))).toBeNull()
    expect(redirectFor(e(403, 'SESSION_LOCKED', '/auth/pin'))).toBeNull()
  })
  it('ignores other errors', () => {
    expect(redirectFor(e(409, 'TILL_OPEN'))).toBeNull()
    expect(redirectFor(new Error('x'))).toBeNull()
  })
})
