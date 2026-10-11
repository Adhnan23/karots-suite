import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Download, FileSpreadsheet, FileUp, Upload } from 'lucide-react'
import { useRef, useState } from 'react'
import { toast } from 'sonner'
import { api, apiUpload } from '@/api/client'
import type { EntityImportResult, SheetExtra } from '@/api/types.gen'
import { FormSheet } from '@/components/admin/FormSheet'
import { Button } from '@/components/ui/button'
import { DropdownMenu, DropdownMenuCheckboxItem, DropdownMenuContent, DropdownMenuItem, DropdownMenuLabel, DropdownMenuSeparator, DropdownMenuTrigger } from '@/components/ui/dropdown-menu'
import { errMsg } from '@/features/till/api'

type Entity = 'products' | 'customers' | 'suppliers'

const HELP: Record<Entity, string[]> = {
  products: [
    'A product with the same barcode (or the same name, when it has none) is updated; anything else is added.',
    'Only the columns your sheet fills change — a sheet of names and prices leaves costs, categories and suppliers alone.',
    'Category can be a path like “Grocery > Rice”. opening_qty adds stock only to a product that has none.',
  ],
  customers: [
    'The phone number matches an existing customer (spaces and dashes don’t matter); without a phone, the name does.',
    'opening_balance is what they already owed — it’s added to new customers only.',
  ],
  suppliers: [
    'The name matches an existing supplier; anything else is added.',
    'opening_balance is what you already owed them — it’s added to new suppliers only.',
  ],
}

/** Products' sheet options: skip disabled (on = core export as always) and the
 * plugin column groups to include. Remembered per device. */
type Opts = { skipDisabled: boolean; extras: string[] }
const OPTS_KEY = 'ks-sheets-products'
function loadOpts(): Opts {
  try {
    return { skipDisabled: true, extras: [], ...JSON.parse(localStorage.getItem(OPTS_KEY) ?? '{}') }
  } catch {
    return { skipDisabled: true, extras: [] }
  }
}

/** The query string the options add (only what differs from the core sheet). */
function optsQS(o: Opts, offered: SheetExtra[]) {
  const keys = o.extras.filter((k) => offered.some((x) => x.key === k))
  return `${o.skipDisabled ? '' : '&disabled=1'}${keys.length ? `&extras=${keys.join(',')}` : ''}`
}

/** Spreadsheets ▾ on a list page: export in three formats, a blank template,
 * and import from a sheet (karots-pos's import modal + export links). Products
 * also get Skip disabled and one toggle per plugin column group. */
export function SpreadsheetMenu({ entity }: { entity: Entity }) {
  const [open, setOpen] = useState(false)
  const base = `/api/v1/admin/${entity}`
  const products = entity === 'products'
  const offered = useQuery({ queryKey: ['admin', 'sheet-extras'], queryFn: () => api<SheetExtra[]>('/admin/products/sheet-extras'), enabled: products, staleTime: Infinity }).data ?? []
  const [opts, setOptsState] = useState(loadOpts)
  const setOpts = (o: Opts) => {
    setOptsState(o)
    try { localStorage.setItem(OPTS_KEY, JSON.stringify(o)) } catch { /* private window: per session only */ }
  }
  const qs = products ? optsQS(opts, offered) : ''
  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button variant="outline" className="h-10 gap-1.5 px-4"><FileSpreadsheet size={16} /> Sheets</Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-56">
          <DropdownMenuItem onSelect={() => setOpen(true)}><Upload size={15} /> Import from a sheet…</DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuLabel className="text-xs text-muted-foreground">Export every {entity.slice(0, -1)}</DropdownMenuLabel>
          {(['csv', 'xlsx', 'ods'] as const).map((f) => (
            <DropdownMenuItem key={f} asChild><a href={`${base}/export?format=${f}${qs}`} download><Download size={15} /> {f.toUpperCase()}</a></DropdownMenuItem>
          ))}
          {products && <>
            <DropdownMenuSeparator />
            <DropdownMenuCheckboxItem checked={opts.skipDisabled} onSelect={(e) => e.preventDefault()}
              onCheckedChange={(skipDisabled) => setOpts({ ...opts, skipDisabled })}>Skip disabled</DropdownMenuCheckboxItem>
            {offered.map((x) => (
              <DropdownMenuCheckboxItem key={x.key} checked={opts.extras.includes(x.key)} onSelect={(e) => e.preventDefault()}
                onCheckedChange={(on) => setOpts({ ...opts, extras: on ? [...opts.extras, x.key] : opts.extras.filter((k) => k !== x.key) })}>
                Include {x.label.toLowerCase()}
              </DropdownMenuCheckboxItem>
            ))}
          </>}
        </DropdownMenuContent>
      </DropdownMenu>
      {open && <ImportSheet entity={entity} qs={qs} included={offered.filter((x) => opts.extras.includes(x.key))} onClose={() => setOpen(false)} />}
    </>
  )
}

