import { noteVersion } from '@/lib/version'

export class ApiError extends Error {
  status: number
  code: string
  path: string
  constructor(status: number, code: string, message: string, path: string) {
    super(message)
    this.status = status
    this.code = code
    this.path = path
  }
}

type Opts = { method?: string; json?: unknown; signal?: AbortSignal }

/** Typed JSON call to /api/v1. Cookie session rides along automatically. */
export async function api<T = void>(path: string, opts: Opts = {}): Promise<T> {
  const hasBody = opts.json !== undefined
  const res = await fetch(`/api/v1${path}`, {
    method: opts.method ?? (hasBody ? 'POST' : 'GET'),
    credentials: 'same-origin',
    headers: hasBody ? { 'Content-Type': 'application/json' } : {},
    body: hasBody ? JSON.stringify(opts.json) : undefined,
    signal: opts.signal,
  })
  noteVersion(res.headers.get('X-Karots-Version'))
  if (res.status === 204) return undefined as T
  const body = await res.json().catch(() => null)
  if (!res.ok) throw new ApiError(res.status, body?.error?.code ?? 'HTTP_ERROR', body?.error?.message ?? res.statusText, path)
  return body as T
}

/** POSTs one file as multipart `field` (plus any extra form fields) to /api/v1
 * and returns the JSON reply. */
export async function apiUpload<T>(path: string, field: string, file: File, extra: Record<string, string> = {}): Promise<T> {
  const fd = new FormData()
  fd.append(field, file)
  for (const [k, v] of Object.entries(extra)) fd.append(k, v)
  const res = await fetch(`/api/v1${path}`, { method: 'POST', body: fd, credentials: 'same-origin' })
  noteVersion(res.headers.get('X-Karots-Version'))
  const body = await res.json().catch(() => null)
  if (!res.ok) throw new ApiError(res.status, body?.error?.code ?? 'HTTP_ERROR', body?.error?.message ?? res.statusText, path)
  return body as T
}
