import type { Appearance } from '@/api/types.gen'
import { applyAppearance } from '@/theme/appearance'
import { getMode } from '@/theme/mode'

/** Live preview while a palette is edited: the page wears the draft CSS (a <style>
 *  after the bundle), and the editor's Light | Dark switch flips this page only. */
const ID = 'ks-palette-draft'
const root = () => document.documentElement

export function startDraft(id: string) {
  root().dataset.palette = id
  if (!document.getElementById(ID)) document.head.append(Object.assign(document.createElement('style'), { id: ID }))
}

export function updateDraft(css: string) {
  const el = document.getElementById(ID)
  if (el) el.textContent = css
}

export function previewMode(m: 'light' | 'dark') {
  root().dataset.theme = m
}

/** Back to the saved look and the device's own mode. */
export function endDraft(saved: Appearance) {
  document.getElementById(ID)?.remove()
  const m = getMode()
  if (m === 'system') delete root().dataset.theme
  else root().dataset.theme = m
  applyAppearance(saved)
}
