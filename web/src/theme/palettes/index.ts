/** Every palette: one CSS file in this folder plus one entry here. The swatch
 * colours only feed the system Appearance page's picker preview. */
export const PALETTES = [
  { id: 'studio', name: 'Studio', swatch: { light: '#EFF0F3', dark: '#111215', brand: '#F2A900' } },
  { id: 'night', name: 'Night', swatch: { light: '#EEF0FA', dark: '#0B1020', brand: '#6E5BFF' } },
] as const
