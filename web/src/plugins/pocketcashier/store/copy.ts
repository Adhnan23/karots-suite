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