function ImportSheet({ entity, qs, included, onClose }: { entity: Entity; qs: string; included: SheetExtra[]; onClose: () => void }) {
  const input = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [byName, setByName] = useState(false)
  const [busy, setBusy] = useState(false)
  const [res, setRes] = useState<EntityImportResult | null>(null)
  const qc = useQueryClient()

  const run = async () => {
    if (!file) return void input.current?.click()
    setBusy(true)
    try {
      setRes(await apiUpload<EntityImportResult>(`/admin/${entity}/import${qs.replace('&', '?')}`, 'file', file, byName ? { matchByName: 'true' } : {}))
      setFile(null)
      void qc.invalidateQueries({ queryKey: ['admin'] })
    } catch (e) {
      toast.error(errMsg(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <FormSheet open onOpenChange={(o) => !o && onClose()} title={`Import ${entity}`} submit={file ? 'Import' : 'Choose a file'} busy={busy} onSubmit={() => void run()}>
      <ul className="grid list-disc gap-1 pl-5 text-[13px] text-muted-foreground">
        {HELP[entity].map((h) => <li key={h}>{h}</li>)}
      </ul>
      <div className="flex flex-wrap gap-2 text-sm">
        <span className="text-muted-foreground">Blank template:</span>
        {(['csv', 'xlsx', 'ods'] as const).map((f) => (
          <a key={f} className="font-semibold text-brand underline-offset-2 hover:underline" href={`/api/v1/admin/${entity}/import/template?format=${f}${qs}`} download>{f.toUpperCase()}</a>
        ))}
      </div>
      <button type="button" onClick={() => input.current?.click()}
        className="flex w-full flex-col items-center gap-1 rounded-[calc(var(--radius)-4px)] border-2 border-dashed border-border px-4 py-6 text-center text-sm text-muted-foreground">
        <FileUp size={22} className="text-brand" />
        <b className="break-all text-foreground">{file ? file.name : 'Choose a sheet'}</b>
        <span>CSV, Excel or LibreOffice</span>
      </button>
      <input ref={input} type="file" hidden accept=".csv,.xlsx,.ods" onChange={(e) => { setFile(e.target.files?.[0] ?? null); setRes(null); e.target.value = '' }} />
      {entity === 'products' && (
        <label className="flex items-start gap-2 text-[13px]">
          <input type="checkbox" className="mt-0.5 size-4 accent-[var(--brand)]" checked={byName} onChange={(e) => setByName(e.target.checked)} />
          <span><b>Also match by name</b> — a row whose barcode isn’t found updates the product with the same name and takes its barcode. Leave off unless merging a catalogue.</span>
        </label>
      )}
      {included.length > 0 && (
        <p className="text-[13px] text-muted-foreground">Also importing: <b className="text-foreground">{included.map((x) => x.label).join(', ')}</b> (turn off in Sheets ▾).</p>
      )}
      {res && <ImportReport r={res} />}
    </FormSheet>
  )
}

function ImportReport({ r }: { r: EntityImportResult }) {
  return (
    <section aria-label="Import result" className="grid gap-2 rounded-[calc(var(--radius)-4px)] border border-border bg-muted/40 p-3 text-sm">
      <div className="flex flex-wrap gap-x-4 gap-y-1 font-semibold">
        <span className="text-success">{r.created} added</span>
        <span>{r.updated} updated</span>
        {r.skipped > 0 && <span className="text-danger">{r.skipped} skipped</span>}
      </div>
      {r.errors.length > 0 && (
        <ul className="grid max-h-40 gap-0.5 overflow-auto text-[13px]">
          {r.errors.map((e) => <li key={e.line}><b className="tabular-nums">Line {e.line}</b> <span className="text-muted-foreground">{e.message}</span></li>)}
        </ul>
      )}
      {r.notes.length > 0 && (
        <ul className="grid max-h-28 gap-0.5 overflow-auto text-[12px] text-faint">{r.notes.map((n) => <li key={n}>{n}</li>)}</ul>
      )}
    </section>
  )
}
