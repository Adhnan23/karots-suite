// Mirrors plugins/productplus/api.go (by hand — core's types.gen.ts is core-only).

export type FieldType = 'text' | 'textarea' | 'number' | 'bool' | 'select' | 'date'
export type Cat = { id: number; name: string }

export type FieldDTO = {
  id: number; key: string; label: string; type: FieldType; defaultValue: string; hint: string
  required: boolean; searchable: boolean; showAtTill: boolean; printOnLabel: boolean; showInIntake: boolean
  options: string[]; isActive: boolean; scope: Cat[]; missing: number; usingDefault: number
}

export type FieldRequest = {
  label: string; type: FieldType; defaultValue: string; hint: string
  required: boolean; searchable: boolean; showAtTill: boolean; printOnLabel: boolean; showInIntake: boolean
  options: string[]; scope: number[]; defaultFor?: 'new' | 'all'; confirmLoss?: boolean
}

export type Impact = { lossy: number; total: number; usingDefault: number }

export type FormField = { id: number; key: string; label: string; type: FieldType; hint: string; required: boolean; options: string[]; value: string; removed: boolean; own: boolean }

/** values: fieldId → own value, null = reads the default; a field that doesn't apply has no key. */
export type GridRow = { id: number; name: string; categoryName: string; values: Record<string, string | null> }
export type Grid = { rows: GridRow[]; total: number }
export type Cell = { productId: number; fieldId: number; value: string }
export type SaveResult = { saved: number; errors: (Omit<Cell, 'value'> & { message: string })[] }

export const TYPE_LABEL: Record<FieldType, string> = { text: 'Text', textarea: 'Long text', number: 'Number', bool: 'Yes / No', select: 'Dropdown', date: 'Date' }

/** Fill in: a product × field cell's key, and the edits the grid holds. */
export const cellKey = (productId: number, fieldId: number) => `${productId}:${fieldId}`
export type Edits = { dirty: Record<string, string>; errors: Record<string, string>; edit: (productId: number, fieldId: number, v: string) => void }
