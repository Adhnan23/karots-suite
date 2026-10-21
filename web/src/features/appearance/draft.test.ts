import { beforeEach, describe, expect, it } from 'vitest'
import { endDraft, previewMode, startDraft, updateDraft } from './draft'

const saved = { palette: 'studio', brandColor: '', radius: 'rounded', receiptStyle: 'classic' }
const root = document.documentElement

describe('palette draft', () => {
  beforeEach(() => {
    localStorage.clear()
    delete root.dataset.theme
  })
  it('previews on the page and puts the saved look back', () => {
    localStorage.setItem('ks-mode', 'light')
    root.dataset.theme = 'light'
    startDraft('custom-2')
    updateDraft(':root{--brand:red}')
    previewMode('dark')
    expect(root.dataset.palette).toBe('custom-2')
    expect(root.dataset.theme).toBe('dark')
    expect(document.getElementById('ks-palette-draft')?.textContent).toBe(':root{--brand:red}')
    endDraft(saved)
    expect(document.getElementById('ks-palette-draft')).toBeNull()
    expect(root.dataset.palette).toBe('studio')
    expect(root.dataset.theme).toBe('light')
  })
  it('a device on System goes back to System', () => {
    startDraft('custom-2')
    previewMode('dark')
    endDraft(saved)
    expect(root.dataset.theme).toBeUndefined()
  })
})
