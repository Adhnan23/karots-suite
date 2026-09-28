import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { api } from '@/api/client'
import type { CustomerCreated } from '@/api/types.gen'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { cn } from '@/lib/utils'
import { customersQuery, errMsg } from '../api'
import { useTill } from '../TillProvider'
import { useTillUi } from '../TillUi'
import { FIELD } from '../ui'

export function AddCustomer() {
  const ui = useTillUi()
  const { dispatch } = useTill()
  const qc = useQueryClient()
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [limit, setLimit] = useState('')
  async function submit() {
    if (!name.trim()) return void toast.error('Enter a customer name')
    if (!phone.trim()) return void toast.error('Enter a phone number') // the key that stops duplicate credit accounts
    try {
      const r = await api<CustomerCreated>('/till/customers', { json: { name: name.trim(), phone: phone.trim(), creditLimit: limit || '0' } })
      await qc.invalidateQueries({ queryKey: customersQuery.queryKey })
      dispatch({ type: 'customer', id: r.customer.id })
      toast.success(r.existed ? 'Customer already exists — using them' : 'Customer added')
      ui.close()
    } catch (e) {
      toast.error(errMsg(e))
    }
  }
  return (
    <Dialog open onOpenChange={(o) => !o && ui.close()}>
      <DialogContent className="sm:max-w-[420px]">
        <DialogHeader>
          <DialogTitle>Add customer</DialogTitle>
          <DialogDescription>Phone is required — it stops the same person getting two accounts.</DialogDescription>
        </DialogHeader>
        <form className="flex flex-col gap-3" autoComplete="off" onSubmit={(e) => { e.preventDefault(); void submit() }}>
          <input autoFocus aria-label="Customer name" placeholder="Name" value={name} onChange={(e) => setName(e.target.value)} className={FIELD} />
          <input aria-label="Customer phone" placeholder="Phone" inputMode="tel" value={phone} onChange={(e) => setPhone(e.target.value)} className={FIELD} />
          <input aria-label="Credit limit" placeholder="Credit limit (optional)" inputMode="decimal" value={limit}
            onChange={(e) => setLimit(e.target.value.replace(/[^\d.]/g, ''))} className={cn(FIELD, 'tabular-nums')} />
          <DialogFooter>
            <Button type="button" variant="ghost" onClick={ui.close}>Cancel</Button>
            <Button type="submit">Add customer</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
