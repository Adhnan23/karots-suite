import type { Job, Status } from './types'

export const STATUS: Record<Status, string> = {
  received: 'Received', in_progress: 'In progress', ready: 'Ready', collected: 'Collected', cancelled: 'Cancelled',
}

/** Status pill tone, theme tokens only. */
export const statusTone: Record<Status, string> = {
  received: 'bg-muted text-muted-foreground',
  in_progress: 'bg-[color-mix(in_srgb,var(--warning)_16%,transparent)] text-warning',
  ready: 'bg-[color-mix(in_srgb,var(--success)_16%,transparent)] text-success',
  collected: 'bg-brand-soft text-foreground',
  cancelled: 'bg-[color-mix(in_srgb,var(--danger)_12%,transparent)] text-danger',
}

/** Sri Lankan numbers for wa.me: 0771234567 → 94771234567. */
export function waNumber(phone: string): string {
  const d = phone.replace(/\D/g, '')
  return d.startsWith('0') ? `94${d.slice(1)}` : d
}

export const readyMessage = (j: Job, shop: string) =>
  `Hello ${j.customerName}, your ${j.deviceModel || 'device'} (${j.ticketNo}) is ready to collect${shop ? ` at ${shop}` : ''}. Please bring your slip. Thank you!`

export const device = (j: { deviceModel: string; repairType: string }) => [j.deviceModel, j.repairType].filter(Boolean).join(' · ') || '—'
