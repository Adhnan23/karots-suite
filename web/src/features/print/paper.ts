// The paper's own fonts, bundled — a receipt never depends on what a PC has installed.
import '@fontsource/jetbrains-mono/400.css'
import '@fontsource/jetbrains-mono/700.css'
import '@fontsource/noto-sans-sinhala/400.css'
import '@fontsource/noto-sans-sinhala/700.css'
import '@fontsource/noto-sans-tamil/400.css'
import '@fontsource/noto-sans-tamil/700.css'

/** Latin first: JetBrains Mono at 20 px advances 12 px = one Font A column (12 × 24 dots). */
export const PAPER_FONTS = ['JetBrains Mono', 'Noto Sans Sinhala', 'Noto Sans Tamil'] as const
export const PAPER_FAMILY = PAPER_FONTS.map((f) => `"${f}"`).join(', ')
