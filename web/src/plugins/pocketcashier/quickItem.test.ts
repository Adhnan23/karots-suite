import { afterEach, describe, expect, it, vi } from 'vitest'
import { createQuickItem, PC_DOWN, SIGN_IN_AGAIN } from './quickItem'

const input = { name: 'Brand new thing', price: '50', qty: '1', barcode: '', unitId: 0 }
const reply = (status: number, body: unknown) => vi.stubGlobal('fetch', vi.fn(async () => new Response(JSON.stringify(body), { status })))

describe('quick item from the phone', () => {
  afterEach(() => vi.unstubAllGlobals())

  it('hands back the product the PC made', async () => {
    reply(201, { id: 9, name: 'Brand new thing' })
    expect(await createQuickItem(input)).toEqual({ product: { id: 9, name: 'Brand new thing' } })
  })

  it('says the PC is down when it cannot be reached', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new TypeError('Failed to fetch') }))
    expect(await createQuickItem(input)).toEqual({ error: PC_DOWN })
  })

  it('asks to sign in again when the person signed in offline (no session on the PC)', async () => {
    reply(401, { error: { code: 'UNAUTHORIZED', message: 'sign in' } })
    expect(await createQuickItem(input)).toEqual({ error: SIGN_IN_AGAIN })
  })

  it("passes the PC's own refusal through", async () => {
    reply(422, { error: { code: 'VALIDATION', message: 'barcode already used' } })
    expect(await createQuickItem(input)).toEqual({ error: 'barcode already used' })
  })
})
