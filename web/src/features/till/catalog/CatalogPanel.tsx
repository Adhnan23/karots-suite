import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { ChevronLeft, RotateCcw, Search } from 'lucide-react'
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
import { useAddProduct } from './useAddProduct'
import { useEntry } from './useEntry'

export function CatalogPanel() {
  const cfg = useSuspenseQuery(configQuery).data
  const ui = useTillUi()
  const till = useTillApi()
  const q = useDebounced(ui.search, 300)
  const searching = ui.search.trim() !== '' && q.trim() !== ''
  const current = ui.stack.at(-1)
  const top = useQuery(groupsQuery).data ?? []
  const group = useQuery({ ...groupQuery(current?.id ?? 0), enabled: !!current && !searching })
  const results = useQuery({ ...searchQuery(q, true), enabled: searching })
  const picks = useQuery(quickPicksQuery).data ?? []
  const opts = useQuery(priceOptionsQuery).data
  const add = useAddProduct()
  const submit = useEntry()
  const menuRoot = slots.tillMenu.find((m) => m.key === ui.menu)

  let title = 'Frequently sold'
  let products: TillProduct[] = picks
  let children: TillGroup[] = []
  if (searching) {
    title = `Results for “${q}”`
    products = results.data?.items ?? []
  } else if (current) {
    title = ui.stack.map((g) => g.name).join(' / ')
    products = group.data?.products ?? []
    children = group.data?.children ?? []
  }
  const openTop = (g: TillGroup) => {
    ui.setSearch('')
    ui.setMenu(null)
    ui.setStack([g])
  }

  return (
    <>
      <div className={cn(PANEL, 'flex h-[52px] shrink-0 items-center gap-3 px-4')}>
        <Search size={18} className="text-muted-foreground" />
        <input id="till-search" aria-label="Scan a barcode or search products" placeholder="Scan a barcode or search products"
          autoComplete="off" autoFocus value={ui.search} onChange={(e) => ui.setSearch(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              void submit(ui.search)
            }
          }}
          className="h-full min-w-0 flex-1 bg-transparent text-[15px] outline-none placeholder:text-faint" />
        <Kbd>F2</Kbd>
        <button type="button" onClick={ui.reset} title="Back to the menu — the cart stays" className={CHIP}>
          <RotateCcw size={14} /> Menu <Kbd>F1</Kbd>
        </button>
      </div>

      <div className="flex flex-wrap gap-2" role="toolbar" aria-label="Groups">
        <button type="button" className={CHIP} aria-pressed={!current && !ui.menu && !searching} onClick={ui.reset}>Frequently sold</button>
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
          <div className="flex items-center justify-between text-[12px] font-semibold text-faint">
            <span className="flex items-center gap-2">
              {current && !searching && (
                <button type="button" className={CHIP} onClick={() => ui.setStack(ui.stack.slice(0, -1))}><ChevronLeft size={14} /> Back</button>
              )}
              {title}
            </span>
            <span>{products.length} items</span>
          </div>
          <div className="grid min-h-0 flex-1 auto-rows-min grid-cols-[repeat(auto-fill,minmax(176px,1fr))] gap-2.5 overflow-auto p-0.5">
            {!searching && children.map((g) => <GroupCard key={`g${g.id}`} group={g} onOpen={() => ui.setStack([...ui.stack, g])} />)}
            {products.map((p) => (
              <ProductTile key={p.id} p={p} sym={cfg.symbol} note={lotNote(opts, p.id)} onAdd={() => add(p)}
                onInfo={() => ui.open({ kind: 'info', product: p })} />
            ))}
            {!products.length && !(children.length && !searching) && (
              <p className="col-span-full py-10 text-center text-faint">
                {searching ? 'No product matches. Press Enter to add it as a quick item.' : current ? 'This group is empty.' : 'Frequently sold items appear here once the shop has sales.'}
              </p>
            )}
          </div>
        </>
      )}
      <DrawerActions />
    </>
  )
}
