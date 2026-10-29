import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ChevronRight, CloudDownload, Copy, Database, Download, FileUp, HardDrive, RotateCcw, Upload } from 'lucide-react'
import { useRef, useState, type ReactNode } from 'react'
import { toast } from 'sonner'
import { apiUpload } from '@/api/client'
import type { BackupFileRow, SettingsImportResult } from '@/api/types.gen'
import { EmptyState, PageHead, SURFACE } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { stamp } from '@/features/money/format'
import { ExportMenu } from '@/features/reports/kit/ExportMenu'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { CopiesCard } from './CopiesCard'
import { dataQuery, fileSize } from './queries'
import { RestoreDialog, type RestoreSource } from './RestoreDialog'
import { UpdateCard } from './UpdateCard'

type Kind = 'full' | 'choose' | 'setup'

/** Setup → Data: backups out and back in, the
 * automatic copies and spreadsheets. Admin only. */
export function DataPage() {
  const data = useQuery(dataQuery).data
  const [restore, setRestore] = useState<RestoreSource | null>(null)
  const latest = data?.backups.find((b) => !b.beforeRestore)

  return (
    <>
      <PageHead title="Data" sub="Back up the shop, bring a backup back, and move lists in and out as spreadsheets." />
      <div className="relative mb-4 flex flex-wrap items-center justify-between gap-4 overflow-hidden rounded-[var(--radius)] bg-hero bg-[image:var(--hero-art)] p-5 text-hero-foreground md:p-6">
        <div className="min-w-0">
          <small className="text-[13px] font-semibold text-hero-muted">Latest backup on this computer</small>
          <div className="font-display text-2xl font-extrabold md:text-3xl">{data ? (latest ? stamp(latest.modifiedAt) : 'None yet') : '…'}</div>
          <p className="mt-1 text-[13px] text-hero-muted">
            {data && (data.autoBackups
              ? `Automatic every ${data.intervalHours} h, newest ${data.keep} kept, in ${data.folder}`
              : 'Automatic backups are off — set BACKUP_DIR on the server to turn them on.')}
          </p>
        </div>
        <Button asChild className="h-11 gap-2 bg-hero-foreground px-5 font-bold text-hero hover:bg-hero-foreground/90">
          <a href="/api/v1/admin/data/backup" download><Download size={17} /> Download a backup</a>
        </Button>
      </div>

      <div className="grid gap-3 lg:grid-cols-2">
        <Card title="Updates" icon={<CloudDownload size={18} />} className="lg:col-span-2">
          <UpdateCard />
        </Card>
        <Card title="Backup" icon={<HardDrive size={18} />}>
          <BackupChooser groups={data?.groups ?? []} />
        </Card>
        <Card title="Restore" icon={<RotateCcw size={18} />}>
          <RestorePicker onPick={(file) => setRestore({ file })} />
        </Card>
        <Card title="Automatic backups" icon={<HardDrive size={18} />} className="lg:col-span-2">
          {data && data.backups.length === 0
            ? <EmptyState title="No backups in the folder">{data.autoBackups ? 'The first one is written soon after the server starts.' : 'A copy is still saved here before every restore.'}</EmptyState>
            : <BackupList rows={data?.backups ?? []} onRestore={(name) => setRestore({ name })} />}
        </Card>
        <Card title="Backup copies" icon={<Copy size={18} />} className="lg:col-span-2">
          <CopiesCard onRestore={(name) => setRestore({ name })} />
        </Card>
        <Card title="Spreadsheets" icon={<FileUp size={18} />}>
          <Sheets />
        </Card>
        {data && <Card title="Where the shop is kept" icon={<Database size={18} />} className="lg:col-span-2"><Where dir={data.fileDataDir} /></Card>}
      </div>
      {restore && <RestoreDialog source={restore} onClose={() => setRestore(null)} />}
    </>
  )
}

/** File mode (the shop's own PostgreSQL in a folder on this PC) or server mode. */
function Where({ dir }: { dir: string }) {
  if (!dir) return <p className="text-sm text-muted-foreground">Server mode: the shop runs on a PostgreSQL server (DATABASE_URL in the .env).</p>
  return (
    <div className="space-y-2.5 text-sm">
      <p>File mode: this PC keeps the whole shop in <b className="break-all font-mono">{dir}</b>.</p>
      <p className="text-muted-foreground">The automatic backups sit on the same disk. Copy them to a USB stick or a cloud folder now and then, so a broken disk doesn&apos;t take them too.</p>
      <details className="text-muted-foreground">
        <summary className="cursor-pointer font-semibold text-foreground">Moving to a server</summary>
        <ol className="mt-1.5 list-decimal space-y-1 pl-5">
          <li>Download a backup (above).</li>
          <li>Install PostgreSQL on the server.</li>
          <li>Put <span className="font-mono">DATABASE_URL=…</span> in the .env next to this program.</li>
          <li>Start the program again, then restore the backup here.</li>
        </ol>
      </details>
    </div>
  )
}

