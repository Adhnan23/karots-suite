/** The phone's own database: the sealed till copy, the outbox of actions not yet
 * sent, the phone's pairing and the key that seals them. Plain IndexedDB. */
const NAME = 'ks-pocket'

let opening: Promise<IDBDatabase> | null = null

export function openDb(): Promise<IDBDatabase> {
  opening ??= new Promise((resolve, reject) => {
    const r = indexedDB.open(NAME, 1)
    r.onupgradeneeded = () => {
      const db = r.result
      db.createObjectStore('keys')
      db.createObjectStore('copy')
      db.createObjectStore('phone')
      db.createObjectStore('outbox', { keyPath: 'seq', autoIncrement: true }).createIndex('ref', 'ref', { unique: true })
    }
    r.onsuccess = () => resolve(r.result)
    r.onerror = () => { opening = null; reject(r.error) }
  })
  return opening
}

/** An IDB request as a promise. */
export function req<T = any>(r: IDBRequest<T>): Promise<T> { // eslint-disable-line @typescript-eslint/no-explicit-any
  return new Promise((resolve, reject) => {
    r.onsuccess = () => resolve(r.result)
    r.onerror = () => reject(r.error)
  })
}

/** Resolves when a write transaction has committed. */
export function done(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = tx.onabort = () => reject(tx.error)
  })
}
