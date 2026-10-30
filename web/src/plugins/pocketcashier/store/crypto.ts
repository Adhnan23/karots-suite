import { done, openDb, req } from './db'

/** Sealed: AES-GCM with a key made on this phone, never extractable — the copy
 * and the phone's key are unreadable outside this app on this phone. */
export type Sealed = { iv: Uint8Array; data: ArrayBuffer }

let key: Promise<CryptoKey> | null = null

export function getKey(): Promise<CryptoKey> {
  key ??= (async () => {
    const db = await openDb()
    const have = await req<CryptoKey | undefined>(db.transaction('keys').objectStore('keys').get('main'))
    if (have) return have
    const k = await crypto.subtle.generateKey({ name: 'AES-GCM', length: 256 }, false, ['encrypt', 'decrypt'])
    const tx = db.transaction('keys', 'readwrite')
    tx.objectStore('keys').put(k, 'main')
    await done(tx)
    return k
  })().catch((e) => { key = null; throw e })
  return key
}

export async function seal(v: unknown): Promise<Sealed> {
  const iv = crypto.getRandomValues(new Uint8Array(12))
  const data = await crypto.subtle.encrypt({ name: 'AES-GCM', iv }, await getKey(), new TextEncoder().encode(JSON.stringify(v)))
  return { iv, data }
}

export async function open<T>(s: Sealed): Promise<T> {
  const plain = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: s.iv }, await getKey(), s.data)
  return JSON.parse(new TextDecoder().decode(plain)) as T
}
