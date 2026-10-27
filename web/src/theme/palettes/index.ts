/** Theme 1 is Dock (the floating-dock shell). Palettes belong to a theme: each one is
 *  one CSS file in ./dock/ plus one entry here. The swatch colours only feed the
 *  system Appearance page's picker preview. */
export const THEME = 'dock'

export const PALETTES = [
  { id: 'studio', theme: THEME, name: 'Studio', swatch: { light: '#EFF0F3', dark: '#111215', brand: '#F2A900' } },
  { id: 'night', theme: THEME, name: 'Night', swatch: { light: '#EEF0FA', dark: '#0B1020', brand: '#6E5BFF' } },
  { id: 'bazaar', theme: THEME, name: 'Bazaar', swatch: { light: '#F6EFE4', dark: '#17110D', brand: '#C2562F' } },
  { id: 'lagoon', theme: THEME, name: 'Lagoon', swatch: { light: '#E6F4F4', dark: '#071A1F', brand: '#087F88' } },
  { id: 'catppuccin', theme: THEME, name: 'Catppuccin', swatch: { light: '#EFF1F5', dark: '#1E1E2E', brand: '#8839EF' } },
  { id: 'gruvbox', theme: THEME, name: 'Gruvbox', swatch: { light: '#FBF1C7', dark: '#282828', brand: '#FE8019' } },
  { id: 'nord', theme: THEME, name: 'Nord', swatch: { light: '#ECEFF4', dark: '#2E3440', brand: '#88C0D0' } },
  { id: 'dracula', theme: THEME, name: 'Dracula', swatch: { light: '#FFFBEB', dark: '#282A36', brand: '#BD93F9' } },
  { id: 'tokyonight', theme: THEME, name: 'Tokyo Night', swatch: { light: '#E1E2E7', dark: '#1A1B26', brand: '#7AA2F7' } },
] as const

/** The built-ins added after the first five (docs/specs/2026-10-27-more-palettes-design.md), in picker order. */
export const NEW_PALETTES: string[] = PALETTES.slice(5).map((p) => p.id)
