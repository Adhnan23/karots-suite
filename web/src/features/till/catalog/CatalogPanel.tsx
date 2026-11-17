import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { ChevronLeft, LayoutGrid, RotateCcw, Search } from 'lucide-react'
import type { TillGroup, TillProduct } from '@/api/types.gen'
import { useDebounced } from '@/lib/useDebounced'
import { cn } from '@/lib/utils'
import { slots } from '@/plugins/slots'
import { configQuery, groupQuery, groupsQuery, priceOptionsQuery, quickPicksQuery, searchQuery } from '../api'
import { Kbd } from '../bits'
import { DrawerActions } from '../drawer/DrawerActions'
import { useTillApi, useTillUi } from '../TillUi'
import { CHIP, PANEL } from '../ui'
import { GroupCard } from './GroupCard'
import { lotNote, ProductTile } from './ProductTile'
import { ProductRow } from './ProductRow'
import { useSellLayout } from '../layout/useSellLayout'
import { useAddProduct } from './useAddProduct'
import { useEntry } from './useEntry'

/** The product grid per card size (Appearance → Sell page layout); List is one column of rows. */
const GRID = {
  L: 'grid-cols-[repeat(auto-fill,minmax(clamp(168px,11vw,230px),1fr))]',
  M: 'grid-cols-[repeat(auto-fill,minmax(170px,1fr))]',
  S: 'grid-cols-[repeat(auto-fill,minmax(140px,1fr))] gap-2',
  T: 'grid-cols-[repeat(auto-fill,minmax(170px,1fr))]', // groups stay cards; the products' list spans the row
} as const

/** What the products section shows: search results, the open group or the frequently
 * sold, and the ↑ ↓ pick. Search only (strip) has no list until something is typed. */
export function useCatalog(strip = false) {
  const cfg = useSuspenseQuery(configQuery).data
  const saved = useSellLayout().layout?.card ?? 'L'
  const card = (saved in GRID ? saved : 'L') as keyof typeof GRID
  const ui = useTillUi()
  const q = useDebounced(ui.search, 300)
  const searching = ui.search.trim() !== '' && q.trim() !== ''
  const current = ui.stack.at(-1)
  const group = useQuery({ ...groupQuery(current?.id ?? 0), enabled: !!current && !searching })
  const results = useQuery({ ...searchQuery(q, true), enabled: searching })
  const picks = useQuery(quickPicksQuery).data ?? []

  let title = 'Frequently sold'
  let products: TillProduct[] = strip ? [] : picks
  let children: TillGroup[] = []
  if (searching) {
    title = `Results for “${q}”`
    products = results.data?.items ?? []
  } else if (current) {
    title = ui.stack.map((g) => g.name).join(' / ')
    products = group.data?.products ?? []
    children = group.data?.children ?? []
  }
  // ↑ ↓ in the search box pick a tile, Enter adds it; typing starts over. A scanner never presses arrows.
  const [hi, setHi] = useState(-1)
  useEffect(() => setHi(-1), [ui.search, q, current, ui.menu])
  const picked = hi >= 0 && ui.search === q ? products[hi] : undefined
  return { cfg, card, ui, q, searching, current, title, products, children, hi, setHi, picked }
}

type Catalog = ReturnType<typeof useCatalog>

/** The search box with Quick item and Menu (or, search only, Browse: the groups' pop-up). */
export function SearchRow({ cat, strip = false }: { cat: Catalog; strip?: boolean }) {
  const { ui, products, picked, setHi } = cat
  const add = useAddProduct()
  const submit = useEntry()
  return (
    <div className={cn(PANEL, 'flex h-[52px] shrink-0 items-center gap-3 px-4')}>
      <Search size={18} className="text-muted-foreground" />
      <input id="till-search" aria-label="Scan a barcode or search products" placeholder="Scan a barcode or search products"
        autoComplete="off" autoFocus value={ui.search} onChange={(e) => ui.setSearch(e.target.value)}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
            e.preventDefault()
            const last = products.length - 1
            setHi((i) => (e.key === 'ArrowDown' ? Math.min(i + 1, last) : Math.max(i - 1, 0)))
          } else if (e.key === 'Enter') {
            e.preventDefault()
            if (picked) {
              ui.setSearch('')
              add(picked)
            } else void submit(ui.search)
          }
        }}
        className="h-full min-w-0 flex-1 bg-transparent text-till-lead outline-none placeholder:text-faint" />
      {/* A narrow products column (Sell page layout) keeps the search box: the buttons drop their words. */}
      <Kbd className="@max-[440px]:hidden">F2</Kbd>
      <button type="button" onClick={() => ui.open({ kind: 'quickItem', barcode: '', name: '' })}
        title="Sell an item that isn't in the catalogue yet" aria-label="+ Quick item" className={CHIP}>
        +<span className="@max-[440px]:hidden"> Quick item</span>
      </button>
      {strip ? (
        <button type="button" onClick={() => { ui.reset(); ui.setBrowse(true) }} title="Groups, frequently sold and menus" aria-label="Browse" className={CHIP}>
          <LayoutGrid size={14} /><span className="@max-[440px]:hidden"> Browse</span> <Kbd className="@max-[440px]:hidden">F1</Kbd>
        </button>
      ) : (
        <button type="button" onClick={ui.reset} title="Back to the menu — the cart stays" aria-label="Menu" className={CHIP}>
          <RotateCcw size={14} /><span className="@max-[440px]:hidden"> Menu</span> <Kbd className="@max-[440px]:hidden">F1</Kbd>
        </button>
      )}
    </div>
  )
}

