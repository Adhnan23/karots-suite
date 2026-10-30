import { describe, expect, it } from 'vitest'
import type { CopyPerson } from '@/api/types.gen'
import { checkPin } from './pin'

// Made by Go: bcrypt.GenerateFromPassword([]byte("1234"), bcrypt.DefaultCost) — the hash the server keeps.
const person = { id: 1, name: 'Nimal', pinHash: '$2a$10$HkDWx2vc4aR.ohZ49LQgWu77AYvWmFIAQurvNFd5Hrw.KmxJwTg5m' } as CopyPerson

describe('checkPin (offline sign-in)', () => {
  it('accepts the right PIN against the server hash', async () => {
    expect(await checkPin(person, '1234')).toBe(true)
  })
  it('refuses a wrong PIN', async () => {
    expect(await checkPin(person, '4321')).toBe(false)
  })
})
