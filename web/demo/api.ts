// The launcher's JSON (cmd/demo/launcher.go).
export type Login = { role: string; name: string; phone: string; pin: string }
export type Shop = {
  key: string; name: string; blurb: string; plugins: string[] | null; palette: string; receiptStyle: string
  catalogue: string; logins: Login[]
}
export type Cheat = { name: string; label: string; group: string }
export type State = {
  installId: string; phase: 'picker' | 'building' | 'ready' | 'failed'; progress: number; step: string; error?: string
  shop: Shop | null; paused: boolean; owner: { name: string; phone: string }; shops: Shop[]; cheats: Cheat[] | null
}
export type Print = { id: number; kind: 'receipt' | 'label'; text?: string[]; kick: boolean; copies: number; at: string }

export async function call<T>(path: string, body?: unknown): Promise<T> {
  const r = await fetch(path, body === undefined ? undefined : { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body) })
  const out = await r.json().catch(() => ({}))
  if (!r.ok) throw new Error((out as { error?: string }).error || `The demo answered ${r.status}`)
  return out as T
}

export const state = () => call<State>('/__demo/state')

export function el<K extends keyof HTMLElementTagNameMap>(tag: K, attrs: Record<string, string> = {}, ...kids: (Node | string)[]) {
  const e = document.createElement(tag)
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v)
  e.append(...kids)
  return e
}
