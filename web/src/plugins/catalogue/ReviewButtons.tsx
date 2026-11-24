import { useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { api } from '@/api/client'
import { ConfirmButton } from '@/components/admin/ConfirmButton'
import { errMsg } from '@/features/till/api'
import type { ReviewAction } from '../slots'
import type { ComponentProps } from 'react'

/** On Items to review, for a product the catalogue changed (CTL-9): Use the catalogue's (the name and fields; a
 * unit is changed by hand) or Keep mine. A plain pick has no buttons of its own: Looks fine reviews it. */
export function ReviewButtons({ productId, name, notes }: ComponentProps<ReviewAction['Action']>) {
  const qc = useQueryClient()
  const change = notes.find((n) => n.startsWith('The catalogue changed'))
  if (!change) return null
  const unit = change.includes('Unit ')
  async function act(how: 'use' | 'keep') {
    try {
      await api(`/x/catalogue/reviews/${productId}/${how}`, { method: 'POST' })
      toast.success(how === 'use' ? `${name} now has the catalogue's details` : `${name} kept as it is`)
      void qc.invalidateQueries({ queryKey: ['admin'] })
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <>
      <ConfirmButton plain label="Keep mine" className="h-9 px-3" title={`Keep ${name} as it is?`} confirm="Keep mine"
        body="The product stays as it is, and this change isn't asked again." onConfirm={() => act('keep')} />
      <ConfirmButton plain label="Use the catalogue's" className="h-9 px-3" title={`Use the catalogue's details for ${name}?`} confirm="Use the catalogue's"
        body={`${change.replace('The catalogue changed: ', '')}.${unit ? ' The unit isn’t changed here (stock is counted in it): change it on the form if it’s wrong.' : ''}`} onConfirm={() => act('use')} />
    </>
  )
}
