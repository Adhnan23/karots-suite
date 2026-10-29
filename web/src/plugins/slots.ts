import type { LucideIcon } from 'lucide-react'
import type { ComponentType } from 'react'
import type { SaleLine, SupplierView, TillProduct } from '@/api/types.gen'
import type { CartLine, PayoutInput, SaleType, ServiceLineInput } from '@/features/till/cart/types'
import type { SlipKit, StyleKey } from '@/features/print/styles/kit'
import type { LabelKind } from '@/features/print/labels/kit'
import type { TokenDef } from '@/theme/palettes/tokens'

/** One member of a line group: a product, its qty in ONE set and a fixed per-unit discount. */
export type GroupLineInput = { productId: number; per: number; discount: string }

/** What a plugin till component may do to the sale in progress. */
export type TillApi = {
  addServiceLine(line: ServiceLineInput): void
  /** Money the shop hands the customer (a pass-through service product): the cart may then
   * total below zero and Pay gives it out as cash, netted with anything bought. */
  addPayout(line: PayoutInput): void
  /** Add lines that stay together in the cart (− / + scale them, removing one breaks the
   * group back to full price). take moves units off existing plain lines in the same step;
   * plugin lands on every line. Resolves to an error text, or null when added. */
  addGroup(group: { label: string; count: number; items: GroupLineInput[]; take?: { key: number; qty: number }[]; plugin?: Record<string, unknown> }): Promise<string | null>
  /** Service lines that belong together (a job and its add-ons), each priced as given: one
   * block in the cart and on the receipt under label, removed as a whole, no − / +. */
  addServiceGroup(group: { label: string; lines: ServiceLineInput[] }): void
  /** Put another product on a plain line, keeping its qty — or taking qty when given
   * (another unit) — and merging into a plain line of that product. Resolves to an
   * error text, or null when swapped. */
  swapLine(key: number, productId: number, qty?: number): Promise<string | null>
  saleType: SaleType
  /** The cart's lines as they are now (read-only). */
  lines: CartLine[]
  refreshDrawer(): void
  /** The sale's customer (null = walk-in) and setting it (e.g. a patient picked in a panel). */
  customerId: number | null
  setCustomer(id: number | null): void
  /** Add a product by id: units (default 1) of it, skipping the pack chooser; plugin data
   * keeps it its own line. Resolves to an error text, or null when added. */
  addProduct(productId: number, opts?: { units?: number; plugin?: Record<string, unknown>; discount?: { value: string; type: 'fixed' | 'percent' } }): Promise<string | null>
  /** Put a payment row on the sale (e.g. On account for a deposit already taken); the
   * cashier can still remove it at Pay. */
  addTender(method: string, amount: string, meta?: Record<string, string>): void
  /** A plugin's notes on the sale in progress (kept on hold / resume, cleared by New sale),
   * handed to tillAfterSale as ctx.meta. undefined removes the key. */
  /** Set a cart line's qty, as typing it on the line would (whole units round, lots spill). */
  setLineQty(key: number, qty: number): void
  saleMeta: Record<string, unknown>
  setSaleMeta(key: string, value: unknown): void
}

/** A strip above the cart's foot (e.g. "these items make a deal — Apply"); mounted on an
 * empty cart too (render nothing then). */
export type TillCartHint = { key: string; Hint: ComponentType<{ till: TillApi; lines: CartLine[] }> }
/** A small note across a plain (ungrouped) cart line, under its name and qty. */
export type TillLineHint = { key: string; Hint: ComponentType<{ till: TillApi; line: CartLine; lines: CartLine[] }> }

/** Offered inside the "sell more than the count shows?" prompt for that line (e.g.
 * an in-stock swap) — so the count isn't corrected up when there's a better way.
 * onDone closes the prompt (checkout doesn't go on: the cart changed). */
export type TillShortHint = { key: string; Hint: ComponentType<{ till: TillApi; line: CartLine; onDone: () => void }> }

/** A chip beside the group chips that opens a plugin menu in the catalogue area.
 * claims, when set, is asked before the till adds a picked product (search, card,
 * scan): true opens this Panel with that product instead of adding a line. */
export type TillMenuRoot = {
  key: string
  label: string
  emoji?: string
  claims?: (p: TillProduct) => boolean
  Panel: ComponentType<{ till: TillApi; onDone: () => void; product?: TillProduct }>
}

/** A button in the till's action row (beside Deposit / Withdraw) opening a panel.
 * noMatch, when set, is a link label also offered when a till search finds nothing;
 * the panel then gets that search as text. claimsText, when set, is asked first on
 * every Enter in the scan box (e.g. a scanned job ticket): true opens this panel with
 * that text instead of looking it up as a product. */
export type TillAction = {
  key: string
  label: string
  noMatch?: string
  claimsText?: (text: string) => boolean
  Panel: ComponentType<{ till: TillApi; onClose: () => void; text?: string }>
}

/** Extra counts in the Open/Close till dialogs (side by side with the cash count).
 * register(save) on mount; save runs around the drawer call — returning false
 * aborts a close (an open is best-effort). */
export type DrawerSection = {
  key: string
  /** Which dialogs it shows in (default both). */
  on?: ('open' | 'close')[]
  Section: ComponentType<{ which: 'open' | 'close'; register: (save: () => Promise<boolean>) => void }>
}

