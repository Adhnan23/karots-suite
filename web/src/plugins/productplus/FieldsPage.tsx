import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { ArrowDown, ArrowUp, Plus } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { DataList } from '@/components/admin/DataList'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { Tag } from '@/features/admin/products/ProductsPage'
import { errMsg } from '@/features/till/api'
import { cn } from '@/lib/utils'
import { fieldsQuery, moveField, setActive } from './api'
import { CHIP, FieldSheet } from './FieldSheet'
import { FillIn } from './FillIn'
import { type FieldDTO, TYPE_LABEL } from './types'

async function act(p: Promise<unknown>, done?: string) {
  try {
    await p
    if (done) toast.success(done)
  } catch (e) {
    toast.error(errMsg(e))
  }
}

function Flags({ f }: { f: FieldDTO }) {
  return (
    <span className="flex flex-wrap gap-1">
      {f.required && <Tag tone="warn">required</Tag>}
      {f.searchable && <Tag tone="brand">searchable</Tag>}
      {f.showAtTill && <Tag tone="muted">till ⓘ</Tag>}
      {f.printOnLabel && <Tag tone="muted">label</Tag>}
      {f.showInIntake && <Tag tone="muted">intake</Tag>}
      {!f.isActive && <Tag tone="muted">disabled</Tag>}
    </span>
  )
}

const TABS = [{ path: '', label: 'Fields' }, { path: 'fill', label: 'Fill in' }]

/** Apps → Product Plus: the custom fields with where
 * each applies, where it shows, and how many products are missing a required one;
 * Fill in (sub-path /fill) sets them across many products. */
export function FieldsPage({ path }: { path: string }) {
  const go = useNavigate()
  const fill = path === 'fill'
  const [disabled, setDisabled] = useState(false)
  const [editing, setEditing] = useState<FieldDTO | 'new' | null>(null)
  const data = useQuery(fieldsQuery(disabled)).data
  const rows = data ?? []
  return (
    <>
      <PageHead title="Product Plus" sub="Extra fields for your products — on the product form, the till, labels, search and sheets."
        actions={!fill && <Button className="h-10 gap-1.5 px-4 font-bold" onClick={() => setEditing('new')}><Plus size={16} /> Add field</Button>} />
      <div className="mb-3 flex flex-wrap gap-1.5">
        {TABS.map((t) => (
          <button key={t.path} type="button" aria-pressed={path === t.path} className={CHIP}
            onClick={() => void go({ to: '/admin/x/$key/$', params: { key: 'productplus', _splat: t.path } })}>{t.label}</button>
        ))}
        {!fill && <button type="button" aria-pressed={disabled} className={cn(CHIP, 'ml-auto')} onClick={() => setDisabled(!disabled)}>Show disabled</button>}
      </div>
      {fill ? <FillIn /> : <DataList label="Custom fields" rows={rows} rowKey={(f) => f.id} onRow={(f) => setEditing(f)}
        empty={<EmptyState title={!data ? 'Loading…' : 'No custom fields yet'}>{data && 'Add one — a model number, a grade, a size, an expiry date…'}</EmptyState>}
        columns={[
          { key: 'label', header: 'Field', cell: (f) => (
            <span className="flex flex-col gap-0.5">
              <b className={f.isActive ? '' : 'text-faint line-through'}>{f.label}</b>
              <small className="font-normal text-faint">{TYPE_LABEL[f.type]}{f.type === 'select' && ` · ${f.options.length} options`}{f.defaultValue && ` · default ${f.type === 'bool' ? 'Yes' : f.defaultValue}`}</small>
            </span>
          ) },
          { key: 'for', header: 'For', cell: (f) => <span className="text-muted-foreground">{f.scope.length ? f.scope.map((c) => c.name).join(', ') : 'All products'}</span> },
          { key: 'flags', header: 'Shows', cell: (f) => <Flags f={f} /> },
          { key: 'missing', header: 'Missing', interactive: true, cell: (f) => f.missing > 0 ? (
            <Link to="/admin/x/$key/$" params={{ key: 'productplus', _splat: 'fill' }} search={{ field: f.id, missing: 1 }}><Tag tone="warn">{`${f.missing} missing`}</Tag></Link>
          ) : <span className="text-faint">—</span> },
          { key: 'act', header: '', interactive: true, className: 'w-[1%] whitespace-nowrap', cell: (f) => (
            <span className="flex items-center gap-1">
              <Button variant="ghost" className="h-9 w-9 p-0" aria-label={`Move ${f.label} up`} onClick={() => void act(moveField(f.id, 'up'))}><ArrowUp size={16} /></Button>
              <Button variant="ghost" className="h-9 w-9 p-0" aria-label={`Move ${f.label} down`} onClick={() => void act(moveField(f.id, 'down'))}><ArrowDown size={16} /></Button>
              <Button variant="outline" className="h-9 px-3" onClick={() => void act(setActive(f.id, !f.isActive), f.isActive ? `${f.label} disabled` : `${f.label} enabled`)}>
                {f.isActive ? 'Disable' : 'Enable'}
              </Button>
            </span>
          ) },
        ]}
      />}
      {editing && <FieldSheet key={editing === 'new' ? 0 : editing.id} field={editing === 'new' ? null : editing} onClose={() => setEditing(null)} />}
    </>
  )
}
