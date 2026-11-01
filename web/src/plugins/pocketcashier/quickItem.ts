import { api, ApiError } from '@/api/client'
import type { CustomerCreated, TillCustomer, TillProduct } from '@/api/types.gen'

export const PC_DOWN = "The PC isn't reachable — sell it at the counter"
export const SIGN_IN_AGAIN = 'Switch person and sign in again while the PC is up'

/** A live-only till call from the phone, its failures said in place: the PC
 * down, or a person who signed in offline (no session cookie on the PC → 401). */
export async function liveCall<T>(path: string, json: unknown, down = PC_DOWN, method?: string): Promise<{ ok: T } | { error: string }> {
  try {
    return { ok: await api<T>(path, { json, method }) }
  } catch (e) {
    if (e instanceof ApiError) return { error: e.status === 401 ? SIGN_IN_AGAIN : e.message }
    return { error: down }
  }
}

export type QuickItemInput = { name: string; price: string; qty: string; barcode: string; unitId: number }

/** The PC's quick item (created for admin review), from the phone. */
export async function createQuickItem(input: QuickItemInput): Promise<{ product: TillProduct } | { error: string }> {
  const r = await liveCall<TillProduct>('/till/quick-item', input)
  return 'ok' in r ? { product: r.ok } : r
}

export type NewCustomer = { name: string; phone: string; creditLimit: string }

/** A new customer, made on the PC from the phone (Pay). */
export async function createCustomer(input: NewCustomer): Promise<{ customer: TillCustomer } | { error: string }> {
  const r = await liveCall<CustomerCreated>('/till/customers', input, "The PC isn't reachable — add the customer at the counter")
  return 'ok' in r ? { customer: r.ok.customer } : r
}
