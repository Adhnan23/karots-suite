import { open, seal, type Sealed } from './crypto'
import { done, openDb, req } from './db'

/** This phone's pairing: its key (sent as X-Ks-Phone), its name, its device id. */
export type PhoneId = { secret: string; name: string; device: string }

export async function savePhone(p: PhoneId) {
  const sealed = await seal(p)
  const tx = (await openDb()).transaction('phone', 'readwrite')
  tx.objectStore('phone').put(sealed, 'me')
  await done(tx)
}

export async function loadPhone(): Promise<PhoneId | null> {
  const s = await req<Sealed | undefined>((await openDb()).transaction('phone').objectStore('phone').get('me'))
  return s ? open<PhoneId>(s) : null
}

export async function forgetPhone() {
  const tx = (await openDb()).transaction('phone', 'readwrite')
  tx.objectStore('phone').delete('me')
  await done(tx)
}
