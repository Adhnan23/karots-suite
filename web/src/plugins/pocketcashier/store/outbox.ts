import { open, seal, type Sealed } from './crypto'
import { done, openDb, req } from './db'

export type ActionKind = 'open' | 'sale' | 'hold' | 'unhold' | 'withdraw' | 'deposit' | 'close' | 'call'

/** Something the phone did, waiting to be sent: its one-time ref makes a
 * repeated send harmless; `at` is when it happened on the phone. */
export type Action = { ref: string; kind: ActionKind; userId: number; at: string; body: unknown }

type Row = { seq?: number; ref: string; sealed: Sealed }

export async function addAction(a: Omit<Action, 'ref' | 'at'>): Promise<Action> {
  const full: Action = { ...a, ref: crypto.randomUUID(), at: new Date().toISOString() }
  const row: Row = { ref: full.ref, sealed: await seal(full) }
  const tx = (await openDb()).transaction('outbox', 'readwrite')
  tx.objectStore('outbox').add(row)
  await done(tx)
  return full
}

/** Every action not yet acknowledged, oldest first. */
export async function pending(): Promise<Action[]> {
  const rows = await req<Row[]>((await openDb()).transaction('outbox').objectStore('outbox').getAll())
  return Promise.all(rows.map((r) => open<Action>(r.sealed)))
}

/** The server answered for these refs (applied, skipped or failed): drop them. */
export async function ack(refs: string[]) {
  const tx = (await openDb()).transaction('outbox', 'readwrite')
  const byRef = tx.objectStore('outbox').index('ref')
  for (const ref of refs) {
    const r = byRef.getKey(ref)
    r.onsuccess = () => { if (r.result !== undefined) tx.objectStore('outbox').delete(r.result) }
  }
  await done(tx)
}
