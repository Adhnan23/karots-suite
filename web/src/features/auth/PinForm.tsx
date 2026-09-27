import { useState, type SubmitEvent } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

type Props = {
  onSubmit: (phone: string, pin: string) => Promise<void>
  submitLabel: string
  error?: string | null
}

/** Shared phone + PIN form for login, unlock and (with its own fields) change PIN. */
export function PinForm({ onSubmit, submitLabel, error }: Props) {
  const [phone, setPhone] = useState('')
  const [pin, setPin] = useState('')
  const [pending, setPending] = useState(false)

  const handleSubmit = async (e: SubmitEvent) => {
    e.preventDefault()
    setPending(true)
    try {
      await onSubmit(phone, pin)
    } finally {
      setPending(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      {error && (
        <p role="alert" className="text-sm text-destructive">
          {error}
        </p>
      )}
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="pin-form-phone">Phone</Label>
        <Input
          id="pin-form-phone"
          inputMode="tel"
          autoComplete="username"
          value={phone}
          onChange={(e) => setPhone(e.target.value)}
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label htmlFor="pin-form-pin">PIN</Label>
        <Input
          id="pin-form-pin"
          type="password"
          inputMode="numeric"
          autoComplete="current-password"
          minLength={4}
          maxLength={6}
          value={pin}
          onChange={(e) => setPin(e.target.value)}
        />
      </div>
      <Button type="submit" disabled={pending}>
        {submitLabel}
      </Button>
    </form>
  )
}
