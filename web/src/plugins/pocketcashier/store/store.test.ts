import 'fake-indexeddb/auto'
import { describe, expect, it } from 'vitest'
import type { TillCopy } from '@/api/types.gen'
import { loadCopy, saveCopy } from './copy'
import { getKey } from './crypto'
import { openDb, req } from './db'
import { ack, addAction, pending } from './outbox'
import { loadPhone, savePhone } from './phone'

const copy = { version: 'v1', symbol: 'Rs.', people: [{ id: 1, name: 'Nimal', pinHash: '$2a$10$x' }] } as unknown as TillCopy

describe('the phone store', () => {
  it('round-trips the copy, sealed', async () => {
    await saveCopy(copy)
    expect((await loadCopy())?.version).toBe('v1')
    const db = await openDb()
    const raw = await req(db.transaction('copy').objectStore('copy').get('current'))
    expect(JSON.stringify(raw)).not.toContain('Nimal')
    expect(raw.data.byteLength).toBeGreaterThan(0)
  })
  it('keeps actions in order until acknowledged', async () => {
    const a = await addAction({ kind: 'sale', userId: 1, body: { n: 1 } })
    const b = await addAction({ kind: 'sale', userId: 1, body: { n: 2 } })
    await addAction({ kind: 'close', userId: 1, body: { n: 3 } })
    expect(a.ref).not.toBe(b.ref)
    expect((await pending()).map((x) => (x.body as { n: number }).n)).toEqual([1, 2, 3])
    await ack([a.ref, b.ref])
    const left = await pending()
    expect(left).toHaveLength(1)
    expect(left[0].kind).toBe('close')
  })
  it('never lets the key out', async () => {
    const key = await getKey()
    expect(key.extractable).toBe(false)
    await expect(crypto.subtle.exportKey('raw', key)).rejects.toBeTruthy()
  })
  it('keeps the phone key sealed', async () => {
    await savePhone({ secret: 'super-secret-0123456789', name: 'Phone 2', device: 'd1' })
    expect((await loadPhone())?.secret).toBe('super-secret-0123456789')
    const db = await openDb()
    expect(JSON.stringify(await req(db.transaction('phone').objectStore('phone').get('me')))).not.toContain('super-secret')
  })
})
