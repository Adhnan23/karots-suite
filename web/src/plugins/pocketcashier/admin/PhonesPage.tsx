import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'
import { EmptyState, PageHead } from '@/components/admin/PageHead'
import { Button } from '@/components/ui/button'
import { errMsg } from '@/features/till/api'
import { approvePhone, blockPhone, phonesQuery, releasePhone, removePhone } from './api'
import { Approve } from './Approve'
import { PhoneList } from './PhoneList'
import { SendDetail } from './SendDetail'
import { Sends } from './Sends'
import { SetupCard } from './SetupCard'

/** /admin/x/pocketcashier: the phones; /sends: what came in; /sends/<id>: one send. */
export function PhonesPage({ path }: { path: string }) {
  const [, sub, id] = path.match(/^(sends)(?:\/(\d+))?/) ?? []
  if (sub && id) return <SendDetail id={Number(id)} />
  if (sub) return <Sends />
  return <Phones />
}

function Phones() {
  const rows = useQuery(phonesQuery).data
  const [approve, setApprove] = useState(false)
  const act = (fn: () => Promise<unknown>, done: string) => fn().then(() => void toast.success(done), (e: unknown) => void toast.error(errMsg(e)))
  return (
    <>
      <PageHead title="Phones" sub="Pocket Cashier phones: approve new ones, see who is on each, block a lost one."
        actions={(
          <>
            <Link to="/admin/x/$key/$" params={{ key: 'pocketcashier', _splat: 'sends' }} className="inline-flex h-10 items-center px-3 text-[13px] font-semibold text-muted-foreground hover:text-foreground">What came in</Link>
            <Button type="button" className="h-10 px-4 font-bold" onClick={() => setApprove(true)}>Approve by code</Button>
          </>
        )} />
      <div className="flex flex-col gap-3">
        <SetupCard />
        {rows && !rows.length && <EmptyState title="No phones yet">Set one up with the code above.</EmptyState>}
        {rows && (
          <PhoneList rows={rows} now={Date.now()}
            onApprove={(p) => void act(() => approvePhone(p.id), `${p.name} approved`)}
            onBlock={(p) => act(() => blockPhone(p.id), `${p.name} blocked`)}
            onRelease={(p) => act(() => releasePhone(p.id), `${p.userName} released`)}
            onRemove={(p) => act(() => removePhone(p.id), `${p.name} removed`)} />
        )}
      </div>
      {approve && <Approve onClose={() => setApprove(false)} />}
    </>
  )
}
