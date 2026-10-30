import { deviceId } from '@/lib/device'
import { loadPhone } from './store/phone'

export const BASE = '/api/v1/x/pocketcashier'

/** A call carrying this phone's key (X-Ks-Phone) and device id. Throws on a
 * network failure (the PC is down) like fetch does. */
export async function phoneFetch(path: string, init: RequestInit = {}): Promise<Response> {
  const phone = await loadPhone()
  const headers: Record<string, string> = { 'X-Ks-Device': deviceId(), ...(init.headers as Record<string, string>) }
  if (phone) headers['X-Ks-Phone'] = phone.secret
  if (init.body) headers['Content-Type'] = 'application/json'
  return fetch(BASE + path, { ...init, headers })
}
