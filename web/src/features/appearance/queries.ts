import { api, ApiError } from '@/api/client'
import type { Appearance, Palette } from '@/api/types.gen'
import { buildPalette, paletteId, type PaletteDoc } from './paletteCss'

export const appearanceQuery = { queryKey: ['system', 'appearance'], queryFn: () => api<Appearance>('/system/appearance') }
export const palettesQuery = { queryKey: ['system', 'palettes'], queryFn: () => api<Palette[]>('/system/palettes') }
export const paletteQuery = (id: number) => ({ queryKey: ['system', 'palettes', id], queryFn: () => api<Palette>(`/system/palettes/${id}`) })

/** Saves doc as a new custom palette. A taken name becomes "<name> 2", "<name> 3"… The CSS
 *  names the palette's own id, which only exists after the insert, hence the second write. */
export async function createPalette(doc: PaletteDoc): Promise<Palette> {
  for (let n = 1; n < 50; n++) {
    const name = n === 1 ? doc.name : `${doc.name} ${n}`
    try {
      const made = await api<Palette>('/system/palettes', { json: { name, base: doc.base, css: buildPalette({ ...doc, name }, 'custom-0') } })
      return await savePalette(made.id, { ...doc, name })
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 409)) throw e
    }
  }
  throw new Error('Too many palettes with that name')
}

export function savePalette(id: number, doc: PaletteDoc) {
  return api<Palette>(`/system/palettes/${id}`, { method: 'PUT', json: { name: doc.name, base: doc.base, css: buildPalette(doc, paletteId(id)) } })
}
