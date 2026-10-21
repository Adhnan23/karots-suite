/** Every token a theme-Dock palette defines, in both its light and dark block
 *  (docs/design/theme.md §2). The palette editor and paletteCss.ts read this
 *  list; tokens.test.ts checks every built-in palette file against it. */
export type TokenGroup = 'colour' | 'material' | 'background' | 'depth' | 'shape' | 'motion'
export type TokenKind = 'color' | 'length' | 'shadow' | 'image' | 'filter' | 'time' | 'ease'
export type TokenDef = { name: string; group: TokenGroup; kind: TokenKind; sub?: string }

const c = (sub: string, ...names: string[]): TokenDef[] => names.map((name) => ({ name, group: 'colour', kind: 'color', sub }))

export const TOKENS: readonly TokenDef[] = [
  ...c('Background', '--background'),
  ...c('Surfaces', '--card', '--card-foreground', '--popover', '--popover-foreground', '--muted', '--secondary', '--accent', '--secondary-foreground', '--accent-foreground', '--border', '--input', '--scrim'),
  ...c('Text', '--foreground', '--muted-foreground', '--faint'),
  ...c('Brand', '--brand', '--brand-foreground', '--brand-soft', '--brand-2', '--brand-2-foreground', '--primary', '--primary-foreground', '--ring'),
  ...c('Selected', '--selected', '--selected-foreground'),
  ...c('Money', '--success', '--danger', '--danger-foreground', '--destructive', '--warning'),
  ...c('Hero', '--hero', '--hero-foreground', '--hero-muted'),
  ...c('Charts', '--chart-1', '--chart-2', '--chart-3', '--chart-4', '--chart-5', '--chart-6'),
  { name: '--bar', group: 'material', kind: 'color' },
  { name: '--glass', group: 'material', kind: 'filter' },
  { name: '--bg-art', group: 'background', kind: 'image' },
  { name: '--hero-art', group: 'background', kind: 'image' },
  { name: '--shadow', group: 'depth', kind: 'shadow' },
  { name: '--shadow-float', group: 'depth', kind: 'shadow' },
  { name: '--glow', group: 'depth', kind: 'shadow' },
  { name: '--radius', group: 'shape', kind: 'length' },
  { name: '--radius-sm', group: 'shape', kind: 'length' },
  { name: '--pill', group: 'shape', kind: 'length' },
  { name: '--border-w', group: 'shape', kind: 'length' },
  { name: '--dur-fast', group: 'motion', kind: 'time' },
  { name: '--dur', group: 'motion', kind: 'time' },
  { name: '--dur-slow', group: 'motion', kind: 'time' },
  { name: '--ease', group: 'motion', kind: 'ease' },
  { name: '--ease-spring', group: 'motion', kind: 'ease' },
  { name: '--lift', group: 'motion', kind: 'length' },
]
