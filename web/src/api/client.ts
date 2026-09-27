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
  if (res.status === 204) return undefined as T
  const body = await res.json().catch(() => null)
  if (!res.ok) throw new ApiError(res.status, body?.error?.code ?? 'HTTP_ERROR', body?.error?.message ?? res.statusText, path)
  return body as T
}