function Card({ title, icon, children, className }: { title: string; icon: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section aria-label={title} className={cn(SURFACE, 'min-w-0 p-4 md:p-5', className)}>
      <h2 className="mb-3 flex items-center gap-2 font-display text-lg font-bold"><span className="text-brand">{icon}</span>{title}</h2>
      {children}
    </section>
  )
}

const KINDS: { kind: Kind; title: string; hint: string }[] = [
  { kind: 'full', title: 'Everything', hint: 'The whole shop — use this to move to a new computer.' },
  { kind: 'choose', title: 'Choose what to include', hint: 'Leave out some add-ons’ records.' },
  { kind: 'setup', title: 'Catalogue & setup only', hint: 'Products, contacts, users and settings — no sales or money, balances at zero. Starts a new shop.' },
]

function BackupChooser({ groups }: { groups: { key: string; name: string }[] }) {
  const [kind, setKind] = useState<Kind>('full')
  const [keep, setKeep] = useState<string[] | null>(null)
  const kept = keep ?? groups.map((g) => g.key)
  const qs = kind === 'setup' ? '?setupOnly=1' : kind === 'choose' ? `?${kept.length ? kept.map((k) => `groups=${encodeURIComponent(k)}`).join('&') : 'groups='}` : ''
  return (
    <div className="grid gap-2">
      <div role="radiogroup" aria-label="What to back up" className="grid gap-2">
        {KINDS.filter((k) => k.kind !== 'choose' || groups.length > 0).map((k) => (
          <button key={k.kind} type="button" role="radio" aria-checked={kind === k.kind} onClick={() => setKind(k.kind)}
            className="rounded-[calc(var(--radius)-4px)] border border-border bg-background/40 p-3 text-left aria-checked:border-selected aria-checked:bg-selected aria-checked:text-selected-foreground">
            <b className="block text-sm">{k.title}</b>
            <span className="text-[13px] opacity-80">{k.hint}</span>
          </button>
        ))}
      </div>
      {kind === 'choose' && (
        <div className="grid gap-1.5 px-1">
          {groups.map((g) => (
            <label key={g.key} className="flex items-center gap-2 text-sm">
              <input type="checkbox" className="size-4 accent-[var(--brand)]" checked={kept.includes(g.key)}
                onChange={(e) => setKeep(e.target.checked ? [...kept, g.key] : kept.filter((k) => k !== g.key))} />
              {g.name}
            </label>
          ))}
        </div>
      )}
      <Button asChild variant="outline" className="mt-1 h-10 gap-1.5 justify-self-start px-4 font-semibold">
        <a href={`/api/v1/admin/data/backup${qs}`} download><Download size={16} /> Download</a>
      </Button>
    </div>
  )
}

function RestorePicker({ onPick }: { onPick: (f: File) => void }) {
  const input = useRef<HTMLInputElement>(null)
  const [over, setOver] = useState(false)
  const take = (f?: File) => {
    if (!f) return
    if (!f.name.endsWith('.gz')) return void toast.error('That isn’t a backup file — pick a .json.gz')
    onPick(f)
  }
  return (
    <>
      <p className="mb-2.5 text-sm text-muted-foreground">Bring a backup back — from this shop or from Karots POS. It replaces everything here; a copy of what's here is saved first.</p>
      <button type="button" onClick={() => input.current?.click()}
        onDragOver={(e) => { e.preventDefault(); setOver(true) }} onDragLeave={() => setOver(false)}
        onDrop={(e) => { e.preventDefault(); setOver(false); take(e.dataTransfer.files[0]) }}
        className={cn('flex w-full flex-col items-center gap-1 rounded-[calc(var(--radius)-4px)] border-2 border-dashed border-border px-4 py-7 text-center text-sm text-muted-foreground',
          over && 'border-selected bg-selected/10')}>
        <Upload size={22} className="text-brand" />
        <b className="text-foreground">Choose a backup file</b>
        <span>or drop it here (.json.gz)</span>
      </button>
      <input ref={input} type="file" hidden accept=".gz,application/gzip" onChange={(e) => { const f = e.target.files?.[0]; e.target.value = ''; take(f) }} />
    </>
  )
}

