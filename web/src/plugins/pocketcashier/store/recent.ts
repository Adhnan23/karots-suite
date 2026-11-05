import type { RecentSales, SaleView } from '@/api/types.gen'
import { open, seal, type Sealed } from './crypto'
import { done, openDb, req } from './db'

const KEEP_DAYS = 30

/** One sale, sealed; `at` stays readable only to drop it when it ages out. */
type Row = { id: number; at: number; sealed: Sealed }

export type RecentCursor = { after: number; since: string; ready: boolean; started?: string }

const EMPTY: RecentCursor = { after: 0, since: '', ready: false }

// What findSales searches: opened once, so nothing readable is ever stored.
let opened: SaleView[] | null = null

export async function recentCursor(): Promise<RecentCursor> {
  const c = await req<RecentCursor | undefined>((await openDb()).transaction('meta').objectStore('meta').get('recent'))
  return c ?? EMPTY
}

/** One page from the PC. A page dropped halfway leaves the cursor where the last
 * good one ended. `since` dates from a run's first page, so a return made while
 * the later pages were coming is still asked for. */
export async function saveRecent(page: RecentSales, now: Date): Promise<void> {
  const prev = page.reset ? EMPTY : await recentCursor()
  const rows = await Promise.all([...page.sales, ...page.changed].map(toRow))
  const started = prev.started ?? page.nextSince
  const next: RecentCursor = page.more
    ? { after: page.nextAfter, since: prev.since, ready: prev.ready && !page.reset, started }
    : { after: page.nextAfter, since: started, ready: true }
  const tx = (await openDb()).transaction(['recent', 'meta'], 'readwrite')
  const store = tx.objectStore('recent')
  if (page.reset) store.clear()
  for (const r of rows) store.put(r)
  const old = store.index('at').openCursor(IDBKeyRange.upperBound(now.getTime() - KEEP_DAYS * 86_400_000))
  old.onsuccess = () => {
    if (!old.result) return
    old.result.delete()
    old.result.continue()
  }
  tx.objectStore('meta').put(next, 'recent')
  await done(tx)
  opened = null
}

async function toRow(s: SaleView): Promise<Row> {
  return { id: s.id, at: new Date(s.createdAt).getTime(), sealed: await seal(s) }
}

export async function getSale(id: number): Promise<SaleView | null> {
  const r = await req<Row | undefined>((await openDb()).transaction('recent').objectStore('recent').get(id))
  return r ? open<SaleView>(r.sealed) : null
}

async function allSales(): Promise<SaleView[]> {
  if (opened) return opened
  const rows = await req<Row[]>((await openDb()).transaction('recent').objectStore('recent').getAll())
  const sales = await Promise.all(rows.map((r) => open<SaleView>(r.sealed)))
  opened = sales.sort((a, b) => b.id - a.id)
  return opened
}

/** Sales matching every word, by receipt, customer or product; newest first. */
export async function findSales(q: string, limit: number): Promise<SaleView[]> {
  const words = q.toLowerCase().split(/\s+/).filter(Boolean)
  const out: SaleView[] = []
  for (const s of await allSales()) {
    const text = [s.receiptNo, s.customerName, ...s.lines.map((l) => l.name)].join(' ').toLowerCase()
    if (words.every((w) => text.includes(w))) out.push(s)
    if (out.length === limit) break
  }
  return out
}

export async function clearRecent(): Promise<void> {
  const tx = (await openDb()).transaction(['recent', 'meta'], 'readwrite')
  tx.objectStore('recent').clear()
  tx.objectStore('meta').delete('recent')
  await done(tx)
  opened = null
}

/** A unit sold in the last 30 days, by its serial. */
export async function findUnit(serial: string): Promise<NonNullable<SaleView['warranty']>['units'][number] | null> {
  const want = serial.trim().toLowerCase()
  for (const s of await allSales()) {
    const hit = s.warranty?.units.find((u) => u.serial.toLowerCase() === want)
    if (hit) return hit
  }
  return null
}
