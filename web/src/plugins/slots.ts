import type { ComponentType } from 'react'
import type { CartLine, ServiceLineInput } from '@/features/till/cart/types'

/** What a plugin till component may do to the sale in progress. */
export type TillApi = {
  addServiceLine(line: ServiceLineInput): void
  refreshDrawer(): void
}

/** A chip beside the group chips that opens a plugin menu in the catalogue area. */
export type TillMenuRoot = { key: string; label: string; emoji?: string; Panel: ComponentType<{ till: TillApi; onDone: () => void }> }

/** A button in the till's action row (beside Deposit / Withdraw) opening a panel. */
export type TillAction = { key: string; label: string; Panel: ComponentType<{ till: TillApi; onClose: () => void }> }

/** Extra counts in the Open/Close till dialogs (side by side with the cash count).
 * register(save) on mount; save runs around the drawer call — returning false
 * aborts a close (an open is best-effort). */
export type DrawerSection = {
  key: string
  Section: ComponentType<{ which: 'open' | 'close'; register: (save: () => Promise<boolean>) => void }>
}

/** Detail picker for a plugin tender method listed in TillConfig.tenderMethods. */
export type TillTender = {
  value: string
  Detail: ComponentType<{ meta: Record<string, string>; setMeta: (m: Record<string, string>) => void }>
}

/** A till tab (in the More menu) owned by a plugin, served at /till/x/<key>. */
export type TillTab = { key: string; label: string; Page: ComponentType }

/** An extra kind in the till's Receipts hub (e.g. a plugin's own job slips). The
 * Page owns its list and preview; the hub keeps the kind switch above it. */
export type TillReceiptKind = { key: string; label: string; Page: ComponentType }

/** A card in the Reports hub's "More" group, linking to a plugin's own report page. */
export type ReportCard = { href: string; title: string; desc: string }

export type SaleHookCtx = { saleId: number; lines: CartLine[] }

/** Filled by enabled plugins at the composition root (M7). Core renders whatever is here. */
export const slots = {
  tillMenu: [] as TillMenuRoot[],
  tillActions: [] as TillAction[],
  drawerSections: [] as DrawerSection[],
  tillTenders: [] as TillTender[],
  tillTabs: [] as TillTab[],
  tillReceiptKinds: [] as TillReceiptKind[],
  reportCards: [] as ReportCard[],
  /** Return an error message to block checkout, or null. */
  tillBeforeCheckout: [] as ((lines: CartLine[]) => Promise<string | null>)[],
  /** Best-effort bookkeeping after a sale commits; a failure never undoes the sale. */
  tillAfterSale: [] as ((ctx: SaleHookCtx) => Promise<void>)[],
}
