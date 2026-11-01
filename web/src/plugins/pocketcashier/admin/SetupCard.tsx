import { useQuery } from '@tanstack/react-query'
import QRCode from 'qrcode'
import { useEffect, useState } from 'react'
import { SURFACE } from '@/components/admin/PageHead'
import { cn } from '@/lib/utils'
import { addressesQuery } from './api'

/** Where a new phone starts: the setup page's address as a QR for its camera
 * (plain HTTP on the shop wifi; it shows that phone's own steps). */
export function SetupCard() {
  const a = useQuery(addressesQuery).data
  const url = a?.setup[0] ?? ''
  const [qr, setQr] = useState('')
  useEffect(() => { if (url) void QRCode.toDataURL(url, { margin: 1, width: 180 }).then(setQr, () => setQr('')) }, [url])
  return (
    <section aria-label="Set up a phone" className={cn(SURFACE, 'flex flex-col items-center gap-3 p-4 text-center sm:flex-row sm:text-left')}>
      {qr ? <img src={qr} alt={`QR code for ${url}`} className="size-[150px] shrink-0 rounded-[var(--radius-sm)] bg-paper p-1.5" /> : <div className="size-[150px] shrink-0 rounded-[var(--radius-sm)] bg-muted" />}
      <div className="min-w-0">
        <h2 className="font-display text-[16px] font-bold">Set up a phone</h2>
        <p className="mt-1 text-[13.5px] text-muted-foreground">On the shop wifi, point the phone’s camera at this code. The page shows the exact steps for that phone, then its code comes up here to approve.</p>
        {a && !a.setup.length && <p className="mt-2 text-[13px] font-semibold text-warning">This PC isn’t on a network — connect it to the shop wifi first.</p>}
        {a?.setup.map((u) => <p key={u} className="mt-1 break-all font-mono text-[12.5px]">{u}</p>)}
      </div>
    </section>
  )
}
