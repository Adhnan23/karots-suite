/** Theme 1 is Dock (the floating-dock shell). Palettes belong to a theme: each one is
 *  one CSS file in ./dock/ plus one entry here. The swatch colours only feed the
 *  system Appearance page's picker preview. */
export const THEME = 'dock'

export const PALETTES = [
  { id: 'studio', theme: THEME, name: 'Studio', swatch: { light: '#EFF0F3', dark: '#111215', brand: '#F2A900' } },
  { id: 'night', theme: THEME, name: 'Night', swatch: { light: '#EEF0FA', dark: '#0B1020', brand: '#6E5BFF' } },
  { id: 'bazaar', theme: THEME, name: 'Bazaar', swatch: { light: '#F6EFE4', dark: '#17110D', brand: '#C2562F' } },
  { id: 'lagoon', theme: THEME, name: 'Lagoon', swatch: { light: '#E6F4F4', dark: '#071A1F', brand: '#087F88' } },
] as const
