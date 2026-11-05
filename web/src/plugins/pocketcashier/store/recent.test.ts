import 'fake-indexeddb/auto'
import { beforeEach, describe, expect, it } from 'vitest'
import type { RecentSales, SaleView } from '@/api/types.gen'
import { clearRecent, findSales, findUnit, getSale, recentCursor, saveRecent } from './recent'
import { openDb, req } from './db'

const NOW = new Date('2026-11-05T12:00:00Z')
const daysAgo = (n: number) => new Date(NOW.getTime() - n * 86_400_000).toISOString()

const aSale = (id: number, over: Partial<SaleView> = {}): SaleView => ({
  id, receiptNo: `R-${id}`, createdAt: daysAgo(1), customerName: 'Walk-in', cashierName: 'Nimal', total: '100.00', status: 'completed',
  customerId: 0, subtotal: '100.00', discount: '0.00', tax: '0.00', paidAmount: '100.00', changeGiven: '0.00', owedOnSale: '0.00',
  lines: [{ name: 'Panadol' } as SaleView['lines'][number]], payments: [], flags: [], tillSource: '', ...over,
})

const aPage = (over: Partial<RecentSales>): RecentSales => ({
  sales: [], changed: [], nextAfter: 0, nextSince: daysAgo(0), more: false, maxId: 0, reset: false, ...over,
})

describe('the sales kept on the phone', () => {
  beforeEach(clearRecent)

  it('keeps a sale sealed, so nothing can be read outside the app', async () => {
    await saveRecent(aPage({ sales: [aSale(1, { customerName: 'Sunil Perera' })], nextAfter: 1 }), NOW)
    const raw = await req((await openDb()).transaction('recent').objectStore('recent').get(1))
    expect(JSON.stringify(raw)).not.toContain('Sunil')
    expect((await getSale(1))?.customerName).toBe('Sunil Perera')
  })

  it('a changed sale replaces its row', async () => {
    await saveRecent(aPage({ sales: [aSale(1)], nextAfter: 1 }), NOW)
    await saveRecent(aPage({ changed: [aSale(1, { status: 'returned' })], nextAfter: 1 }), NOW)
    expect((await getSale(1))?.status).toBe('returned')
  })

  it('drops sales past 30 days', async () => {
    await saveRecent(aPage({ sales: [aSale(1, { createdAt: daysAgo(31) }), aSale(2, { createdAt: daysAgo(29) })], nextAfter: 2 }), NOW)
    expect(await getSale(1)).toBeNull()
    expect(await getSale(2)).not.toBeNull()
  })

  it('a reset throws away what it holds', async () => {
    await saveRecent(aPage({ sales: [aSale(5)], nextAfter: 5 }), NOW)
    await saveRecent(aPage({ reset: true, sales: [aSale(2)], nextAfter: 2 }), NOW)
    expect(await getSale(5)).toBeNull()
    expect(await getSale(2)).not.toBeNull()
    expect((await recentCursor()).after).toBe(2)
  })

  it('finds by receipt, customer and product, every word, newest first', async () => {
    await saveRecent(aPage({ nextAfter: 3, sales: [
      aSale(1, { customerName: 'Sunil Perera' }),
      aSale(2, { lines: [{ name: 'Amoxicillin 500mg' } as SaleView['lines'][number]] }),
      aSale(3, { customerName: 'Sunil Silva', lines: [{ name: 'Amoxicillin 250mg' } as SaleView['lines'][number]] }),
    ] }), NOW)
    expect((await findSales('R-2', 10)).map((s) => s.id)).toEqual([2])
    expect((await findSales('sunil', 10)).map((s) => s.id)).toEqual([3, 1])
    expect((await findSales('sunil amox', 10)).map((s) => s.id)).toEqual([3])
    expect((await findSales('', 2)).map((s) => s.id)).toEqual([3, 2])
  })

  it('the cursor is ready only when the last page is in, and since dates from the first page', async () => {
    const first = daysAgo(0.5)
    await saveRecent(aPage({ sales: [aSale(1)], nextAfter: 1, more: true, nextSince: first }), NOW)
    expect(await recentCursor()).toMatchObject({ after: 1, since: '', ready: false })
    await saveRecent(aPage({ sales: [aSale(2)], nextAfter: 2, nextSince: daysAgo(0.1) }), NOW)
    expect(await recentCursor()).toMatchObject({ after: 2, since: first, ready: true })
  })

  it('finds a sold unit by serial, ignoring case and spaces', async () => {
    const unit = { id: 3, serial: 'SN-77', productName: 'Phone' } as NonNullable<SaleView['warranty']>['units'][number]
    await saveRecent(aPage({ nextAfter: 2, sales: [aSale(1), aSale(2, { warranty: { units: [unit] } })] }), NOW)
    expect((await findUnit(' sn-77 '))?.id).toBe(3)
    expect(await findUnit('nope')).toBeNull()
  })
})
