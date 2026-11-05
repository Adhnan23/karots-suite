import type { TillCopy } from '@/api/types.gen'
import { open, seal, type Sealed } from './crypto'
import { done, openDb, req } from './db'

export async function saveCopy(c: TillCopy) {
  const sealed = await seal(c)
  const tx = (await openDb()).transaction('copy', 'readwrite')
  tx.objectStore('copy').put(sealed, 'current')
  await done(tx)
}

export async function loadCopy(): Promise<TillCopy | null> {
  const s = await req<Sealed | undefined>((await openDb()).transaction('copy').objectStore('copy').get('current'))
  return s ? open<TillCopy>(s) : null
}

/** What the compiled-in plugins' screens read offline, with the ETag it came with. */
export async function saveParts(parts: Record<string, unknown>, etag: string) {
  const sealed = await seal({ parts, etag })
  const tx = (await openDb()).transaction('copy', 'readwrite')
  tx.objectStore('copy').put(sealed, 'parts')
  await done(tx)
}

async function readParts(): Promise<{ parts: Record<string, unknown>; etag: string } | null> {
  const s = await req<Sealed | undefined>((await openDb()).transaction('copy').objectStore('copy').get('parts'))
  return s ? open<{ parts: Record<string, unknown>; etag: string }>(s) : null
}

export async function loadParts(): Promise<Record<string, unknown>> {
  return (await readParts())?.parts ?? {}
}

export async function partsTag(): Promise<string> {
  return (await readParts())?.etag ?? ''
}
