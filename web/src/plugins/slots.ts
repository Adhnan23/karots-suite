import type { LucideIcon } from 'lucide-react'
import type { ComponentType } from 'react'
import type { CartLine, SaleType, ServiceLineInput } from '@/features/till/cart/types'

/** One member of a line group: a product, its qty in ONE set and a fixed per-unit discount. */
export type GroupLineInput = { productId: number; per: number; discount: string }

/** What a plugin till component may do to the sale in progress. */
export type TillApi = {
  addServiceLine(line: ServiceLineInput): void
  /** Add lines that stay together in the cart (− / + scale them, removing one breaks the
   * group back to full price). take moves units off existing plain lines in the same step;
   * plugin lands on every line. Resolves to an error text, or null when added. */
  addGroup(group: { label: string; count: number; items: GroupLineInput[]; take?: { key: number; qty: number }[]; plugin?: Record<string, unknown> }): Promise<string | null>
  /** Put another product on a plain line, keeping its qty (merges into a plain
   * line of that product). Resolves to an error text, or null when swapped. */
  swapLine(key: number, productId: number): Promise<string | null>
  saleType: SaleType
  refreshDrawer(): void
}

/** A strip above the cart's foot (e.g. "these items make a deal — Apply"). */
export type TillCartHint = { key: string; Hint: ComponentType<{ till: TillApi; lines: CartLine[] }> }
/** A small note under a plain (ungrouped) cart line's name. */
export type TillLineHint = { key: string; Hint: ComponentType<{ till: TillApi; line: CartLine; lines: CartLine[] }> }

/** A chip beside the group chips that opens a plugin menu in the catalogue area. */
export type TillMenuRoot = { key: string; label: string; emoji?: string; Panel: ComponentType<{ till: TillApi; onDone: () => void }> }

/** A button in the till's action row (beside Deposit / Withdraw) opening a panel.
 * noMatch, when set, is a link label also offered when a till search finds nothing;
 * the panel then gets that search as text. */
export type TillAction = {
  key: string
  label: string
  noMatch?: string
  Panel: ComponentType<{ till: TillApi; onClose: () => void; text?: string }>
}

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

/** A section on Setup → Settings owned by a plugin (karots-pos plugin.SettingsSections). */
export type SettingsSection = { key: string; title: string; Section: ComponentType }

/** A tile on the dock's Apps shelf (also in Ctrl K and the phone's More). color is a
 * theme token expression such as 'var(--brand)', never a raw colour. */
export type AppEntry = { key: string; label: string; description: string; icon: LucideIcon; color: string; to: string }

/** A plugin's admin page, served at /admin/x/<key>/*; path is what follows the key
 * ('' on the page's own root), for the plugin's own sub-pages. */
export type AdminPage = { key: string; Page: ComponentType<{ path: string }> }

/** A plugin block on the admin product form and the intake New item form. value is
 * what the Section last passed to onChange, sent as ProductRequest.extra[key]
 * (checked by the plugin's Go ProductExtras before the save). productId 0 = a new
 * product; sourceId is the product being duplicated (0 otherwise). */
export type ProductFormSection = {
  key: string
  Section: ComponentType<{ productId: number; sourceId: number; categoryId: number; where: 'form' | 'intake'; value: unknown; onChange: (v: unknown) => void }>
}

export type SaleHookCtx = { saleId: number; lines: CartLine[] }

/** Filled by enabled plugins at the composition root (M7). Core renders whatever is here. */
export const slots = {
  apps: [] as AppEntry[],
  adminPages: [] as AdminPage[],
  tillMenu: [] as TillMenuRoot[],
  tillActions: [] as TillAction[],
  tillCartHints: [] as TillCartHint[],
  tillLineHints: [] as TillLineHint[],
  drawerSections: [] as DrawerSection[],
  tillTenders: [] as TillTender[],
  tillTabs: [] as TillTab[],
  tillReceiptKinds: [] as TillReceiptKind[],
  reportCards: [] as ReportCard[],
  settingsSections: [] as SettingsSection[],
  productFormSections: [] as ProductFormSection[],
  /** Return an error message to block checkout, or null. */
  tillBeforeCheckout: [] as ((lines: CartLine[]) => Promise<string | null>)[],
  /** Best-effort bookkeeping after a sale commits; a failure never undoes the sale. */
  tillAfterSale: [] as ((ctx: SaleHookCtx) => Promise<void>)[],
}
