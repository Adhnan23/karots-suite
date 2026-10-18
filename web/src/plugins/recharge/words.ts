import type { Float } from './types'

export const LABEL = 'flex flex-col gap-1.5 text-[13px] font-semibold text-muted-foreground'
export const QUICK = [50, 100, 200, 500, 1000]
/** A float as a Pick item: key d<id>, the balance beside its name. */
export const floatItem = (f: Float) => ({ key: `d${f.id}`, label: `${f.carrier} · ${f.label}`, balance: f.balance })
