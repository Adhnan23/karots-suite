// The screen recordings web/docs-shots/clips.spec.ts makes (public/clips/<name>.webm).
// Phone-width ones end in -phone or are admin pages; the till ones are desktop.
export const CLIPS = [
  'dashboard-phone', 'sales-phone', 'whos-on', 'lowstock', 'reorder', 'credit-list', 'statement',
  'cashflow-phone', 'activity', 'till-close', 'till-lock', 'supplier-dues', 'profit', 'sell', 'backup',
  'requests', 'similar', 'search-local', 'hold',
  'pharmacy', 'repairs', 'recharge', 'documents', 'clearance', 'combos',
] as const
