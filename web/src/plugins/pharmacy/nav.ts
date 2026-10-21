import { useNavigate } from '@tanstack/react-router'

export const CHIP = 'rounded-pill border border-border bg-card px-3.5 py-2 text-[13px] font-semibold text-muted-foreground whitespace-nowrap aria-pressed:border-selected aria-pressed:bg-selected aria-pressed:text-selected-foreground'

/** Go to a pharmacy sub-page ('' = Patients, 'patients/3', 'controlled/12', …). */
export function useGo() {
  const go = useNavigate()
  return (path: string) => void go({ to: '/admin/x/$key/$', params: { key: 'pharmacy', _splat: path } })
}
