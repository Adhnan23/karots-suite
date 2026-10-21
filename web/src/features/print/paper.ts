// The paper's own fonts, bundled — a receipt never depends on what a PC has installed.
import '@fontsource/jetbrains-mono/400.css'
import '@fontsource/jetbrains-mono/700.css'
import '@fontsource/noto-sans-sinhala/400.css'
import '@fontsource/noto-sans-sinhala/700.css'
import '@fontsource/noto-sans-tamil/400.css'
import '@fontsource/noto-sans-tamil/700.css'
// The receipt styles' sans and serif (Classic uses neither).
import '@fontsource/inter/400.css'
import '@fontsource/inter/700.css'
import '@fontsource/inter/900.css'
import '@fontsource/noto-serif/400.css'
import '@fontsource/noto-serif/400-italic.css'
import '@fontsource/noto-serif/700.css'
import '@fontsource/noto-serif-sinhala/400.css'
import '@fontsource/noto-serif-sinhala/700.css'
import '@fontsource/noto-serif-tamil/400.css'
import '@fontsource/noto-serif-tamil/700.css'

const family = (fonts: readonly string[]) => fonts.map((f) => `"${f}"`).join(', ')
const LOCAL_SANS = ['Noto Sans Sinhala', 'Noto Sans Tamil'] as const

/** Latin first: JetBrains Mono at 20 px advances 12 px = one Font A column (12 × 24 dots). */
export const PAPER_FONTS = ['JetBrains Mono', ...LOCAL_SANS] as const
export const PAPER_FAMILY = family(PAPER_FONTS)
/** A sans paper (Inter), local names in Noto Sans. */
export const SANS_FONTS = ['Inter', ...LOCAL_SANS] as const
export const SANS_FAMILY = family(SANS_FONTS)
/** A serif paper (Noto Serif), local names in Noto Serif. */
export const SERIF_FONTS = ['Noto Serif', 'Noto Serif Sinhala', 'Noto Serif Tamil'] as const
export const SERIF_FAMILY = family(SERIF_FONTS)
