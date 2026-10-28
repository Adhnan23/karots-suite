import '@fontsource/figtree/600.css'
import '@fontsource/figtree/800.css'
import '@fontsource/sora/800.css'
import '@fontsource/noto-sans-sinhala/600.css'
import '@fontsource/noto-sans-sinhala/800.css'
import '@fontsource/noto-sans-tamil/600.css'
import '@fontsource/noto-sans-tamil/800.css'
import type { Lang } from './types'

// Captions in the language's own face first; Latin words inside fall back to the suite's.
export function fontFor(lang: Lang): string {
  const own = { si: '"Noto Sans Sinhala", ', ta: '"Noto Sans Tamil", ', en: '' }[lang]
  return `${own}"Figtree", system-ui, sans-serif`
}
export const display = '"Sora", "Figtree", sans-serif'
