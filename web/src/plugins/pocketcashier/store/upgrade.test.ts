import 'fake-indexeddb/auto'
import { describe, expect, it } from 'vitest'
import { openDb, req } from './db'

describe('the upgrade from release 1', () => {
  it('keeps the outbox and the copy', async () => {
    await new Promise<void>((resolve, reject) => {
      const r = indexedDB.open('ks-pocket', 1)
      r.onupgradeneeded = () => {
        const db = r.result
        db.createObjectStore('keys')
        db.createObjectStore('copy').put('the copy', 'current')
        db.createObjectStore('phone')
        const o = db.createObjectStore('outbox', { keyPath: 'seq', autoIncrement: true })
        o.createIndex('ref', 'ref', { unique: true })
        o.put({ ref: 'waiting-sale', sealed: 'x' })
      }
      r.onsuccess = () => { r.result.close(); resolve() }
      r.onerror = () => reject(r.error)
    })
    const db = await openDb()
    expect(db.version).toBe(2)
    expect(await req(db.transaction('copy').objectStore('copy').get('current'))).toBe('the copy')
    expect(await req(db.transaction('outbox').objectStore('outbox').getAll())).toHaveLength(1)
    db.close()
  })
})