/** The products as cards or a list, at the saved card size. */
export function ProductList({ cat, rows = false }: { cat: Catalog; rows?: boolean }) {
  const { cfg, card, ui, products, hi, picked } = cat
  const opts = useQuery(priceOptionsQuery).data
  const add = useAddProduct()
  if (rows || card === 'T')
    return products.length > 0 ? (
      <div className={cn(PANEL, 'col-span-full overflow-hidden')}>
        {products.map((p, i) => (
          <ProductRow key={p.id} active={i === hi && p === picked} p={p} sym={cfg.symbol} note={lotNote(opts, p.id)} onAdd={() => add(p)}
            onInfo={() => ui.open({ kind: 'info', product: p })} />
        ))}
      </div>
    ) : null
  return products.map((p, i) => (
    <ProductTile key={p.id} size={card} active={i === hi && p === picked} p={p} sym={cfg.symbol} note={lotNote(opts, p.id)} onAdd={() => add(p)}
      onInfo={() => ui.open({ kind: 'info', product: p })} />
  ))
}

/** Groups and plugin menus, then the open one: what the products column shows under the search box. */
export function Browser({ cat, popup = false }: { cat: Catalog; popup?: boolean }) {
  const { card, ui, searching, current, title, products, children } = cat
  const till = useTillApi()
  const top = useQuery(groupsQuery).data ?? []
  const menuRoot = slots.tillMenu.find((m) => m.key === ui.menu)
  const openTop = (g: TillGroup) => {
    ui.setSearch('')
    ui.setMenu(null)
    ui.setStack([g])
  }
  return (
    <>
      <div className="flex flex-wrap gap-2" role="toolbar" aria-label="Groups">
        <button type="button" className={CHIP} aria-pressed={!current && !ui.menu && !searching} onClick={popup ? () => { ui.setStack([]); ui.setMenu(null) } : ui.reset}>Frequently sold</button>
        {slots.tillMenu.map((m) => (
          <button key={m.key} type="button" className={CHIP} aria-pressed={ui.menu === m.key}
            onClick={() => { ui.setSearch(''); ui.setStack([]); ui.setMenu(m.key) }}>
            {m.emoji} {m.label}
          </button>
        ))}
        {top.map((g) => (
          <button key={g.id} type="button" className={CHIP} aria-pressed={ui.stack[0]?.id === g.id && !searching} onClick={() => openTop(g)}>
            {g.emoji && <span>{g.emoji}</span>}{g.name}
          </button>
        ))}
      </div>

      {menuRoot ? (
        <div className={cn(PANEL, 'min-h-0 flex-1 overflow-auto p-4')}>
          <menuRoot.Panel key={ui.menuProduct?.id ?? 0} till={till} product={ui.menuProduct ?? undefined} onDone={() => ui.setMenu(null)} />
        </div>
      ) : (
        <>
          <div className="flex items-center justify-between text-till-sm font-semibold text-faint">
            <span className="flex items-center gap-2">
              {current && !searching && (
                <button type="button" className={CHIP} onClick={() => ui.setStack(ui.stack.slice(0, -1))}><ChevronLeft size={14} /> Back</button>
              )}
              {title}
            </span>
            <span className="flex items-center gap-1.5">
              {products.length > 1 && !popup && <><Kbd>↑ ↓</Kbd> Enter ·</>}
              {products.length} items
            </span>
          </div>
          <div className={cn('grid min-h-0 flex-1 auto-rows-min gap-2.5 overflow-auto p-0.5', GRID[card])}>
            {!searching && children.map((g) => <GroupCard key={`g${g.id}`} group={g} onOpen={() => ui.setStack([...ui.stack, g])} />)}
            <ProductList cat={cat} />
            {!products.length && !(children.length && !searching) && (
              <p className="col-span-full py-10 text-center text-faint">
                {searching ? 'No product matches. Press Enter to add it as a quick item.' : current ? 'This group is empty.' : 'Frequently sold items appear here once the shop has sales.'}
              </p>
            )}
          </div>
        </>
      )}
    </>
  )
}

export function CatalogPanel() {
  const cat = useCatalog()
  return (
    <>
      <SearchRow cat={cat} />
      <Browser cat={cat} />
      <DrawerActions />
    </>
  )
}
