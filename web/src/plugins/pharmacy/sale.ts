import type { TillApi } from '../slots'

/** The patient on the sale in progress (TillApi.saleMeta.pharmacy). */
export type SalePatient = { patientId: number; name: string }
export const salePatient = (till: Pick<TillApi, 'saleMeta'>) => till.saleMeta.pharmacy as SalePatient | undefined
