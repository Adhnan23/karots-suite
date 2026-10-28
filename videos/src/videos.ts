import type { Focus, Scene, Video, Visual } from './types'

// Every video, as data. A short = the owner's problem, then the screens that
// make it go away. The promo = a hook, the five big ones, a quick montage.
// Lines (caption + voice) live in lines/<lang>.ts under the scene's id.

const problem: Visual = { kind: 'problem' }
const clip = (src: string, focus?: Focus, from?: number): Visual => ({ kind: 'clip', src, focus, from })
const shot = (src: string, focus?: Focus): Visual => ({ kind: 'shot', src, focus })

const S = {
  awayDashboard: { id: 'away-dashboard', visual: clip('dashboard-phone'), note: 'internet' },
  awaySales: { id: 'away-sales', visual: clip('sales-phone'), note: 'internet' },
  awayStaff: { id: 'away-staff', visual: clip('whos-on') },
  buyingLow: { id: 'buying-lowstock', visual: clip('lowstock') },
  buyingReorder: { id: 'buying-reorder', visual: clip('reorder') },
  creditList: { id: 'credit-list', visual: clip('credit-list') },
  creditStatement: { id: 'credit-statement', visual: clip('statement') },
  money: { id: 'money-cashflow', visual: clip('cashflow-phone') },
  staffActivity: { id: 'staff-activity', visual: clip('activity') },
  staffClose: { id: 'staff-close', visual: clip('till-close') },
  staffLock: { id: 'staff-lock', visual: clip('till-lock') },
  suppliersDues: { id: 'suppliers-dues', visual: clip('supplier-dues') },
  profitReport: { id: 'profit-report', visual: clip('profit') },
  safeSell: { id: 'safe-sell', visual: clip('sell') },
  safeBackup: { id: 'safe-backup', visual: clip('backup') },
  safeCloud: { id: 'safe-cloud', visual: clip('backup', undefined, 3), addon: true },
} satisfies Record<string, Scene>

const p = (id: string): Scene => ({ id, visual: problem })
const addon = (id: string, src: string): Scene => ({ id, visual: clip(src), addon: true })
const m = (id: string, visual: Visual, isAddon = false): Scene => ({ id: `m-${id}`, visual, addon: isAddon || undefined })

export const VIDEOS: Video[] = [
  {
    id: 'promo',
    scenes: [
      p('promo-hook'),
      S.awayDashboard, S.buyingReorder, S.creditList, S.money, S.staffActivity,
      m('suppliers', clip('supplier-dues')),
      m('profit', clip('profit')),
      m('requests', clip('requests')),
      m('similar', clip('similar')),
      m('search', clip('search-local')),
      m('hold', clip('hold')),
      m('labels', shot('printing/label-styles')),
      m('receipts', shot('printing/receipt-styles')),
      m('offline', clip('sell')),
      m('backup', clip('backup')),
      m('addons', shot('plugins/pharmacy'), true),
    ],
  },
  { id: 'away', scenes: [p('away-problem'), S.awayDashboard, S.awaySales, S.awayStaff] },
  { id: 'buying', scenes: [p('buying-problem'), S.buyingLow, S.buyingReorder] },
  { id: 'credit', scenes: [p('credit-problem'), S.creditList, S.creditStatement] },
  { id: 'staff', scenes: [p('staff-problem'), S.staffActivity, S.staffClose, S.staffLock] },
  { id: 'suppliers', scenes: [p('suppliers-problem'), S.suppliersDues] },
  { id: 'profit', scenes: [p('profit-problem'), S.profitReport] },
  { id: 'safe', scenes: [p('safe-problem'), S.safeSell, S.safeBackup, S.safeCloud] },
  {
    id: 'addons',
    scenes: [
      p('addons-problem'),
      addon('addons-pharmacy', 'pharmacy'),
      addon('addons-repairs', 'repairs'),
      addon('addons-recharge', 'recharge'),
      addon('addons-print', 'documents'),
      addon('addons-clearance', 'clearance'),
      addon('addons-combos', 'combos'),
    ],
  },
]

/** Every scene id any video uses, once. */
export const SCENE_IDS = [...new Set(VIDEOS.flatMap((v) => v.scenes.map((s) => s.id)))]
