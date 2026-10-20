const KEY = 'ks-device'
let id: string | undefined

/** This browser's random id (kept in localStorage), sent as X-Ks-Device so the admin's
 * "Who's on now" can tell the shop's PCs apart. '' when storage is blocked. */
export function deviceId(): string {
  if (id !== undefined) return id
  try {
    id = localStorage.getItem(KEY) ?? ''
    // Not crypto.randomUUID: it's missing over plain http, which is how cashier PCs reach the server.
    if (!id) localStorage.setItem(KEY, (id = Date.now().toString(36) + Math.random().toString(36).slice(2, 10)))
  } catch {
    id = ''
  }
  return id
}