const SHOWN = 8

function BackupList({ rows, onRestore }: { rows: BackupFileRow[]; onRestore: (name: string) => void }) {
  const [all, setAll] = useState(false)
  return (
    <>
    <ul aria-label="Backup files" className="divide-y divide-border">
      {(all ? rows : rows.slice(0, SHOWN)).map((b) => (
        <li key={b.name} className="flex flex-wrap items-center gap-x-3 gap-y-1.5 py-2.5 first:pt-0 last:pb-0">
          <div className="min-w-0 basis-full sm:basis-0 sm:flex-1">
            <b className="block truncate text-sm">{stamp(b.modifiedAt)}{b.beforeRestore && <span className="ml-2 rounded-pill bg-warning/15 px-2 py-0.5 text-[11px] font-bold text-warning">Before restore</span>}</b>
            <span className="block truncate text-xs text-faint">{b.name} · {fileSize(b.size)}</span>
          </div>
          <div className="flex gap-1.5">
            <Button asChild variant="outline" className="h-9 gap-1.5 px-3"><a href={`/api/v1/admin/data/backups/${encodeURIComponent(b.name)}`} download><Download size={15} /> Download</a></Button>
            <Button variant="outline" className="h-9 gap-1.5 px-3" onClick={() => onRestore(b.name)}><RotateCcw size={15} /> Restore</Button>
          </div>
        </li>
      ))}
    </ul>
    {!all && rows.length > SHOWN && (
      <Button variant="ghost" className="mt-2 h-9 px-3 font-semibold" onClick={() => setAll(true)}>Show all {rows.length}</Button>
    )}
    </>
  )
}

const LISTS = [
  { to: '/admin/products', label: 'Products', hint: 'Import a price list, export the catalogue' },
  { to: '/admin/customers', label: 'Customers', hint: 'With credit limits and what they owe' },
  { to: '/admin/suppliers', label: 'Suppliers', hint: 'With credit days and what you owe' },
] as const

function Sheets() {
  const input = useRef<HTMLInputElement>(null)
  const qc = useQueryClient()
  const [res, setRes] = useState<SettingsImportResult | null>(null)
  const upload = async (f: File) => {
    try {
      const r = await apiUpload<SettingsImportResult>('/admin/data/settings/import', 'file', f)
      setRes(r)
      toast.success(`${r.applied} setting${r.applied === 1 ? '' : 's'} imported`)
      void qc.invalidateQueries()
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <div className="grid gap-3">
      <ul className="grid gap-1.5">
        {LISTS.map((l) => (
          <li key={l.to}>
            <Link to={l.to} className="flex items-center gap-2 rounded-[calc(var(--radius)-4px)] border border-border bg-background/40 px-3 py-2.5 hover:border-selected">
              <span className="min-w-0 flex-1"><b className="block text-sm">{l.label}</b><span className="text-xs text-muted-foreground">{l.hint}</span></span>
              <ChevronRight size={16} className="shrink-0 text-faint" />
            </Link>
          </li>
        ))}
      </ul>
      <div className="rounded-[calc(var(--radius)-4px)] border border-border p-3">
        <b className="block text-sm">Settings</b>
        <p className="mb-2 text-xs text-muted-foreground">Copy one shop's receipt, tax and till rules to another. This shop's identity, printers and default locker stay as they are.</p>
        <div className="flex flex-wrap gap-2">
          <ExportMenu url="/api/v1/admin/data/settings/export" />
          <Button variant="outline" className="h-10 gap-1.5 px-4" onClick={() => input.current?.click()}><Upload size={16} /> Import</Button>
          <input ref={input} type="file" hidden accept=".csv,.xlsx,.ods" onChange={(e) => { const f = e.target.files?.[0]; e.target.value = ''; if (f) void upload(f) }} />
        </div>
        {res && (res.skipped.length > 0 || res.unknown.length > 0) && (
          <p className="mt-2 break-words text-xs text-muted-foreground">
            {res.skipped.length > 0 && <>Kept this shop's own: {res.skipped.join(', ')}. </>}
            {res.unknown.length > 0 && <>Not settings here: {res.unknown.join(', ')}.</>}
          </p>
        )}
      </div>
    </div>
  )
}
