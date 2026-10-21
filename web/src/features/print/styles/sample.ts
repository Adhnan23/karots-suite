import type { SlipData, SlipItem, SlipSale } from '@/api/types.gen'

// Made-up slips with every line kind a slip can carry — for the style tests and
// the Appearance thumbnails' look checks. Amounts add up like a real sale.

const shop = {
  name: 'Karots Store',
  nameLocal: 'කැරට්ස් ස්ටෝර්',
  address: '12 Main Street, Kandy',
  phone: '081 222 3344',
  vatNo: '114-556677-7000',
  footer: 'Goods once sold are not returnable.',
  logo: '',
}

const item = (o: Partial<SlipItem>): SlipItem => ({
  name: '',
  payout: false,
  quantity: '1',
  unitAbbr: 'pc',
  unitPrice: '0',
  netUnitPrice: '0',
  packLabel: '',
  gross: '0',
  discount: '0',
  discountLabel: '',
  subtotal: '0',
  warranty: '',
  serials: [],
  group: false,
  sets: '',
  setGross: '',
  setNet: '',
  members: [],
  ...o,
})

const sale = (o: Partial<SlipSale>): SlipSale => ({
  receiptNo: 'S-00231',
  date: '2026-10-21 14:32',
  cashier: 'Kamal',
  customer: '',
  items: [],
  subtotal: '0',
  itemSavings: '0',
  billSaving: '0',
  billSavingLabel: '',
  tax: '0',
  given: '0',
  total: '0',
  youSaved: '0',
  payments: [],
  credit: false,
  fromAdvance: '0',
  due: '0',
  balance: '0',
  change: '0',
  onAccount: '0',
  changeKept: '0',
  ...o,
})

const busyItems = [
  item({ name: 'Panadol 500mg', quantity: '14', unitAbbr: 'tab', unitPrice: '20', netUnitPrice: '20', packLabel: '1 Card + 2', gross: '280', subtotal: '280' }),
  item({
    name: 'Samsung Charger 25W',
    unitPrice: '2450',
    netUnitPrice: '2205',
    gross: '2450',
    discount: '245',
    discountLabel: ' (10%)',
    subtotal: '2205',
    warranty: '12-month warranty until 2027-10-21',
    serials: ['SC-88231'],
  }),
  item({
    name: 'Breakfast Deal',
    group: true,
    sets: '2',
    setGross: '650',
    setNet: '600',
    gross: '1300',
    discount: '100',
    subtotal: '1200',
    members: [
      { name: 'Bread', quantity: '2', warranty: '', serials: [] },
      { name: 'Milk 1L', quantity: '2', warranty: '', serials: [] },
    ],
  }),
  item({ name: 'Reload cash out', payout: true, quantity: '1', gross: '-500', subtotal: '-500' }),
  item({ name: 'Rice Samba 5kg', unitPrice: '3080', netUnitPrice: '3080', gross: '3080', subtotal: '3080' }),
]

/** Kinds × fixtures the style tests render. */
export const SAMPLE_NAMES = ['busy', 'payout', 'credit', 'refund', 'money', 'debt', 'warranty', 'test'] as const
export type SampleName = (typeof SAMPLE_NAMES)[number]

export function sample(name: SampleName, width: '80' | '58' = '80'): SlipData {
  const base = { width, dots: width === '58' ? 384 : 576, columns: width === '58' ? 32 : 48, currency: 'Rs.', shop }
  switch (name) {
    case 'busy':
      return {
        ...base,
        kind: 'sale',
        sale: sale({
          items: busyItems,
          subtotal: '6765',
          itemSavings: '345',
          given: '500',
          total: '6265',
          youSaved: '345',
          payments: [
            { method: 'Cash', amount: '5000' },
            { method: 'Card', amount: '1500' },
          ],
          change: '234.50',
          changeKept: '0.50',
        }),
      }
    case 'payout':
      return {
        ...base,
        kind: 'sale',
        sale: sale({
          receiptNo: 'S-00232',
          items: [item({ name: 'Bank cash out', payout: true, gross: '-2000', subtotal: '-2000' }), item({ name: 'Biscuits', unitPrice: '300', netUnitPrice: '300', gross: '300', subtotal: '300' })],
          subtotal: '300',
          given: '2000',
          total: '-1700',
          payments: [{ method: 'Cash', amount: '0' }],
          change: '1700',
        }),
      }
    case 'credit':
      return {
        ...base,
        kind: 'sale',
        sale: sale({
          receiptNo: 'S-00233',
          customer: 'Nimal Perera',
          items: [item({ name: 'Rice Samba 5kg', unitPrice: '2580', netUnitPrice: '2580', gross: '2580', subtotal: '2580' })],
          subtotal: '2580',
          total: '2580',
          payments: [{ method: 'Cash', amount: '1000' }],
          credit: true,
          fromAdvance: '200',
          due: '1380',
          balance: '4380',
          onAccount: '0.25',
        }),
      }
    case 'refund':
      return {
        ...base,
        kind: 'refund',
        refund: {
          receiptNo: 'S-00231',
          date: '2026-10-22 09:10',
          customer: 'Nimal Perera',
          reason: 'Wrong size',
          items: [{ name: 'Samsung Charger 25W', quantity: '1', unitAbbr: 'pc', refund: '2205' }],
          refund: '1205',
          creditReduction: '1000',
          remainingBalance: '3380',
        },
      }
    case 'money':
      return {
        ...base,
        kind: 'money',
        money: { title: 'Withdrawal', receiptNo: 'CR-000018', date: '2026-10-21 18:00', from: 'Till 1', to: 'Owner', party: 'Sunil', by: 'Kamal', amount: '15000', note: 'Weekly draw' },
      }
    case 'debt':
      return {
        ...base,
        kind: 'debt',
        debt: { receiptNo: 'DP-000002', date: '2026-10-21 11:05', customer: 'Nimal Perera', phone: '0771234567', amount: '2000', method: 'Cash', balanceBefore: '6380', balanceAfter: '4380', cashier: 'Kamal' },
      }
    case 'warranty':
      return {
        ...base,
        kind: 'warranty',
        warranty: { date: '2026-10-21', customer: 'Nimal Perera', product: 'Samsung Charger 25W', oldSerial: 'SC-88231', newSerial: 'SC-90412', warrantyUntil: '2027-10-21', warrantyLeft: '12 months left' },
      }
    case 'test':
      return { ...base, kind: 'test', test: { printed: '2026-10-21 14:32' } }
  }
}
