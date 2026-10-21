import { bold } from './bold/kit'
import { classic } from './classic/kit'
import type { StyleDef } from './kit'
import { grid } from './grid/kit'
import { GridSale } from './grid/sale'
import { minimal } from './minimal/kit'
import { poster } from './poster/kit'
import { PosterSale } from './poster/sale'
import { stamp } from './stamp/kit'
import { StampSale } from './stamp/sale'
import { ticket } from './ticket/kit'

/** Every receipt style, in the order the Appearance page shows them. */
export const STYLES: StyleDef[] = [
  { key: 'classic', label: 'Classic', kit: classic },
  { key: 'bold', label: 'Bold', kit: bold },
  { key: 'minimal', label: 'Minimal', kit: minimal },
  { key: 'ticket', label: 'Ticket', kit: ticket },
  { key: 'poster', label: 'Poster', kit: poster, overrides: { sale: PosterSale } },
  { key: 'grid', label: 'Grid', kit: grid, overrides: { sale: GridSale } },
  { key: 'stamp', label: 'Stamp', kit: stamp, overrides: { sale: StampSale } },
]

/** The style for a saved key; unknown or old keys draw Classic. */
export const styleOf = (key: string): StyleDef => STYLES.find((s) => s.key === key) ?? STYLES[0]