/** Detail picker for a plugin tender method listed in TillConfig.tenderMethods. */
export type TillTender = {
  value: string
  /** The tender's button icon (a wallet when none). */
  Icon?: LucideIcon
  Detail: ComponentType<{ meta: Record<string, string>; setMeta: (m: Record<string, string>) => void }>
}

/** A till tab (in the More menu) owned by a plugin, served at /till/x/<key>. */
export type TillTab = { key: string; label: string; Page: ComponentType }

/** An extra kind in the till's Receipts hub (e.g. a plugin's own job slips). The
 * Page owns its list and preview; the hub keeps the kind switch above it. */
export type TillReceiptKind = { key: string; label: string; Page: ComponentType }

/** A plugin slip's React template for Image mode, keyed by the `kind` its print path's
 * ?data=1 answers (e.g. "repairs.job"); `d` is that JSON. Draw it with `kit`'s parts
 * and it follows every receipt style (`style` = the style's key); `overrides` draw
 * the whole slip for a style. `label` + `sample` (a path answering ?data=1 with a
 * made-up slip, never printing) put it on Appearance's slip switcher. A kind with
 * no template prints its decoded Classic lines. */
/** A plugin's own look tokens (e.g. `--repairs-due`). The plugin sets their defaults in
 * its own CSS on `:root`, built from core tokens (`--repairs-due: var(--warning)`), so
 * every palette covers them; the palette editor lists them under Plugins and a
 * palette may override them like any token. */
export type PaletteTokens = { plugin: string; label: string; tokens: TokenDef[] }

export type SlipTemplate = {
  kind: string
  label: string
  sample?: string
  Template: ComponentType<{ d: unknown; kit: SlipKit; style: string }>
  overrides?: Partial<Record<StyleKey, ComponentType<{ d: unknown; kit: SlipKit; style: string }>>>
}

/** A plugin's input on a line being returned at the counter (e.g. how much float came
 * back). Render nothing for lines the plugin doesn't own; value goes to the return
 * request's line.plugin[key], read by the plugin's Go ReturnLineHook. */
export type ReturnLineField = {
  key: string
  Field: ComponentType<{ saleId: number; line: SaleLine; qty: number; value: unknown; onChange: (v: unknown) => void }>
}

/** A job in a supplier's workspace on the counter's Suppliers tab, beside Pay · Receive
 * stock · Order (e.g. buying reload float from a dealer); its Panel gets that supplier. */
export type SupplierJob = { key: string; label: string; Panel: ComponentType<{ supplier: SupplierView }> }

/** A card in the Reports hub's "More" group, linking to a plugin's own report page. */
export type ReportCard = { href: string; title: string; desc: string }

/** A section on Setup → Settings owned by a plugin. */
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
/** kind says what a new product with a sourceId is: a 'copy' (Duplicate — default) or a
 * 'loose' version of it (another unit, e.g. a 25 kg bag sold by the kg). */
export type ProductFormSection = {
  key: string
  Section: ComponentType<{ productId: number; sourceId: number; kind?: 'copy' | 'loose'; categoryId: number; where: 'form' | 'intake'; value: unknown; onChange: (v: unknown) => void }>
}

/** A helper under the Name field of the product form and intake's New item (e.g. identify a
 * product from what was typed). fill sets the name and/or category on the form; extra lands in
 * extras[key], sent as ProductRequest.extra[key] when the person saves. */
export type ProductAssist = {
  key: string
  Assist: ComponentType<{ name: string; barcode: string; where: 'form' | 'intake'; fill: (f: AssistFill) => void }>
}
export type AssistFill = { name?: string; categoryId?: number; extra?: unknown }

/** customerId and meta (the plugins' notes set with TillApi.setSaleMeta) as they were at completion. */
export type SaleHookCtx = { saleId: number; lines: CartLine[]; customerId: number | null; meta: Record<string, unknown> }

/** Filled by enabled plugins at the composition root (M7). Core renders whatever is here. */
export const slots = {
  apps: [] as AppEntry[],
  adminPages: [] as AdminPage[],
  tillMenu: [] as TillMenuRoot[],
  tillActions: [] as TillAction[],
  tillCartHints: [] as TillCartHint[],
  tillLineHints: [] as TillLineHint[],
  tillShortHints: [] as TillShortHint[],
  drawerSections: [] as DrawerSection[],
  tillTenders: [] as TillTender[],
  tillTabs: [] as TillTab[],
  tillReceiptKinds: [] as TillReceiptKind[],
  slipTemplates: [] as SlipTemplate[],
  /** A plugin's own label kinds (e.g. "pharmacy.dose") for Image mode: its styles,
   * the first being `classic`, drawn from the plugin's own data; print with
   * `printLabel` from '@/features/print/labels/print'. Appearance lists each kind. */
  labelTemplates: [] as LabelKind[],
  paletteTokens: [] as PaletteTokens[],
  returnLineFields: [] as ReturnLineField[],
  supplierJobs: [] as SupplierJob[],
  reportCards: [] as ReportCard[],
  settingsSections: [] as SettingsSection[],
  productFormSections: [] as ProductFormSection[],
  productAssists: [] as ProductAssist[],
  /** Return an error message to block checkout, or null. */
  tillBeforeCheckout: [] as ((lines: CartLine[]) => Promise<string | null>)[],
  /** Best-effort bookkeeping after a sale commits; a failure never undoes the sale. */
  tillAfterSale: [] as ((ctx: SaleHookCtx) => Promise<void>)[],
}
